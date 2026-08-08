package service_test

import (
	"backend/internal/models"

	"github.com/stretchr/testify/require"
)

func (s *QueueServiceTestSuite) TestLeaveQueue_Queued() {
	s.mockMembershipFetch(models.MembershipStatusQueued, nil)
	s.mockDurableUpsert(models.MembershipStatusDeclined, nil)
	s.mockSyncCacheState(models.MembershipStatusDeclined, false, false)
	s.mockCache.EXPECT().RemoveFromQueue(s.ctx, "prod-1", "user-1").Return(nil)

	err := s.srv.LeaveQueue(s.ctx, "prod-1", "user-1")

	require.NoError(s.T(), err)
}

func (s *QueueServiceTestSuite) TestLeaveQueue_RightActive() {
	token := "right-token"
	s.mockCache.EXPECT().GetMembership(s.ctx, "prod-1", "user-1").Return(&models.QueueMembership{
		ProductID:    "prod-1",
		UserID:       "user-1",
		Status:       models.MembershipStatusRightActive,
		Quantity:     3,
		CurrentToken: &token,
	}, nil)
	s.mockDurableUpsert(models.MembershipStatusDeclined, nil)
	s.mockSyncCacheState(models.MembershipStatusDeclined, false, false)
	s.mockCache.EXPECT().RemoveFromExpiryTimer(s.ctx, "prod-1", "user-1").Return(nil)
	s.mockCache.EXPECT().RestoreAvailableUnits(s.ctx, "prod-1", 3).Return(nil)
	s.mockCache.EXPECT().PopAndAllocate(s.ctx, "prod-1").Return(
		"", 0, 0, false, models.MembershipStatus(""), 0.0, nil,
	)

	err := s.srv.LeaveQueue(s.ctx, "prod-1", "user-1")

	require.NoError(s.T(), err)
}

func (s *QueueServiceTestSuite) TestLeaveQueue_TerminalStatus() {
	s.mockMembershipFetch(models.MembershipStatusPurchased, nil)

	err := s.srv.LeaveQueue(s.ctx, "prod-1", "user-1")

	require.ErrorIs(s.T(), err, models.ErrInvalidStatus)
}
