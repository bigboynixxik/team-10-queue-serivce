package service

import (
	"context"
	"fmt"

	"backend/internal/models"
)

// GetQueueStats reports the demand for a product: who is waiting, who is
// mid-purchase, and how much is left.
//
// The stock comes from the cache and the head counts from Postgres, so the two
// halves may disagree by a moment under load. That is acceptable here: this is a
// reporting read, and nothing is decided on its result — allocation still happens
// atomically in Redis (docs/storage/postgres.md).
func (s *QueueService) GetQueueStats(ctx context.Context, productID string) (*models.QueueStats, error) {
	productCount, available, err := s.cache.GetStock(ctx, productID)
	if err != nil {
		return nil, fmt.Errorf("service.GetQueueStats stock: %w", err)
	}

	counts, err := s.durable.CountMembershipsByStatus(ctx, productID)
	if err != nil {
		return nil, fmt.Errorf("service.GetQueueStats counts: %w", err)
	}

	return &models.QueueStats{
		Waiting:      counts[models.MembershipStatusQueued],
		HoldingRight: counts[models.MembershipStatusRightActive],
		PendingOffer: counts[models.MembershipStatusOfferPending],
		Available:    available,
		ProductCount: productCount,
	}, nil
}
