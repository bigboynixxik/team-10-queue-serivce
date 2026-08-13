// Package service_test provides behavioral tests for the queue service business logic.
package service_test

import (
	"time"

	"backend/internal/models"
)

// TestGetProductMetrics_ValidationFailure verifies that an empty product ID
// immediately returns an invalid request error without querying the repository.
func (s *QueueServiceTestSuite) TestGetProductMetrics_ValidationFailure() {
	metrics, err := s.srv.GetProductMetrics(s.ctx, "")

	s.Require().ErrorIs(err, models.ErrInvalidRequest)
	s.Nil(metrics)
}

// TestGetProductMetrics_RepositoryError verifies that errors returned by the
// durable repository (e.g., product not found) are properly propagated to the caller.
func (s *QueueServiceTestSuite) TestGetProductMetrics_RepositoryError() {
	productID := "prod-unknown"

	s.mockDurable.EXPECT().
		GetProductMetrics(s.ctx, productID).
		Return(nil, models.ErrProductNotFound)

	metrics, err := s.srv.GetProductMetrics(s.ctx, productID)

	s.Require().ErrorIs(err, models.ErrProductNotFound)
	s.Nil(metrics)
}

// TestGetProductMetrics_Success verifies that a successful retrieval of product
// analytics from the durable repository is returned transparently.
func (s *QueueServiceTestSuite) TestGetProductMetrics_Success() {
	productID := "prod-success"
	expectedMetrics := &models.ProductMetrics{
		TotalStock:         100,
		TotalContenders:    8,
		UsedRightsCount:    2,
		ExpiredRightsCount: 1,
		SoldOutCount:       3,
		DropOffCount:       2,
		AvgPaymentTime:     new(20 * time.Second),
		AvgDropOffTime:     new(50 * time.Second),
	}

	s.mockDurable.EXPECT().
		GetProductMetrics(s.ctx, productID).
		Return(expectedMetrics, nil)

	metrics, err := s.srv.GetProductMetrics(s.ctx, productID)

	s.Require().NoError(err)
	s.Equal(expectedMetrics, metrics)
}
