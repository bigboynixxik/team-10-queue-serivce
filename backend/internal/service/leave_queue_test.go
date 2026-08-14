package service_test

import (
	"backend/internal/models"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func (s *QueueServiceTestSuite) TestLeaveQueue_Queued() {
	s.expectMembershipClaim("prod-1", "user-1")
	s.mockMembershipFetch(models.MembershipStatusQueued, nil)
	s.mockDurableUpsert(models.MembershipStatusDeclined, nil)
	s.mockSyncCacheState(models.MembershipStatusDeclined, false, false)
	s.mockCache.EXPECT().RemoveFromQueue(s.ctx, "prod-1", "user-1").Return(nil)

	err := s.srv.LeaveQueue(s.ctx, "prod-1", "user-1")

	require.NoError(s.T(), err)
}

func (s *QueueServiceTestSuite) TestLeaveQueue_RightActive() {
	s.expectMembershipClaim("prod-1", "user-1")
	token := "right-token"
	mem := &models.QueueMembership{
		ProductID:    "prod-1",
		UserID:       "user-1",
		Status:       models.MembershipStatusRightActive,
		Quantity:     3,
		CurrentToken: &token,
	}
	expiredRight := &models.Right{
		Token:     token,
		UserID:    "user-1",
		ProductID: "prod-1",
		Quantity:  3,
		Status:    models.RightStatusExpired,
	}

	s.mockCache.EXPECT().GetMembership(s.ctx, "prod-1", "user-1").Return(mem, nil)
	s.mockDurable.EXPECT().ExpireRightAndUpsertMembershipTx(s.ctx, token, gomock.Cond(func(value any) bool {
		membership, ok := value.(*models.QueueMembership)
		return ok && membership.Status == models.MembershipStatusDeclined &&
			membership.CurrentToken == nil && membership.ExpiresAt == nil
	})).Return(expiredRight, true, nil)
	s.mockCache.EXPECT().SetRight(s.ctx, expiredRight).Return(nil)
	s.mockCache.EXPECT().SetMembership(s.ctx, gomock.Any()).Return(nil)
	s.mockCache.EXPECT().PublishEvent(s.ctx, "prod-1", "user-1", map[string]string{"status": "DECLINED"}).Return(nil)
	s.mockCache.EXPECT().RemoveFromExpiryTimer(s.ctx, "prod-1", "user-1").Return(nil)
	s.mockCache.EXPECT().RestoreAvailableUnits(s.ctx, "prod-1", 3).Return(nil)
	s.mockCache.EXPECT().PopAndAllocate(s.ctx, "prod-1").Return(
		"", 0, 0, false, models.MembershipStatus(""), 0.0, nil,
	)

	err := s.srv.LeaveQueue(s.ctx, "prod-1", "user-1")

	require.NoError(s.T(), err)
}

func (s *QueueServiceTestSuite) TestLeaveQueue_TerminalStatus() {
	s.expectMembershipClaim("prod-1", "user-1")
	s.mockMembershipFetch(models.MembershipStatusPurchased, nil)

	err := s.srv.LeaveQueue(s.ctx, "prod-1", "user-1")

	require.ErrorIs(s.T(), err, models.ErrInvalidStatus)
}

func (s *QueueServiceTestSuite) TestLeaveQueue_ConcurrentTransitionKeepsCurrentState() {
	s.mockCache.EXPECT().ClaimMembership(
		gomock.Any(), "prod-1", "user-1", gomock.Any(), gomock.Any(),
	).Return(false, nil)

	err := s.srv.LeaveQueue(s.ctx, "prod-1", "user-1")

	require.ErrorIs(s.T(), err, models.ErrConcurrentJoin)
}
