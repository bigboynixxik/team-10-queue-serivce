package service_test

import (
	"time"

	"backend/internal/models"

	"go.uber.org/mock/gomock"
)

func (s *QueueServiceTestSuite) TestRefreshRightHeartbeat_ExtendsActiveLease() {
	token := "right-token"
	expiresAt := time.Now().UTC().Add(2 * time.Minute)
	membership := &models.QueueMembership{
		ProductID:    "prod-1",
		UserID:       "user-1",
		Status:       models.MembershipStatusRightActive,
		CurrentToken: &token,
		ExpiresAt:    &expiresAt,
	}
	earliest := time.Now().UTC().Add(29 * time.Second)
	latest := time.Now().UTC().Add(31 * time.Second)

	s.mockCache.EXPECT().GetMembership(s.ctx, "prod-1", "user-1").Return(membership, nil)
	s.mockCache.EXPECT().RefreshExpiryTimer(
		s.ctx,
		"prod-1",
		"user-1",
		gomock.Cond(func(deadline time.Time) bool {
			return !deadline.Before(earliest) && !deadline.After(latest)
		}),
	).Return(true, nil)

	err := s.srv.RefreshRightHeartbeat(s.ctx, "prod-1", "user-1")

	s.Require().NoError(err)
}

func (s *QueueServiceTestSuite) TestRefreshRightHeartbeat_CapsLeaseAtRightExpiration() {
	token := "right-token"
	expiresAt := time.Now().UTC().Add(5 * time.Second)
	membership := &models.QueueMembership{
		ProductID:    "prod-1",
		UserID:       "user-1",
		Status:       models.MembershipStatusRightActive,
		CurrentToken: &token,
		ExpiresAt:    &expiresAt,
	}

	s.mockCache.EXPECT().GetMembership(s.ctx, "prod-1", "user-1").Return(membership, nil)
	s.mockCache.EXPECT().
		RefreshExpiryTimer(s.ctx, "prod-1", "user-1", expiresAt).
		Return(true, nil)

	err := s.srv.RefreshRightHeartbeat(s.ctx, "prod-1", "user-1")

	s.Require().NoError(err)
}

func (s *QueueServiceTestSuite) TestRefreshRightHeartbeat_RejectsNonActiveMembership() {
	membership := &models.QueueMembership{
		ProductID: "prod-1",
		UserID:    "user-1",
		Status:    models.MembershipStatusQueued,
	}
	s.mockCache.EXPECT().GetMembership(s.ctx, "prod-1", "user-1").Return(membership, nil)

	err := s.srv.RefreshRightHeartbeat(s.ctx, "prod-1", "user-1")

	s.Require().ErrorIs(err, models.ErrInvalidStatus)
}

func (s *QueueServiceTestSuite) TestRefreshRightHeartbeat_DoesNotResurrectClaimedTimer() {
	token := "right-token"
	expiresAt := time.Now().UTC().Add(time.Minute)
	membership := &models.QueueMembership{
		ProductID:    "prod-1",
		UserID:       "user-1",
		Status:       models.MembershipStatusRightActive,
		CurrentToken: &token,
		ExpiresAt:    &expiresAt,
	}

	s.mockCache.EXPECT().GetMembership(s.ctx, "prod-1", "user-1").Return(membership, nil)
	s.mockCache.EXPECT().
		RefreshExpiryTimer(s.ctx, "prod-1", "user-1", gomock.Any()).
		Return(false, nil)

	err := s.srv.RefreshRightHeartbeat(s.ctx, "prod-1", "user-1")

	s.Require().ErrorIs(err, models.ErrInvalidStatus)
}

func (s *QueueServiceTestSuite) TestRefreshRightHeartbeat_RejectsExpiredRight() {
	token := "right-token"
	expiresAt := time.Now().UTC().Add(-time.Second)
	membership := &models.QueueMembership{
		ProductID:    "prod-1",
		UserID:       "user-1",
		Status:       models.MembershipStatusRightActive,
		CurrentToken: &token,
		ExpiresAt:    &expiresAt,
	}
	s.mockCache.EXPECT().GetMembership(s.ctx, "prod-1", "user-1").Return(membership, nil)

	err := s.srv.RefreshRightHeartbeat(s.ctx, "prod-1", "user-1")

	s.Require().ErrorIs(err, models.ErrTokenExpired)
}
