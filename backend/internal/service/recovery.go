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

// recoveryRepairs collects the durable writes that bring inconsistent rows back
// in line with the state machine. They are applied to PostgreSQL before Redis is
// rebuilt, so both stores end up agreeing.
type recoveryRepairs struct {
	expiredRights  []string
	declinedMember []*models.QueueMembership
	skipped        int
}

// RecoverCache rebuilds Redis from PostgreSQL before the HTTP API and workers
// start accepting work. PostgreSQL is the source of truth; every Redis write
// here returns an error instead of being logged and ignored.
//
// Inconsistent durable rows do not stop the service. A row that contradicts the
// state machine — an ACTIVE right nobody holds, a RIGHT_ACTIVE membership whose
// right is gone — is repaired towards the terminal state it should already have
// reached, because refusing to start leaves the service permanently down over
// data that no longer holds any stock. Only a failing store is fatal.
func (s *QueueService) RecoverCache(ctx context.Context) error {
	log := logger.FromContext(ctx)

	snapshot, err := s.durable.LoadRecoverySnapshot(ctx)
	if err != nil {
		return fmt.Errorf("service.RecoverCache load snapshot: %w", err)
	}

	products, timers, repairs := s.buildRecoveryState(ctx, snapshot, time.Now().UTC())

	if err := s.applyRecoveryRepairs(ctx, repairs); err != nil {
		return err
	}

	if err := s.cache.ResetExpiryTimers(ctx); err != nil {
		return fmt.Errorf("service.RecoverCache reset expiry timers: %w", err)
	}

	for _, stock := range snapshot.Stocks {
		product, ok := products[stock.ProductID]
		if !ok {
			continue
		}

		available := stock.ProductCount - product.reserved
		if available < 0 {
			// Held units exceed the stock: the durable rows disagree with each
			// other. Handing out from a negative pool is the one thing that must
			// not happen, so the pool is closed rather than trusted.
			log.WarnContext(ctx, "recovery clamped negative available units",
				slog.String("product_id", stock.ProductID),
				slog.Int("product_count", stock.ProductCount),
				slog.Int("reserved", product.reserved),
			)

			available = 0
		}

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
		slog.Int("repaired_rights", len(repairs.expiredRights)),
		slog.Int("repaired_memberships", len(repairs.declinedMember)),
		slog.Int("skipped_rows", repairs.skipped),
	)

	return nil
}

// applyRecoveryRepairs persists the corrections before Redis is filled, so a
// crash in between leaves PostgreSQL consistent and the next pass idempotent.
func (s *QueueService) applyRecoveryRepairs(ctx context.Context, repairs recoveryRepairs) error {
	if len(repairs.expiredRights) > 0 {
		if err := s.durable.ExpireRights(ctx, repairs.expiredRights); err != nil {
			return fmt.Errorf("service.RecoverCache expire orphaned rights: %w", err)
		}
	}

	for _, membership := range repairs.declinedMember {
		if err := s.durable.UpsertMembership(ctx, membership); err != nil {
			return fmt.Errorf(
				"service.RecoverCache settle membership %s/%s: %w",
				membership.ProductID, membership.UserID, err,
			)
		}
	}

	return nil
}

func (s *QueueService) buildRecoveryState(
	ctx context.Context,
	snapshot *models.RecoverySnapshot,
	now time.Time,
) (map[string]*recoveryProductState, []recoveryTimer, recoveryRepairs) {
	log := logger.FromContext(ctx)
	repairs := recoveryRepairs{}

	products := recoveryProducts(ctx, snapshot.Stocks, &repairs)
	rightsByToken, activeRights := recoveryRights(ctx, snapshot.Rights, products, &repairs)

	timers := make([]recoveryTimer, 0)
	for _, membership := range snapshot.Memberships {
		timer, hasTimer := s.applyRecoveryMembership(
			ctx, membership, products, rightsByToken, activeRights, &repairs, now,
		)
		if hasTimer {
			timers = append(timers, timer)
		}
	}

	// Whatever is still unclaimed belongs to no live membership. The stock it
	// once held has already been returned by whichever transition ended that
	// membership, so the right is settled rather than honoured.
	for token, claimed := range activeRights {
		if claimed {
			continue
		}

		log.WarnContext(ctx, "recovery expired orphaned active right", slog.String("token", token))
		repairs.expiredRights = append(repairs.expiredRights, token)

		if right, ok := rightsByToken[token]; ok {
			right.Status = models.RightStatusExpired
		}
	}

	return products, timers, repairs
}

