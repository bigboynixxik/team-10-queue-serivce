// Package service provides the core business logic for the queue and order allocation system.
// It acts as a strict State Machine ensuring data consistency between cache and durable storage.
package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"backend/internal/models"
	"backend/pkg/logger"

	"github.com/google/uuid"
)

// QueueService orchestrates the queue state machine, durable storage, and fast cache.
type QueueService struct {
	durable               DurableRepo
	cache                 CacheRepo
	avito                 AvitoClient
	offerTTL              time.Duration
	paymentTTL            time.Duration
	avgPaymentTime        time.Duration
	heartbeatTimeout      time.Duration
	stockOutboxLease      time.Duration
	stockOutboxBatchSize  int
	stockOutboxMaxBackoff time.Duration
}

// Option tweaks QueueService runtime settings that do not belong to the core
// state-machine constructor.
type Option func(*QueueService)

// WithStockOutbox configures the durable Avito stock delivery worker.
func WithStockOutbox(lease time.Duration, batchSize int, maxBackoff time.Duration) Option {
	return func(s *QueueService) {
		if lease > 0 {
			s.stockOutboxLease = lease
		}
		if batchSize > 0 {
			s.stockOutboxBatchSize = batchSize
		}
		if maxBackoff > 0 {
			s.stockOutboxMaxBackoff = maxBackoff
		}
	}
}

// NewQueueService constructs a new QueueService.
func NewQueueService(
	durable DurableRepo,
	cache CacheRepo,
	avito AvitoClient,
	offerTTL time.Duration,
	paymentTTL time.Duration,
	avgPaymentTime time.Duration,
	heartbeatTimeout time.Duration,
	options ...Option,
) *QueueService {
	s := &QueueService{
		durable:               durable,
		cache:                 cache,
		avito:                 avito,
		offerTTL:              offerTTL,
		paymentTTL:            paymentTTL,
		avgPaymentTime:        avgPaymentTime,
		heartbeatTimeout:      heartbeatTimeout,
		stockOutboxLease:      defaultStockOutboxLease,
		stockOutboxBatchSize:  defaultStockOutboxBatchSize,
		stockOutboxMaxBackoff: defaultStockOutboxMaxBackoff,
	}

	for _, option := range options {
		option(s)
	}

	return s
}

// membershipClaimTTL bounds how long one transition may hold the claim. It only
// has to outlive a normal call; a crashed process releases the user by expiry
// rather than locking them out.
const membershipClaimTTL = 30 * time.Second

// joinAwaitAttempts and joinAwaitDelay define how long a losing request waits for
// the winner to finish.
const (
	joinAwaitAttempts = 20
	joinAwaitDelay    = 25 * time.Millisecond
)

// awaitConcurrentJoin serves the request that lost the claim. Rather than failing,
// it waits for the winner to publish the membership and returns it, which is what
// keeps POST idempotent under a double click (docs/design_context.md, п. 2.1).
func (s *QueueService) awaitConcurrentJoin(
	ctx context.Context, productID, userID string,
) (*models.QueueMembership, *models.Right, error) {
	for attempt := 0; attempt < joinAwaitAttempts; attempt++ {
		mem, right, isHandled, err := s.checkIdempotency(ctx, productID, userID)
		if err != nil {
			return nil, nil, fmt.Errorf("service.JoinQueue await: %w", err)
		}
		if isHandled {
			return mem, right, nil
		}

		select {
		case <-ctx.Done():
			return nil, nil, ctx.Err()
		case <-time.After(joinAwaitDelay):
		}
	}

	// The winner neither finished nor left a membership behind. Reporting a
	// conflict is honest: the client may retry, and the claim has a TTL.
	return nil, nil, models.ErrConcurrentJoin
}

