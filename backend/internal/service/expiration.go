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

const (
	// expirationLease is how long a worker may hold a claimed timer before it is
	// considered abandoned. It has to comfortably outlive one handling pass:
	// reclaiming too eagerly would let two workers act on the same timer.
	expirationLease = 30 * time.Second

	// expirationBatchSize bounds one pass so a large backlog is worked through
	// gradually instead of being held under a single lease that may run out
	// halfway.
	expirationBatchSize = 100

	// expirationRetryDelay postpones a failed item, giving whatever broke a moment
	// to recover instead of spinning on it every tick.
	expirationRetryDelay = time.Second
)

// ProcessExpirations moves the state machine forward on time: it returns expired
// rights to the queue and drops offers nobody answered.
//
// Timers are claimed under a lease rather than deleted. If this process dies
// mid-pass, the claim expires and the next pass picks the work up again — before,
// such timers were gone for good, leaving a right ACTIVE with nothing left to
// expire it and its unit permanently out of circulation.
func (s *QueueService) ProcessExpirations(ctx context.Context) error {
	log := logger.FromContext(ctx)
	now := time.Now().UTC()

	// Work abandoned by a dead worker comes first: it has been waiting longest.
	if rescued, err := s.cache.ReclaimStaleExpired(ctx, now); err != nil {
		log.ErrorContext(ctx, "failed to reclaim stale expirations", slog.Any("error", err))
	} else if rescued > 0 {
		log.WarnContext(ctx, "reclaimed abandoned expirations", slog.Int("count", rescued))
	}

	claimed, err := s.cache.ClaimExpired(ctx, now, expirationLease, expirationBatchSize)
	if err != nil {
		return fmt.Errorf("service.ProcessExpirations claim: %w", err)
	}

	if len(claimed) == 0 {
		return nil
	}

	handled := make([]models.ExpiryClaim, 0, len(claimed))
	failed := make([]models.ExpiryClaim, 0)

	for _, claim := range claimed {
		if errHandle := s.handleExpiredKey(ctx, claim, now); errHandle != nil {
			log.ErrorContext(ctx, "failed to handle expiration",
				slog.String("key", claim.Key), slog.Any("error", errHandle))
			failed = append(failed, claim)

			continue
		}

		handled = append(handled, claim)
	}

	if len(handled) > 0 {
		if errAck := s.cache.AckExpired(ctx, handled); errAck != nil {
			// The items stay claimed and return to the schedule once the lease runs
			// out, so nothing is lost — they are merely handled twice, which is
			// harmless for every transition here.
			log.ErrorContext(ctx, "failed to acknowledge expirations", slog.Any("error", errAck))
		}
	}

	if len(failed) > 0 {
		if errNack := s.cache.NackExpired(ctx, failed, now.Add(expirationRetryDelay)); errNack != nil {
			log.ErrorContext(ctx, "failed to reschedule failed expirations", slog.Any("error", errNack))
		}
	}

	return nil
}