func recoveryProducts(
	ctx context.Context,
	stocks []*models.ProductStock,
	repairs *recoveryRepairs,
) map[string]*recoveryProductState {
	log := logger.FromContext(ctx)
	products := make(map[string]*recoveryProductState, len(stocks))

	for _, stock := range stocks {
		invalid := stock.ProductID == "" || stock.ProductCount < 0 ||
			stock.TotalStock < 0 || stock.ProductCount > stock.TotalStock
		if invalid {
			log.ErrorContext(ctx, "recovery skipped malformed stock row",
				slog.String("product_id", stock.ProductID),
				slog.Int("product_count", stock.ProductCount),
				slog.Int("total_stock", stock.TotalStock),
			)
			repairs.skipped++

			continue
		}
		if _, exists := products[stock.ProductID]; exists {
			log.ErrorContext(ctx, "recovery skipped duplicate stock row",
				slog.String("product_id", stock.ProductID))
			repairs.skipped++

			continue
		}

		products[stock.ProductID] = &recoveryProductState{stock: stock}
	}

	return products
}

// recoveryRights indexes the rights and reports which ACTIVE ones still need a
// membership to claim them.
func recoveryRights(
	ctx context.Context,
	rights []*models.Right,
	products map[string]*recoveryProductState,
	repairs *recoveryRepairs,
) (map[string]*models.Right, map[string]bool) {
	log := logger.FromContext(ctx)
	rightsByToken := make(map[string]*models.Right, len(rights))
	activeRights := make(map[string]bool)

	for _, right := range rights {
		malformed := right.Token == "" || right.UserID == "" ||
			right.ProductID == "" || right.Quantity <= 0
		if malformed || right.Status.Valid() != nil {
			log.ErrorContext(ctx, "recovery skipped malformed right",
				slog.String("token", right.Token), slog.String("status", string(right.Status)))
			repairs.skipped++

			continue
		}
		if _, exists := rightsByToken[right.Token]; exists {
			log.ErrorContext(ctx, "recovery skipped duplicate right", slog.String("token", right.Token))
			repairs.skipped++

			continue
		}

		rightsByToken[right.Token] = right

		if right.Status != models.RightStatusActive {
			continue
		}

		if _, exists := products[right.ProductID]; !exists {
			// Without a stock row there is nothing to reserve against, so the
			// right cannot be honoured however it got here.
			log.WarnContext(ctx, "recovery expired active right without stock",
				slog.String("token", right.Token), slog.String("product_id", right.ProductID))
			repairs.expiredRights = append(repairs.expiredRights, right.Token)
			right.Status = models.RightStatusExpired

			continue
		}

		activeRights[right.Token] = false
	}

	return rightsByToken, activeRights
}