// JoinQueue acts as the entry point for users requesting to buy a product.
func (s *QueueService) JoinQueue(ctx context.Context, productID, userID string, quantity int) (*models.QueueMembership, *models.Right, error) {
	log := logger.FromContext(ctx)

	if quantity <= 0 {
		return nil, nil, models.ErrQuantityInvalid
	}

	mem, right, isHandled, errIdemp := s.checkIdempotency(ctx, productID, userID)
	if errIdemp != nil {
		return nil, nil, fmt.Errorf("service.JoinQueue: %w", errIdemp)
	}
	if isHandled {
		return mem, right, nil
	}

	// The idempotency check above reads state and decides to create a new
	// membership, but between those two moments nothing stops a second request
	// from doing exactly the same. Without a claim, N parallel requests from one
	// user each allocate their own units and walk away with N rights.
	claimOwner := uuid.NewString()
	won, errClaim := s.cache.ClaimMembership(ctx, productID, userID, claimOwner, membershipClaimTTL)
	if errClaim != nil {
		return nil, nil, fmt.Errorf("service.JoinQueue claim: %w", errClaim)
	}
	if !won {
		return s.awaitConcurrentJoin(ctx, productID, userID)
	}
	defer func() {
		if errRelease := s.cache.ReleaseMembershipClaim(
			context.WithoutCancel(ctx), productID, userID, claimOwner,
		); errRelease != nil {
			log.WarnContext(ctx, "failed to release join claim", slog.Any("error", errRelease))
		}
	}()

	// State may have changed while waiting for the claim, so the idempotency
	// check is repeated — this time under exclusive access.
	mem, right, isHandled, errIdemp = s.checkIdempotency(ctx, productID, userID)
	if errIdemp != nil {
		return nil, nil, fmt.Errorf("service.JoinQueue: %w", errIdemp)
	}
	if isHandled {
		return mem, right, nil
	}

	totalStock, err := s.avito.GetInitialStock(ctx, productID)
	if err != nil {
		return nil, nil, fmt.Errorf("service.JoinQueue fetch initial stock: %w", err)
	}

	if errInit := s.cache.InitStock(ctx, productID, totalStock); errInit != nil {
		log.WarnContext(ctx, "failed to initialize stock in cache", slog.Any("error", errInit))
	}

	stockModel := &models.ProductStock{
		ProductID:    productID,
		TotalStock:   totalStock,
		ProductCount: totalStock,
		UpdatedAt:    time.Now().UTC(),
	}
	if errSave := s.durable.SaveInitialStock(ctx, stockModel); errSave != nil {
		log.ErrorContext(ctx, "CRITICAL: failed to save initial stock to db", slog.Any("error", errSave))
	}

	alloc, avail, soldOut, errAlloc := s.cache.TryAllocate(ctx, productID, quantity)
	if errAlloc != nil {
		return nil, nil, fmt.Errorf("service.JoinQueue try allocate: %w", errAlloc)
	}

	newMem := &models.QueueMembership{
		ProductID: productID,
		UserID:    userID,
		Quantity:  quantity,
		CreatedAt: time.Now().UTC(),
	}

	allocatedRight, errProcess := s.processAllocation(ctx, newMem, alloc, avail, soldOut)
	if errProcess != nil {
		return nil, nil, errProcess
	}

	if newMem.Status == models.MembershipStatusQueued {
		if errEnq := s.cache.Enqueue(ctx, productID, userID); errEnq != nil {
			log.ErrorContext(ctx, "failed to enqueue user", slog.Any("error", errEnq))
		}
	}

	return newMem, allocatedRight, nil
}

// checkIdempotency is a helper method to reduce cyclomatic complexity in JoinQueue.
// It verifies if a user is already in the queue or has an active right, returning
// early if no further processing is needed.
func (s *QueueService) checkIdempotency(ctx context.Context, productID, userID string) (*models.QueueMembership, *models.Right, bool, error) {
	existingMem, err := s.cache.GetMembership(ctx, productID, userID)
	if err != nil {
		if errors.Is(err, models.ErrTokenNotFound) {
			return nil, nil, false, nil
		}
		return nil, nil, false, fmt.Errorf("check membership: %w", err)
	}

	if existingMem.Status == models.MembershipStatusRightActive && existingMem.CurrentToken != nil {
		right, _ := s.cache.GetRight(ctx, *existingMem.CurrentToken)
		return existingMem, right, true, nil
	}

	if existingMem.Status == models.MembershipStatusQueued || existingMem.Status == models.MembershipStatusOfferPending {
		return existingMem, nil, true, nil
	}

	return nil, nil, false, nil
}

