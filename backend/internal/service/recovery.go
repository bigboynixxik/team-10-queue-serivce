package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"backend/internal/models"
	"backend/pkg/logger"
)

type recoveryProductState struct {
	stock       *models.ProductStock
	queuedUsers []string
	reserved    int
}

type recoveryTimer struct {
	productID string
	userID    string
	deadline  time.Time
}

// RecoverCache rebuilds Redis from PostgreSQL before the HTTP API and workers
// start accepting work. PostgreSQL is the source of truth; every Redis write
// here returns an error instead of being logged and ignored.
func (s *QueueService) RecoverCache(ctx context.Context) error {
	log := logger.FromContext(ctx)

	snapshot, err := s.durable.LoadRecoverySnapshot(ctx)
	if err != nil {
		return fmt.Errorf("service.RecoverCache load snapshot: %w", err)
	}

	products, timers, err := s.buildRecoveryState(snapshot, time.Now().UTC())
	if err != nil {
		return err
	}

	if err := s.cache.ResetExpiryTimers(ctx); err != nil {
		return fmt.Errorf("service.RecoverCache reset expiry timers: %w", err)
	}

	for _, stock := range snapshot.Stocks {
		product := products[stock.ProductID]
		available := stock.ProductCount - product.reserved
		if err := s.cache.RestoreProductState(
			ctx, stock.ProductID, stock.ProductCount, available, product.queuedUsers,
		); err != nil {
			return fmt.Errorf("service.RecoverCache restore product %s: %w", stock.ProductID, err)
		}
	}

	for _, membership := range snapshot.Memberships {
		if err := s.cache.SetMembership(ctx, membership); err != nil {
			return fmt.Errorf(
				"service.RecoverCache cache membership %s/%s: %w",
				membership.ProductID, membership.UserID, err,
			)
		}
	}

	for _, right := range snapshot.Rights {
		if err := s.cache.SetRight(ctx, right); err != nil {
			return fmt.Errorf("service.RecoverCache cache right %s: %w", right.Token, err)
		}
	}

	for _, timer := range timers {
		if err := s.cache.AddToExpiryTimer(ctx, timer.productID, timer.userID, timer.deadline); err != nil {
			return fmt.Errorf(
				"service.RecoverCache add expiry timer %s/%s: %w",
				timer.productID, timer.userID, err,
			)
		}
	}

	log.InfoContext(ctx, "redis cache recovered from postgres",
		slog.Int("products", len(snapshot.Stocks)),
		slog.Int("memberships", len(snapshot.Memberships)),
		slog.Int("rights", len(snapshot.Rights)),
		slog.Int("timers", len(timers)),
	)

	return nil
}

func (s *QueueService) buildRecoveryState(
	snapshot *models.RecoverySnapshot,
	now time.Time,
) (map[string]*recoveryProductState, []recoveryTimer, error) {
	products, err := recoveryProducts(snapshot.Stocks)
	if err != nil {
		return nil, nil, err
	}

	rightsByToken, activeRightLinked, err := recoveryRights(snapshot.Rights, products)
	if err != nil {
		return nil, nil, err
	}

	timers := make([]recoveryTimer, 0)
	for _, membership := range snapshot.Memberships {
		timer, hasTimer, errMembership := s.applyRecoveryMembership(
			membership, products, rightsByToken, activeRightLinked, now,
		)
		if errMembership != nil {
			return nil, nil, errMembership
		}
		if hasTimer {
			timers = append(timers, timer)
		}
	}

	for token, linked := range activeRightLinked {
		if !linked {
			return nil, nil, fmt.Errorf(
				"service.RecoverCache validate active right %s: %w",
				token, models.ErrInvalidStatus,
			)
		}
	}

	for _, product := range products {
		available := product.stock.ProductCount - product.reserved
		if available < 0 {
			return nil, nil, fmt.Errorf(
				"service.RecoverCache product %s negative available units: %w",
				product.stock.ProductID, models.ErrStockDepleted,
			)
		}
	}

	return products, timers, nil
}

func recoveryProducts(stocks []*models.ProductStock) (map[string]*recoveryProductState, error) {
	products := make(map[string]*recoveryProductState, len(stocks))
	for _, stock := range stocks {
		if stock.ProductID == "" || stock.ProductCount < 0 || stock.TotalStock < 0 || stock.ProductCount > stock.TotalStock {
			return nil, fmt.Errorf(
				"service.RecoverCache validate stock %s: %w",
				stock.ProductID, models.ErrInvalidStatus,
			)
		}
		if _, exists := products[stock.ProductID]; exists {
			return nil, fmt.Errorf(
				"service.RecoverCache duplicate stock %s: %w",
				stock.ProductID, models.ErrInvalidStatus,
			)
		}

		products[stock.ProductID] = &recoveryProductState{stock: stock}
	}

	return products, nil
}

