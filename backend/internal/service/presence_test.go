package service_test

import (
	"time"

	"backend/internal/models"

	"go.uber.org/mock/gomock"
)

func (s *QueueServiceTestSuite) TestRefreshUserPresence_ExtendsEveryQueuedLease() {
	queued := &models.QueueMembership{ProductID: "prod-1", UserID: "user-1", Status: models.MembershipStatusQueued}
	right := &models.QueueMembership{ProductID: "prod-2", UserID: "user-1", Status: models.MembershipStatusRightActive}

	s.mockCache.EXPECT().ClaimMembership(
		s.ctx, "__user_presence__", "user-1", gomock.Any(), gomock.Any(),
	).Return(true, nil)
	s.mockCache.EXPECT().ReleaseMembershipClaim(
		gomock.Any(), "__user_presence__", "user-1", gomock.Any(),
	).Return(nil)
	s.mockCache.EXPECT().ListQueuedProducts(s.ctx, "user-1").Return([]string{"prod-1", "prod-2"}, nil)
	s.mockCache.EXPECT().SetUserPresenceDeadline(s.ctx, "user-1", gomock.Any()).Return(nil)
	s.mockCache.EXPECT().GetMembership(s.ctx, "prod-1", "user-1").Return(queued, nil)
	s.mockCache.EXPECT().RefreshExpiryTimer(
		s.ctx, "prod-1", "user-1",
		gomock.Cond(func(deadline time.Time) bool {
			return deadline.After(time.Now().UTC().Add(29 * time.Second))
		}),
	).Return(true, nil)
	s.mockCache.EXPECT().GetMembership(s.ctx, "prod-2", "user-1").Return(right, nil)
	s.mockCache.EXPECT().RemoveFromQueue(s.ctx, "prod-2", "user-1").Return(nil)

	s.Require().NoError(s.srv.RefreshUserPresence(s.ctx, "user-1"))
}

func (s *QueueServiceTestSuite) TestRefreshUserPresence_LeavesClaimedLeaseToWorker() {
	queued := &models.QueueMembership{ProductID: "prod-1", UserID: "user-1", Status: models.MembershipStatusQueued}
	s.mockCache.EXPECT().ClaimMembership(
		s.ctx, "__user_presence__", "user-1", gomock.Any(), gomock.Any(),
	).Return(true, nil)
	s.mockCache.EXPECT().ReleaseMembershipClaim(
		gomock.Any(), "__user_presence__", "user-1", gomock.Any(),
	).Return(nil)
	s.mockCache.EXPECT().ListQueuedProducts(s.ctx, "user-1").Return([]string{"prod-1"}, nil)
	s.mockCache.EXPECT().SetUserPresenceDeadline(s.ctx, "user-1", gomock.Any()).Return(nil)
	s.mockCache.EXPECT().GetMembership(s.ctx, "prod-1", "user-1").Return(queued, nil)
	s.mockCache.EXPECT().RefreshExpiryTimer(s.ctx, "prod-1", "user-1", gomock.Any()).Return(false, nil)

	s.Require().NoError(s.srv.RefreshUserPresence(s.ctx, "user-1"))
}