// processAllocation encapsulates the state machine transition logic.
func (s *QueueService) processAllocation(
	ctx context.Context,
	mem *models.QueueMembership,
	alloc, avail int,
	soldOut bool,
) (*models.Right, error) {
	log := logger.FromContext(ctx)
	now := time.Now().UTC()
	mem.UpdatedAt = now

	if alloc == mem.Quantity {
		right := &models.Right{
			Token:     uuid.NewString(),
			UserID:    mem.UserID,
			ProductID: mem.ProductID,
			Quantity:  alloc,
			Status:    models.RightStatusActive,
			CreatedAt: now,
			ExpiresAt: now.Add(s.paymentTTL),
		}

		if err := s.durable.SaveRight(ctx, right); err != nil {
			if errRb := s.cache.RestoreAvailableUnits(context.Background(), mem.ProductID, alloc); errRb != nil {
				log.ErrorContext(ctx, "CRITICAL: failed to rollback redis stock after failed PG save", slog.Any("error", errRb))
			}
			return nil, fmt.Errorf("service.processAllocation save right: %w", err)
		}

		mem.Status = models.MembershipStatusRightActive
		mem.CurrentToken = &right.Token
		mem.ExpiresAt = &right.ExpiresAt

		if err := s.durable.UpsertMembership(ctx, mem); err != nil {
			log.ErrorContext(ctx, "failed to upsert membership after right active", slog.Any("error", err))
		}

		s.syncCacheState(ctx, mem, right)
		return right, nil
	}

	if avail > 0 {
		mem.Status = models.MembershipStatusOfferPending
		mem.AvailableQuantity = &avail
		exp := new(time.Time)
		*exp = now.Add(s.offerTTL)
		mem.ExpiresAt = exp

		if err := s.durable.UpsertMembership(ctx, mem); err != nil {
			if errRb := s.cache.RestoreAvailableUnits(context.Background(), mem.ProductID, avail); errRb != nil {
				log.ErrorContext(ctx, "CRITICAL: failed to rollback partial redis stock", slog.Any("error", errRb))
			}
			return nil, fmt.Errorf("service.processAllocation upsert partial: %w", err)
		}

		s.syncCacheState(ctx, mem, nil)
		return nil, nil
	}

	if soldOut {
		mem.Status = models.MembershipStatusSoldOut
	} else {
		mem.Status = models.MembershipStatusQueued
	}

	if err := s.durable.UpsertMembership(ctx, mem); err != nil {
		return nil, fmt.Errorf("service.processAllocation upsert final state: %w", err)
	}

	s.syncCacheState(ctx, mem, nil)
	return nil, nil
}