func recoveryRights(
	rights []*models.Right,
	products map[string]*recoveryProductState,
) (map[string]*models.Right, map[string]bool, error) {
	rightsByToken := make(map[string]*models.Right, len(rights))
	activeRightLinked := make(map[string]bool)

	for _, right := range rights {
		if right.Token == "" || right.UserID == "" || right.ProductID == "" || right.Quantity <= 0 {
			return nil, nil, fmt.Errorf(
				"service.RecoverCache validate right %s: %w",
				right.Token, models.ErrInvalidStatus,
			)
		}
		if err := right.Status.Valid(); err != nil {
			return nil, nil, fmt.Errorf(
				"service.RecoverCache validate right %s status: %w",
				right.Token, err,
			)
		}
		if _, exists := rightsByToken[right.Token]; exists {
			return nil, nil, fmt.Errorf(
				"service.RecoverCache duplicate right %s: %w",
				right.Token, models.ErrInvalidStatus,
			)
		}
		if right.Status == models.RightStatusActive {
			if _, exists := products[right.ProductID]; !exists {
				return nil, nil, fmt.Errorf(
					"service.RecoverCache active right %s missing stock %s: %w",
					right.Token, right.ProductID, models.ErrInvalidStatus,
				)
			}
			activeRightLinked[right.Token] = false
		}

		rightsByToken[right.Token] = right
	}

	return rightsByToken, activeRightLinked, nil
}

func (s *QueueService) applyRecoveryMembership(
	membership *models.QueueMembership,
	products map[string]*recoveryProductState,
	rightsByToken map[string]*models.Right,
	activeRightLinked map[string]bool,
	now time.Time,
) (recoveryTimer, bool, error) {
	product, exists := products[membership.ProductID]
	if !exists {
		return recoveryTimer{}, false, fmt.Errorf(
			"service.RecoverCache membership %s/%s missing stock: %w",
			membership.ProductID, membership.UserID, models.ErrInvalidStatus,
		)
	}
	if membership.UserID == "" || membership.Quantity <= 0 {
		return recoveryTimer{}, false, fmt.Errorf(
			"service.RecoverCache validate membership %s/%s: %w",
			membership.ProductID, membership.UserID, models.ErrInvalidStatus,
		)
	}
	if err := membership.Status.Valid(); err != nil {
		return recoveryTimer{}, false, fmt.Errorf(
			"service.RecoverCache validate membership %s/%s status: %w",
			membership.ProductID, membership.UserID, err,
		)
	}

	switch membership.Status {
	case models.MembershipStatusQueued:
		product.queuedUsers = append(product.queuedUsers, membership.UserID)
	case models.MembershipStatusRightActive:
		timer, err := s.applyRecoveredActiveRight(
			membership, product, rightsByToken, activeRightLinked, now,
		)
		if err != nil {
			return recoveryTimer{}, false, err
		}

		return timer, true, nil
	case models.MembershipStatusOfferPending:
		timer, err := applyRecoveredPendingOffer(membership, product)
		if err != nil {
			return recoveryTimer{}, false, err
		}

		return timer, true, nil
	case models.MembershipStatusDeclined, models.MembershipStatusPurchased, models.MembershipStatusSoldOut:
		// Terminal states are cached for reads but do not reserve stock or timers.
	}

	return recoveryTimer{}, false, nil
}

func (s *QueueService) applyRecoveredActiveRight(
	membership *models.QueueMembership,
	product *recoveryProductState,
	rightsByToken map[string]*models.Right,
	activeRightLinked map[string]bool,
	now time.Time,
) (recoveryTimer, error) {
	if membership.CurrentToken == nil || membership.ExpiresAt == nil {
		return recoveryTimer{}, fmt.Errorf(
			"service.RecoverCache active membership %s/%s missing token or expiry: %w",
			membership.ProductID, membership.UserID, models.ErrInvalidStatus,
		)
	}

	right, exists := rightsByToken[*membership.CurrentToken]
	if !exists || right.Status != models.RightStatusActive ||
		right.UserID != membership.UserID ||
		right.ProductID != membership.ProductID ||
		right.Quantity != membership.Quantity {
		return recoveryTimer{}, fmt.Errorf(
			"service.RecoverCache active membership %s/%s mismatched right: %w",
			membership.ProductID, membership.UserID, models.ErrInvalidStatus,
		)
	}

	product.reserved += membership.Quantity
	activeRightLinked[right.Token] = true

	return recoveryTimer{
		productID: membership.ProductID,
		userID:    membership.UserID,
		deadline:  s.rightHeartbeatDeadline(now, *membership.ExpiresAt),
	}, nil
}

func applyRecoveredPendingOffer(
	membership *models.QueueMembership,
	product *recoveryProductState,
) (recoveryTimer, error) {
	if membership.AvailableQuantity == nil || membership.ExpiresAt == nil ||
		*membership.AvailableQuantity <= 0 ||
		*membership.AvailableQuantity > membership.Quantity {
		return recoveryTimer{}, fmt.Errorf(
			"service.RecoverCache pending offer %s/%s invalid quantity or expiry: %w",
			membership.ProductID, membership.UserID, models.ErrInvalidStatus,
		)
	}

	product.reserved += *membership.AvailableQuantity

	return recoveryTimer{
		productID: membership.ProductID,
		userID:    membership.UserID,
		deadline:  *membership.ExpiresAt,
	}, nil
}
