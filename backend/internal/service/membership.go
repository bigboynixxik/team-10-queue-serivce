package service

import (
	"context"
	"fmt"
	"time"

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

// CalculateETA computes the user's human-readable position in the queue (1-indexed)
// and the estimated wait time before they receive an offer or right.
func (s *QueueService) CalculateETA(ctx context.Context, productID, userID string) (int, time.Duration, error) {
	rank, availableUnits, err := s.cache.GetQueueMetrics(ctx, productID, userID)
	if err != nil {
		return 0, 0, fmt.Errorf("service.CalculateETA get metrics: %w", err)
	}

	position := rank + 1
	effectiveWaiters := position - availableUnits

	if effectiveWaiters <= 0 {
		return position, 0, nil
	}

	eta := time.Duration(effectiveWaiters) * s.avgPaymentTime

	return position, eta, nil
}