// AcceptOffer confirms a partial offer. If the accepted quantity is less than the
// available quantity, the unused remainder is restored to the pool and the queue advances.
func (s *QueueService) AcceptOffer(ctx context.Context, productID, userID string, acceptedQuantity int) (*models.Right, error) {
	log := logger.FromContext(ctx)

	if acceptedQuantity <= 0 {
		return nil, models.ErrQuantityInvalid
	}

	// Same race as JoinQueue had: the status check and the write are separate
	// steps, so N parallel accepts each pass the check and each issue a right —
	// ten of them turned an offer of two units into ten active rights.
	claimOwner := uuid.NewString()
	won, errClaim := s.cache.ClaimMembership(ctx, productID, userID, claimOwner, membershipClaimTTL)
	if errClaim != nil {
		return nil, fmt.Errorf("service.AcceptOffer claim: %w", errClaim)
	}
	if !won {
		return s.awaitConcurrentAccept(ctx, productID, userID)
	}
	defer func() {
		if errRelease := s.cache.ReleaseMembershipClaim(
			context.WithoutCancel(ctx), productID, userID, claimOwner,
		); errRelease != nil {
			log.WarnContext(ctx, "failed to release membership claim", slog.Any("error", errRelease))
		}
	}()

	mem, err := s.cache.GetMembership(ctx, productID, userID)
	if err != nil {
		return nil, fmt.Errorf("service.AcceptOffer get membership: %w", err)
	}

	if mem.ExpiresAt != nil && time.Now().UTC().After(*mem.ExpiresAt) {
		return nil, models.ErrTokenExpired
	}

	if mem.Status != models.MembershipStatusOfferPending || mem.AvailableQuantity == nil {
		return nil, models.ErrInvalidStatus
	}

	if acceptedQuantity > *mem.AvailableQuantity {
		return nil, models.ErrQuantityExceeded
	}

	returnedQty := *mem.AvailableQuantity - acceptedQuantity
	now := time.Now().UTC()

	right := &models.Right{
		Token:     uuid.NewString(),
		UserID:    mem.UserID,
		ProductID: mem.ProductID,
		Quantity:  acceptedQuantity,
		Status:    models.RightStatusActive,
		CreatedAt: now,
		ExpiresAt: now.Add(s.paymentTTL),
	}

	if err := s.durable.SaveRight(ctx, right); err != nil {
		return nil, fmt.Errorf("service.AcceptOffer save right: %w", err)
	}

	mem.Status = models.MembershipStatusRightActive
	mem.Quantity = acceptedQuantity
	mem.AvailableQuantity = nil
	mem.CurrentToken = &right.Token
	mem.ExpiresAt = &right.ExpiresAt
	mem.UpdatedAt = now

	if err := s.durable.UpsertMembership(ctx, mem); err != nil {
		log.ErrorContext(ctx, "failed to upsert membership after accept", slog.Any("error", err))
	}

	s.syncCacheState(ctx, mem, right)

	if returnedQty > 0 {
		if err := s.cache.RestoreAvailableUnits(ctx, productID, returnedQty); err != nil {
			log.ErrorContext(ctx, "failed to restore unused units", slog.Any("error", err))
		}
		if err := s.AdvanceQueue(ctx, productID); err != nil {
			log.ErrorContext(ctx, "failed to advance queue after partial accept", slog.Any("error", err))
		}
	}

	return right, nil
}

// DeclineOffer rejects a pending offer. The reserved stock is entirely returned
// to the available pool, and the queue is advanced.
func (s *QueueService) DeclineOffer(ctx context.Context, productID, userID string) error {
	return s.leaveQueue(ctx, productID, userID, true)
}

// LeaveQueue ends a user's participation when they are queued, considering a
// partial offer, or hold an active purchase right.
func (s *QueueService) LeaveQueue(ctx context.Context, productID, userID string) error {
	return s.leaveQueue(ctx, productID, userID, false)
}

