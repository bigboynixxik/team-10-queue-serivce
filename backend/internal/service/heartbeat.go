package service

import (
	"context"
	"fmt"
	"time"

	"backend/internal/models"
)

// RefreshRightHeartbeat extends the short-lived Redis lease of an active right.
// The durable right expiration remains the hard upper bound.
func (s *QueueService) RefreshRightHeartbeat(ctx context.Context, productID, userID string) error {
	membership, err := s.cache.GetMembership(ctx, productID, userID)
	if err != nil {
		return fmt.Errorf("service.RefreshRightHeartbeat membership: %w", err)
	}
	if membership == nil {
		return fmt.Errorf("service.RefreshRightHeartbeat membership: %w", models.ErrMembershipNotFound)
	}
	if membership.Status != models.MembershipStatusRightActive ||
		membership.CurrentToken == nil ||
		membership.ExpiresAt == nil {
		return fmt.Errorf("service.RefreshRightHeartbeat state: %w", models.ErrInvalidStatus)
	}

	now := time.Now().UTC()
	if !now.Before(*membership.ExpiresAt) {
		return fmt.Errorf("service.RefreshRightHeartbeat expired: %w", models.ErrTokenExpired)
	}

	deadline := s.rightHeartbeatDeadline(now, *membership.ExpiresAt)
	refreshed, err := s.cache.RefreshExpiryTimer(ctx, productID, userID, deadline)
	if err != nil {
		return fmt.Errorf("service.RefreshRightHeartbeat refresh timer: %w", err)
	}
	if !refreshed {
		return fmt.Errorf("service.RefreshRightHeartbeat timer claimed: %w", models.ErrInvalidStatus)
	}

	return nil
}

func (s *QueueService) rightHeartbeatDeadline(now, rightExpiresAt time.Time) time.Time {
	heartbeatDeadline := now.Add(s.heartbeatTimeout)
	if rightExpiresAt.Before(heartbeatDeadline) {
		return rightExpiresAt
	}

	return heartbeatDeadline
}
