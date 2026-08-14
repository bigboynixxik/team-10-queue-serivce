package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"backend/internal/models"

	"github.com/google/uuid"
)

const presenceClaimProduct = "__user_presence__"

// RefreshUserPresence extends the waiting lease of every queue the user is in.
// Offers and purchase rights have their own fixed business deadlines.
func (s *QueueService) RefreshUserPresence(ctx context.Context, userID string) error {
	claimOwner := uuid.NewString()
	won, errClaim := s.cache.ClaimMembership(
		ctx, presenceClaimProduct, userID, claimOwner, membershipClaimTTL,
	)
	if errClaim != nil {
		return fmt.Errorf("service.RefreshUserPresence claim: %w", errClaim)
	}
	if !won {
		// Another tab heartbeat or the expiration worker is already deciding the
		// same presence generation. Keeping this connection open is safe; the next
		// probe will retry.
		return nil
	}
	defer func() {
		_ = s.cache.ReleaseMembershipClaim(
			context.WithoutCancel(ctx), presenceClaimProduct, userID, claimOwner,
		)
	}()

	productIDs, err := s.cache.ListQueuedProducts(ctx, userID)
	if err != nil {
		return fmt.Errorf("service.RefreshUserPresence list queues: %w", err)
	}

	deadline := time.Now().UTC().Add(s.presenceTimeout)
	if err := s.cache.SetUserPresenceDeadline(ctx, userID, deadline); err != nil {
		return fmt.Errorf("service.RefreshUserPresence set deadline: %w", err)
	}

	var refreshErrors []error
	for _, productID := range productIDs {
		membership, errMembership := s.cache.GetMembership(ctx, productID, userID)
		if errMembership != nil {
			if errors.Is(errMembership, models.ErrTokenNotFound) ||
				errors.Is(errMembership, models.ErrMembershipNotFound) {
				if errRemove := s.cache.RemoveFromQueue(ctx, productID, userID); errRemove != nil {
					refreshErrors = append(refreshErrors, fmt.Errorf("%s remove missing membership: %w", productID, errRemove))
				}
				continue
			}
			refreshErrors = append(refreshErrors, fmt.Errorf("%s membership: %w", productID, errMembership))
			continue
		}
		if membership == nil || membership.Status != models.MembershipStatusQueued {
			if errRemove := s.cache.RemoveFromQueue(ctx, productID, userID); errRemove != nil {
				refreshErrors = append(refreshErrors, fmt.Errorf("%s remove stale queue index: %w", productID, errRemove))
			}
			continue
		}

		refreshed, errRefresh := s.cache.RefreshExpiryTimer(ctx, productID, userID, deadline)
		if errRefresh != nil {
			refreshErrors = append(refreshErrors, fmt.Errorf("%s refresh timer: %w", productID, errRefresh))
			continue
		}
		if !refreshed {
			// The expiration worker may own the previous generation. It will observe
			// the shared presence deadline under the same user claim and reschedule it.
			continue
		}
	}

	if err := errors.Join(refreshErrors...); err != nil {
		return fmt.Errorf("service.RefreshUserPresence: %w", err)
	}

	return nil
}