func (s *QueueService) leaveQueue(ctx context.Context, productID, userID string, offerOnly bool) error {
	log := logger.FromContext(ctx)
	claimOwner := uuid.NewString()

	won, errClaim := s.cache.ClaimMembership(
		ctx, productID, userID, claimOwner, membershipClaimTTL,
	)
	if errClaim != nil {
		return fmt.Errorf("service.leaveQueue claim: %w", errClaim)
	}
	if !won {
		return models.ErrConcurrentJoin
	}
	defer func() {
		if errRelease := s.cache.ReleaseMembershipClaim(
			context.WithoutCancel(ctx), productID, userID, claimOwner,
		); errRelease != nil {
			log.WarnContext(ctx, "failed to release leave claim", slog.Any("error", errRelease))
		}
	}()

	mem, err := s.cache.GetMembership(ctx, productID, userID)
	if err != nil {
		return fmt.Errorf("service.leaveQueue get membership: %w", err)
	}

	if mem.ExpiresAt != nil && time.Now().UTC().After(*mem.ExpiresAt) {
		return models.ErrTokenExpired
	}

	if offerOnly && mem.Status != models.MembershipStatusOfferPending {
		return models.ErrInvalidStatus
	}

	returnedQty := 0
	removeFromQueue := false
	removeExpiryTimer := false

	switch mem.Status {
	case models.MembershipStatusQueued:
		if offerOnly {
			return models.ErrInvalidStatus
		}
		removeFromQueue = true
	case models.MembershipStatusOfferPending:
		if mem.AvailableQuantity == nil {
			return models.ErrInvalidStatus
		}
		returnedQty = *mem.AvailableQuantity
		removeExpiryTimer = true
	case models.MembershipStatusRightActive:
		if offerOnly {
			return models.ErrInvalidStatus
		}
		return s.expireActiveRight(ctx, mem, true)
	default:
		return models.ErrInvalidStatus
	}

	now := time.Now().UTC()

	mem.Status = models.MembershipStatusDeclined
	mem.AvailableQuantity = nil
	mem.CurrentToken = nil
	mem.ExpiresAt = nil
	mem.UpdatedAt = now

	if err := s.durable.UpsertMembership(ctx, mem); err != nil {
		return fmt.Errorf("service.leaveQueue upsert final state: %w", err)
	}

	s.syncCacheState(ctx, mem, nil)

	if removeFromQueue {
		if err := s.cache.RemoveFromQueue(ctx, productID, userID); err != nil {
			log.ErrorContext(ctx, "failed to remove user from queue", slog.Any("error", err))
		}
	}

	if removeExpiryTimer {
		if err := s.cache.RemoveFromExpiryTimer(ctx, productID, userID); err != nil {
			log.ErrorContext(ctx, "failed to remove from expiry timer", slog.Any("error", err))
		}
	}

	if returnedQty > 0 {
		if err := s.cache.RestoreAvailableUnits(ctx, productID, returnedQty); err != nil {
			log.ErrorContext(ctx, "failed to restore unused units", slog.Any("error", err))
		}
		if err := s.AdvanceQueue(ctx, productID); err != nil {
			log.ErrorContext(ctx, "failed to advance queue after leaving", slog.Any("error", err))
		}
	}

	return nil
}

// expireActiveRight performs the single allowed unpaid terminal transition.
// PostgreSQL decides which concurrent operation won; only the winner restores stock.
func (s *QueueService) expireActiveRight(
	ctx context.Context,
	mem *models.QueueMembership,
	removeExpiryTimer bool,
) error {
	log := logger.FromContext(ctx)

	if mem.CurrentToken == nil {
		return models.ErrInvalidStatus
	}

	token := *mem.CurrentToken
	returnedQty := mem.Quantity
	finalMem := *mem
	finalMem.Status = models.MembershipStatusDeclined
	finalMem.AvailableQuantity = nil
	finalMem.CurrentToken = nil
	finalMem.ExpiresAt = nil
	finalMem.UpdatedAt = time.Now().UTC()

	right, transitioned, err := s.durable.ExpireRightAndUpsertMembershipTx(ctx, token, &finalMem)
	if err != nil {
		return fmt.Errorf("service.expireActiveRight transaction: %w", err)
	}

	if !transitioned {
		if right != nil {
			if errCache := s.cache.SetRight(ctx, right); errCache != nil {
				log.ErrorContext(ctx, "failed to refresh terminal right cache", slog.Any("error", errCache))
			}
			if right.Status == models.RightStatusExpired {
				s.syncCacheState(ctx, &finalMem, right)
			}
		}
		return nil
	}

	s.syncCacheState(ctx, &finalMem, right)

	if removeExpiryTimer {
		if errRemove := s.cache.RemoveFromExpiryTimer(ctx, mem.ProductID, mem.UserID); errRemove != nil {
			log.ErrorContext(ctx, "failed to remove expired right timer", slog.Any("error", errRemove))
		}
	}

	if returnedQty > 0 {
		if errRestore := s.cache.RestoreAvailableUnits(ctx, mem.ProductID, returnedQty); errRestore != nil {
			log.ErrorContext(ctx, "failed to restore expired right units", slog.Any("error", errRestore))
		}
	}

	if errAdvance := s.AdvanceQueue(ctx, mem.ProductID); errAdvance != nil {
		log.ErrorContext(ctx, "failed to advance queue after right expiration", slog.Any("error", errAdvance))
	}

	return nil
}