func (s *QueueService) applyRecoveryMembership(
	ctx context.Context,
	membership *models.QueueMembership,
	products map[string]*recoveryProductState,
	rightsByToken map[string]*models.Right,
	activeRights map[string]bool,
	repairs *recoveryRepairs,
	now time.Time,
) (recoveryTimer, bool) {
	log := logger.FromContext(ctx)

	if membership.UserID == "" || membership.Quantity <= 0 || membership.Status.Valid() != nil {
		log.ErrorContext(ctx, "recovery skipped malformed membership",
			slog.String("product_id", membership.ProductID),
			slog.String("user_id", membership.UserID),
			slog.String("status", string(membership.Status)),
		)
		repairs.skipped++

		return recoveryTimer{}, false
	}

	product, exists := products[membership.ProductID]
	if !exists {
		// No stock row means the product is unknown to us: the membership can be
		// read back, but it cannot queue for or hold anything.
		log.WarnContext(ctx, "recovery kept membership without stock row",
			slog.String("product_id", membership.ProductID),
			slog.String("user_id", membership.UserID),
		)
		repairs.skipped++

		return recoveryTimer{}, false
	}

	switch membership.Status {
	case models.MembershipStatusQueued:
		product.queuedUsers = append(product.queuedUsers, membership.UserID)
	case models.MembershipStatusRightActive:
		return s.applyRecoveredActiveRight(ctx, membership, product, rightsByToken, activeRights, repairs, now)
	case models.MembershipStatusOfferPending:
		return applyRecoveredPendingOffer(ctx, membership, product, repairs)
	case models.MembershipStatusDeclined, models.MembershipStatusPurchased, models.MembershipStatusSoldOut:
		// Terminal states are cached for reads but do not reserve stock or timers.
	}

	return recoveryTimer{}, false
}

func (s *QueueService) applyRecoveredActiveRight(
	ctx context.Context,
	membership *models.QueueMembership,
	product *recoveryProductState,
	rightsByToken map[string]*models.Right,
	activeRights map[string]bool,
	repairs *recoveryRepairs,
	now time.Time,
) (recoveryTimer, bool) {
	log := logger.FromContext(ctx)

	var right *models.Right
	if membership.CurrentToken != nil {
		right = rightsByToken[*membership.CurrentToken]
	}

	usable := right != nil && membership.ExpiresAt != nil &&
		right.Status == models.RightStatusActive &&
		right.UserID == membership.UserID &&
		right.ProductID == membership.ProductID &&
		right.Quantity == membership.Quantity

	if !usable {
		// The membership claims a right that does not back it. Whatever ended the
		// right also returned its units, so the user is settled as DECLINED —
		// the same terminal state an expiry would have produced.
		log.WarnContext(ctx, "recovery settled membership without a usable right",
			slog.String("product_id", membership.ProductID),
			slog.String("user_id", membership.UserID),
		)

		settleRecoveredMembership(membership, now)
		repairs.declinedMember = append(repairs.declinedMember, membership)

		return recoveryTimer{}, false
	}

	product.reserved += membership.Quantity
	activeRights[right.Token] = true

	return recoveryTimer{
		productID: membership.ProductID,
		userID:    membership.UserID,
		deadline:  s.rightHeartbeatDeadline(now, *membership.ExpiresAt),
	}, true
}

func applyRecoveredPendingOffer(
	ctx context.Context,
	membership *models.QueueMembership,
	product *recoveryProductState,
	repairs *recoveryRepairs,
) (recoveryTimer, bool) {
	log := logger.FromContext(ctx)

	invalid := membership.AvailableQuantity == nil || membership.ExpiresAt == nil ||
		*membership.AvailableQuantity <= 0 ||
		*membership.AvailableQuantity > membership.Quantity

	if invalid {
		// An offer with no quantity or no deadline can never be accepted, and
		// silence on an offer means DECLINED (docs/design_context.md, п. 5.4).
		log.WarnContext(ctx, "recovery settled malformed pending offer",
			slog.String("product_id", membership.ProductID),
			slog.String("user_id", membership.UserID),
		)

		settleRecoveredMembership(membership, time.Now().UTC())
		repairs.declinedMember = append(repairs.declinedMember, membership)

		return recoveryTimer{}, false
	}

	product.reserved += *membership.AvailableQuantity

	return recoveryTimer{
		productID: membership.ProductID,
		userID:    membership.UserID,
		deadline:  *membership.ExpiresAt,
	}, true
}

// settleRecoveredMembership moves a membership to the terminal DECLINED state in
// place, so the same object is what gets written to both stores.
func settleRecoveredMembership(membership *models.QueueMembership, now time.Time) {
	membership.Status = models.MembershipStatusDeclined
	membership.AvailableQuantity = nil
	membership.CurrentToken = nil
	membership.ExpiresAt = nil
	membership.UpdatedAt = now
}