// handleExpiredKey applies one expired timer. Returning an error means the item
// should be retried; returning nil means it is settled, including the cases where
// the timer turned out to be irrelevant.
func (s *QueueService) handleExpiredKey(
	ctx context.Context, claim models.ExpiryClaim, now time.Time,
) error {
	log := logger.FromContext(ctx)

	productID, userID, found := parseExpiredKey(claim.Key)
	if !found {
		// A malformed key will never parse, so retrying it forever is pointless.
		log.ErrorContext(ctx, "malformed expired key format", slog.String("key", claim.Key))

		return nil
	}

	claimOwner := uuid.NewString()
	won, errClaim := s.cache.ClaimMembership(
		ctx, productID, userID, claimOwner, membershipClaimTTL,
	)
	if errClaim != nil {
		return fmt.Errorf("claim membership: %w", errClaim)
	}
	if !won {
		return models.ErrConcurrentJoin
	}
	defer func() {
		if errRelease := s.cache.ReleaseMembershipClaim(
			context.WithoutCancel(ctx), productID, userID, claimOwner,
		); errRelease != nil {
			log.WarnContext(ctx, "failed to release expiration claim", slog.Any("error", errRelease))
		}
	}()

	mem, err := s.cache.GetMembership(ctx, productID, userID)
	if err != nil {
		return fmt.Errorf("get membership: %w", err)
	}

	// The user has already paid or left; the timer refers to a state that no
	// longer exists.
	if mem.Status != models.MembershipStatusOfferPending && mem.Status != models.MembershipStatusRightActive {
		return nil
	}
	if mem.ExpiresAt == nil {
		return fmt.Errorf("active membership has no expiration: %w", models.ErrInvalidStatus)
	}

	// A timer is identified by product and user, so an old claimed timer may
	// survive while the same user enters a newer lifecycle. UpdatedAt belongs to
	// the current state, while Deadline belongs to the claimed state. A newer
	// state must never be expired by the older claim. This comparison deliberately
	// does not use ExpiresAt: RIGHT_ACTIVE may validly expire earlier because its
	// heartbeat lease ran out.
	if claim.Deadline.Before(mem.UpdatedAt) {
		deadline := *mem.ExpiresAt
		if mem.Status == models.MembershipStatusRightActive {
			deadline = s.rightHeartbeatDeadline(now, deadline)
		}

		refreshed, errRefresh := s.cache.RefreshExpiryTimer(
			ctx, productID, userID, deadline,
		)
		if errRefresh != nil {
			return fmt.Errorf("refresh current expiration: %w", errRefresh)
		}
		if !refreshed {
			if errAdd := s.cache.AddToExpiryTimer(ctx, productID, userID, deadline); errAdd != nil {
				return fmt.Errorf("restore current expiration: %w", errAdd)
			}
		}

		return nil
	}

	if mem.Status == models.MembershipStatusRightActive {
		if errExpire := s.expireActiveRight(ctx, mem, false); errExpire != nil {
			// A right already claimed by another path is settled, not failed.
			if errors.Is(errExpire, models.ErrInvalidStatus) {
				return nil
			}

			return fmt.Errorf("expire active right: %w", errExpire)
		}

		return nil
	}

	return s.expirePendingOffer(ctx, mem, now)
}

// expirePendingOffer turns an unanswered offer into the terminal DECLINED and
// returns the units it was holding.
//
// Silence on a partial offer reads as "not interested", which is why it does not
// send the user back to the queue the way an expired right does
// (docs/design_context.md, пп. 3–4).
func (s *QueueService) expirePendingOffer(
	ctx context.Context, mem *models.QueueMembership, now time.Time,
) error {
	if mem.AvailableQuantity == nil {
		return fmt.Errorf("expired offer for user %s has no available quantity: %w",
			mem.UserID, models.ErrInvalidStatus)
	}

	productID := mem.ProductID
	returnedQty := *mem.AvailableQuantity

	mem.Status = models.MembershipStatusDeclined
	mem.AvailableQuantity = nil
	mem.CurrentToken = nil
	mem.ExpiresAt = nil
	mem.UpdatedAt = now

	if err := s.durable.UpsertMembership(ctx, mem); err != nil {
		return fmt.Errorf("upsert expired membership: %w", err)
	}

	var operationErrors []error
	if errSync := s.syncCacheState(ctx, mem, nil); errSync != nil {
		operationErrors = append(operationErrors, fmt.Errorf("sync expired offer: %w", errSync))
	}

	if returnedQty > 0 {
		if errRestore := s.cache.RestoreAvailableUnits(ctx, productID, returnedQty); errRestore != nil {
			operationErrors = append(operationErrors, fmt.Errorf("restore expired offer units: %w", errRestore))
		}
	}

	if errAdvance := s.AdvanceQueue(ctx, productID); errAdvance != nil {
		operationErrors = append(operationErrors, fmt.Errorf("advance queue: %w", errAdvance))
	}

	if errJoined := errors.Join(operationErrors...); errJoined != nil {
		return fmt.Errorf("expire pending offer cache reconciliation: %w", errJoined)
	}

	return nil
}