// syncCacheState is a DRY helper to update Redis and broadcast the state.
func (s *QueueService) syncCacheState(ctx context.Context, mem *models.QueueMembership, right *models.Right) {
	log := logger.FromContext(ctx)

	if right != nil {
		if err := s.cache.SetRight(ctx, right); err != nil {
			log.ErrorContext(ctx, "failed to cache right", slog.Any("error", err))
		}
	}
	if err := s.cache.SetMembership(ctx, mem); err != nil {
		log.ErrorContext(ctx, "failed to cache membership", slog.Any("error", err))
	}
	if mem.ExpiresAt != nil {
		expiryDeadline := *mem.ExpiresAt
		if mem.Status == models.MembershipStatusRightActive {
			expiryDeadline = s.rightHeartbeatDeadline(time.Now().UTC(), expiryDeadline)
		}
		if err := s.cache.AddToExpiryTimer(ctx, mem.ProductID, mem.UserID, expiryDeadline); err != nil {
			log.ErrorContext(ctx, "failed to add to expiry timer", slog.Any("error", err))
		}
	}
	if err := s.cache.PublishEvent(ctx, mem.ProductID, mem.UserID, map[string]string{"status": string(mem.Status)}); err != nil {
		log.ErrorContext(ctx, "failed to publish event", slog.Any("error", err))
	}
}

// AdvanceQueue acts as an internal engine to push the queue forward when stock frees up.
// It intentionally ignores the boolean soldOut flag returned by PopAndAllocate,
// relying exclusively on the strict models.MembershipStatus for state transitions.
func (s *QueueService) AdvanceQueue(ctx context.Context, productID string) error {
	for {
		uid, alloc, avail, _, status, score, err := s.cache.PopAndAllocate(ctx, productID)
		if err != nil {
			return fmt.Errorf("service.AdvanceQueue pop and allocate: %w", err)
		}

		if uid == "" || status == models.MembershipStatusQueued {
			break
		}

		if status != models.MembershipStatusRightActive &&
			status != models.MembershipStatusOfferPending &&
			status != models.MembershipStatusSoldOut {
			continue
		}

		claimOwner := uuid.NewString()
		won, errClaim := s.cache.ClaimMembership(
			ctx, productID, uid, claimOwner, membershipClaimTTL,
		)
		if errClaim != nil {
			errRollback := s.rollbackAdvance(ctx, productID, uid, alloc+avail, score)

			return fmt.Errorf("service.AdvanceQueue claim membership: %w",
				errors.Join(errClaim, errRollback))
		}
		if !won {
			errRollback := s.rollbackAdvance(ctx, productID, uid, alloc+avail, score)

			return fmt.Errorf("service.AdvanceQueue claim membership: %w",
				errors.Join(models.ErrConcurrentJoin, errRollback))
		}

		func() {
			defer func() {
				if errRelease := s.cache.ReleaseMembershipClaim(
					context.WithoutCancel(ctx), productID, uid, claimOwner,
				); errRelease != nil {
					logger.FromContext(ctx).WarnContext(ctx,
						"failed to release advance claim", slog.Any("error", errRelease))
				}
			}()

			s.applyAdvanceStep(ctx, productID, uid, alloc, avail, status, score)
		}()
	}

	return nil
}

