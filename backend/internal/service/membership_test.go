package service_test

import (
	"errors"
	"time"

	"backend/internal/models"
	"backend/internal/service"
)

// TestCalculateETA_HappyPath verifies that ETA is accurately calculated
// when a user is in the queue and there are fewer available units than their position.
func (s *QueueServiceTestSuite) TestCalculateETA_HappyPath() {
	avgPaymentTime := 75 * time.Second
	srv := service.NewQueueService(s.mockDurable, s.mockCache, s.mockAvito, time.Minute, time.Minute, avgPaymentTime, 30*time.Second)

	s.mockCache.EXPECT().GetQueueMetrics(s.ctx, "prod-1", "user-1").Return(4, 2, nil)

	pos, eta, err := srv.CalculateETA(s.ctx, "prod-1", "user-1")

	s.Require().NoError(err)
	s.Equal(5, pos)
	s.Equal(3*avgPaymentTime, eta)
}

// TestCalculateETA_BestCase verifies that if the available stock exceeds
// the user's position, the estimated wait time is zero.
func (s *QueueServiceTestSuite) TestCalculateETA_BestCase() {
	avgPaymentTime := 75 * time.Second
	srv := service.NewQueueService(s.mockDurable, s.mockCache, s.mockAvito, time.Minute, time.Minute, avgPaymentTime, 30*time.Second)

	s.mockCache.EXPECT().GetQueueMetrics(s.ctx, "prod-2", "user-2").Return(1, 5, nil)

	pos, eta, err := srv.CalculateETA(s.ctx, "prod-2", "user-2")

	s.Require().NoError(err)
	s.Equal(2, pos)
	s.Equal(time.Duration(0), eta)
}

// TestCalculateETA_NotInQueue verifies that domain errors representing a missing
// user are properly wrapped and propagated for the transport layer to handle.
func (s *QueueServiceTestSuite) TestCalculateETA_NotInQueue() {
	avgPaymentTime := 75 * time.Second
	srv := service.NewQueueService(s.mockDurable, s.mockCache, s.mockAvito, time.Minute, time.Minute, avgPaymentTime, 30*time.Second)

	s.mockCache.EXPECT().GetQueueMetrics(s.ctx, "prod-3", "ghost").Return(0, 0, models.ErrMembershipNotFound)

	pos, eta, err := srv.CalculateETA(s.ctx, "prod-3", "ghost")

	s.Require().Error(err)
	s.Require().ErrorIs(err, models.ErrMembershipNotFound)
	s.Equal(0, pos)
	s.Equal(time.Duration(0), eta)
}

// TestCalculateETA_InfrastructureError verifies that generic infrastructure
// errors from the cache are securely wrapped and propagated.
func (s *QueueServiceTestSuite) TestCalculateETA_InfrastructureError() {
	avgPaymentTime := 75 * time.Second
	srv := service.NewQueueService(s.mockDurable, s.mockCache, s.mockAvito, time.Minute, time.Minute, avgPaymentTime, 30*time.Second)
	infraErr := errors.New("redis timeout")

	s.mockCache.EXPECT().GetQueueMetrics(s.ctx, "prod-4", "user-4").Return(0, 0, infraErr)

	pos, eta, err := srv.CalculateETA(s.ctx, "prod-4", "user-4")

	s.Require().Error(err)
	s.Require().ErrorContains(err, "service.CalculateETA get metrics")
	s.Require().ErrorIs(err, infraErr)
	s.Equal(0, pos)
	s.Equal(time.Duration(0), eta)
}
