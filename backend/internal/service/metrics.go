package service

import (
	"context"
	"fmt"

	"backend/internal/models"
)

// GetProductMetrics orchestrates the collection of analytical data for a specific product.
// It relies exclusively on durable storage to guarantee data consistency for the seller's dashboard.
// Returns models.ErrInvalidRequest if the provided productID is empty.
func (s *QueueService) GetProductMetrics(ctx context.Context, productID string) (*models.ProductMetrics, error) {
	if productID == "" {
		return nil, fmt.Errorf("service.GetProductMetrics validation: %w", models.ErrInvalidRequest)
	}

	// TODO: add redis cache
	metrics, err := s.durable.GetProductMetrics(ctx, productID)
	if err != nil {
		return nil, fmt.Errorf("service.GetProductMetrics durable fetch: %w", err)
	}

	return metrics, nil
}