// applyAdvanceStep processes a single iteration of the queue advancement logic,
// updating the durable store and cache based on the user's new status.
func (s *QueueService) applyAdvanceStep(ctx context.Context, productID, uid string, alloc, avail int, status models.MembershipStatus, score float64) {
	log := logger.FromContext(ctx)

	mem, err := s.cache.GetMembership(ctx, productID, uid)
	if err != nil {
		_ = s.rollbackAdvance(ctx, productID, uid, alloc+avail, score)
		log.ErrorContext(ctx, "failed to get membership for advanced user", slog.Any("error", err), slog.String("user_id", uid))
		return
	}

	now := time.Now().UTC()
	mem.UpdatedAt = now

	switch status {
	case models.MembershipStatusRightActive:
		right := &models.Right{
			Token:     uuid.NewString(),
			UserID:    mem.UserID,
			ProductID: mem.ProductID,
			Quantity:  alloc,
			Status:    models.RightStatusActive,
			CreatedAt: now,
			ExpiresAt: now.Add(s.paymentTTL),
		}

		if errSave := s.durable.SaveRight(ctx, right); errSave != nil {
			_ = s.rollbackAdvance(ctx, productID, uid, alloc, score)
			log.ErrorContext(ctx, "failed to save right in advance queue", slog.Any("error", errSave))
			return
		}

		mem.Status = models.MembershipStatusRightActive
		mem.CurrentToken = &right.Token
		mem.ExpiresAt = &right.ExpiresAt

		if errUpsert := s.durable.UpsertMembership(ctx, mem); errUpsert != nil {
			log.ErrorContext(ctx, "failed to upsert membership", slog.Any("error", errUpsert))
		}
		s.syncCacheState(ctx, mem, right)

	case models.MembershipStatusOfferPending:
		mem.Status = models.MembershipStatusOfferPending
		mem.AvailableQuantity = &avail

		exp := new(time.Time)
		*exp = now.Add(s.offerTTL)
		mem.ExpiresAt = exp

		if errUpsert := s.durable.UpsertMembership(ctx, mem); errUpsert != nil {
			_ = s.rollbackAdvance(ctx, productID, uid, avail, score)
			log.ErrorContext(ctx, "failed to upsert partial membership", slog.Any("error", errUpsert))
			return
		}
		s.syncCacheState(ctx, mem, nil)

	case models.MembershipStatusSoldOut:
		mem.Status = models.MembershipStatusSoldOut
		if errUpsert := s.durable.UpsertMembership(ctx, mem); errUpsert != nil {
			_ = s.rollbackAdvance(ctx, productID, uid, 0, score)
			log.ErrorContext(ctx, "failed to upsert sold_out membership", slog.Any("error", errUpsert))
			return
		}
		s.syncCacheState(ctx, mem, nil)
	}
}

// rollbackAdvance handles disaster recovery if the durable database fails during queue advancement.
func (s *QueueService) rollbackAdvance(
	ctx context.Context, productID, userID string, qty int, score float64,
) error {
	log := logger.FromContext(ctx)
	var rollbackErrors []error

	if qty > 0 {
		if err := s.cache.RestoreAvailableUnits(context.Background(), productID, qty); err != nil {
			log.ErrorContext(ctx, "CRITICAL: failed to restore units on advance rollback", slog.Any("error", err))
			rollbackErrors = append(rollbackErrors, fmt.Errorf("restore units: %w", err))
		}
	}

	if err := s.cache.Requeue(context.Background(), productID, userID, score); err != nil {
		log.ErrorContext(ctx, "CRITICAL: failed to requeue user on advance rollback", slog.Any("error", err))
		rollbackErrors = append(rollbackErrors, fmt.Errorf("requeue user: %w", err))
	}

	return errors.Join(rollbackErrors...)
}

func (s *QueueService) getActiveRight(ctx context.Context, token string) (*models.Right, error) {
	log := logger.FromContext(ctx)

	right, err := s.cache.GetRight(ctx, token)
	if errors.Is(err, models.ErrTokenNotFound) {
		right, err = s.durable.GetRightByToken(ctx, token)
		if err == nil {
			if errCache := s.cache.SetRight(ctx, right); errCache != nil {
				log.ErrorContext(ctx, "failed to restore right cache", slog.Any("error", errCache))
			}
		}
	}
	if err != nil {
		return nil, fmt.Errorf("service.getActiveRight fetch: %w", err)
	}

	switch right.Status {
	case models.RightStatusUsed:
		return nil, models.ErrTokenUsed
	case models.RightStatusExpired:
		return nil, models.ErrTokenExpired
	case models.RightStatusActive:
		// Continue with the time-bound validation.
	default:
		return nil, models.ErrInvalidStatus
	}

	if !time.Now().UTC().Before(right.ExpiresAt) {
		return nil, models.ErrTokenExpired
	}

	return right, nil
}

