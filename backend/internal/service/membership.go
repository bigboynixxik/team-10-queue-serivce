package service

import (
	"context"
	"fmt"

	"backend/internal/models"
)

// GetMembership returns the user's current state in the queue — what
// GET /queue/{product_id}/members/me answers and what the realtime channel
// pushes on every change.
//
// It reads the cache only: the durable copy in Postgres exists for restarts and
// support, not for the hot read path (docs/storage/postgres.md).
func (s *QueueService) GetMembership(ctx context.Context, productID, userID string) (*models.QueueMembership, error) {
	mem, err := s.cache.GetMembership(ctx, productID, userID)
	if err != nil {
		return nil, fmt.Errorf("service.GetMembership: %w", err)
	}

	return mem, nil
}