// ValidateRight validates a purchase right before allowing the user to proceed to checkout.
// It strictly checks ownership, expiration, and status to prevent fraud, and uses a database
// fallback in case of a cache miss.
func (s *QueueService) ValidateRight(ctx context.Context, token string, userID string) (*models.Right, error) {
	right, err := s.getActiveRight(ctx, token)
	if err != nil {
		return nil, err
	}

	if right.UserID != userID {
		return nil, models.ErrForbidden
	}

	return right, nil
}

// ValidateRightForCheckout validates the token from AvitoBackend before it
// creates an order. At this point the caller is a trusted service, so the product
// binding is the critical anti-bypass check.
func (s *QueueService) ValidateRightForCheckout(ctx context.Context, token string, productID string) (*models.Right, error) {
	right, err := s.getActiveRight(ctx, token)
	if err != nil {
		return nil, err
	}

	if right.ProductID != productID {
		return nil, models.ErrForbidden
	}

	return right, nil
}

// ProcessPayment handles the asynchronous webhook from the payment gateway.
// It is idempotent, persists the Avito stock decrement into the outbox, and
// triggers queue advancement after the local purchase transition.
func (s *QueueService) ProcessPayment(ctx context.Context, token string, orderID string) error {
	log := logger.FromContext(ctx)
	now := time.Now().UTC()

	right, transitioned, err := s.durable.UseRightTx(ctx, token, orderID, now)
	if err != nil {
		return fmt.Errorf("service.ProcessPayment transaction: %w", err)
	}

	if transitioned {
		if errCommit := s.cache.CommitPurchase(context.Background(), right.ProductID, right.Quantity); errCommit != nil {
			log.ErrorContext(ctx, "failed to commit physical purchase in cache", slog.Any("error", errCommit))
		}
	}

	if errCacheRight := s.cache.SetRight(ctx, right); errCacheRight != nil {
		log.ErrorContext(ctx, "failed to refresh used right cache", slog.Any("error", errCacheRight))
	}

	if _, errCacheMembership := s.cache.MarkPurchasedIfCurrentToken(ctx, right, now); errCacheMembership != nil {
		log.ErrorContext(ctx, "failed to mark cached membership as purchased", slog.Any("error", errCacheMembership))
	}

	if transitioned {
		if errAdvance := s.AdvanceQueue(context.Background(), right.ProductID); errAdvance != nil {
			log.ErrorContext(ctx, "failed to advance queue after successful payment", slog.Any("error", errAdvance))
		}
	}

	return nil
}

// ProcessExpirations scans for expired offers or payment rights, rolls back their stock,
// parseExpiredKey is a small helper to split the "productID:userID" string.
func parseExpiredKey(key string) (string, string, bool) {
	for i := 0; i < len(key); i++ {
		if key[i] == ':' {
			return key[:i], key[i+1:], true
		}
	}
	return "", "", false
}

// awaitConcurrentAccept serves the accept request that lost the claim.
//
// A double click on «take N» should not punish the user: if the winner already
// turned the offer into a right, that right is returned as is. Only when the
// offer is gone without a right behind it does this report a conflict.
func (s *QueueService) awaitConcurrentAccept(
	ctx context.Context, productID, userID string,
) (*models.Right, error) {
	for attempt := 0; attempt < joinAwaitAttempts; attempt++ {
		mem, err := s.cache.GetMembership(ctx, productID, userID)
		if err != nil {
			return nil, fmt.Errorf("service.AcceptOffer await: %w", err)
		}

		if mem.Status == models.MembershipStatusRightActive && mem.CurrentToken != nil {
			right, errRight := s.cache.GetRight(ctx, *mem.CurrentToken)
			if errRight != nil {
				return nil, fmt.Errorf("service.AcceptOffer await right: %w", errRight)
			}

			return right, nil
		}

		// The offer is no longer pending and no right came out of it: the winner
		// declined it, or it expired. There is nothing left to accept.
		if mem.Status != models.MembershipStatusOfferPending {
			return nil, models.ErrInvalidStatus
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(joinAwaitDelay):
		}
	}

	return nil, models.ErrConcurrentJoin
}
