package service_test

import (
	"errors"

	"backend/internal/models"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// popResult encapsulates the return values for a PopAndAllocate mock.
type popResult struct {
	UID     string
	Alloc   int
	Avail   int
	SoldOut bool
	Status  models.MembershipStatus
	Score   float64
	Err     error
}

// mockAdvanceQueueSequence creates an ordered sequence of PopAndAllocate calls,
// automatically appending an empty return at the end to safely break the infinite loop.
func (s *QueueServiceTestSuite) mockAdvanceQueueSequence(results ...popResult) {
	var calls []any
	for _, r := range results {
		calls = append(calls, s.mockCache.EXPECT().PopAndAllocate(s.ctx, "prod-1").
			Return(r.UID, r.Alloc, r.Avail, r.SoldOut, r.Status, r.Score, r.Err))
		if r.Status == models.MembershipStatusRightActive ||
			r.Status == models.MembershipStatusOfferPending ||
			r.Status == models.MembershipStatusSoldOut {
			s.expectMembershipClaim("prod-1", r.UID)
		}
	}

	calls = append(calls, s.mockCache.EXPECT().PopAndAllocate(s.ctx, "prod-1").
		Return("", 0, 0, false, models.MembershipStatus(""), 0.0, nil).AnyTimes())

	gomock.InOrder(calls...)
}

// mockRollbackHelper sets up expectations for the disaster recovery rollback process.
// It uses gomock.Any() for context because rollbackAdvance explicitly uses context.Background().
func (s *QueueServiceTestSuite) mockRollbackHelper(qty int, score float64) {
	if qty > 0 {
		s.mockCache.EXPECT().RestoreAvailableUnits(gomock.Any(), "prod-1", qty).Return(nil)
	}
	s.mockCache.EXPECT().Requeue(gomock.Any(), "prod-1", "user-1", score).Return(nil)
}

// TestAdvanceQueue_PopError verifies that a failure in the underlying Lua script
// immediately aborts the queue advancement process and returns the error.
func (s *QueueServiceTestSuite) TestAdvanceQueue_PopError() {
	expectedErr := errors.New("lua script execution failed")
	s.mockCache.EXPECT().PopAndAllocate(s.ctx, "prod-1").
		Return("", 0, 0, false, models.MembershipStatus(""), 0.0, expectedErr)

	err := s.srv.AdvanceQueue(s.ctx, "prod-1")

	require.ErrorIs(s.T(), err, expectedErr)
}

// TestAdvanceQueue_QueuedBreaks verifies that if the first user in the queue cannot be
// processed due to held stock (QUEUED status), the engine correctly stops.
func (s *QueueServiceTestSuite) TestAdvanceQueue_QueuedBreaks() {
	s.mockCache.EXPECT().PopAndAllocate(s.ctx, "prod-1").
		Return("user-1", 0, 0, false, models.MembershipStatusQueued, 1.0, nil)

	err := s.srv.AdvanceQueue(s.ctx, "prod-1")

	require.NoError(s.T(), err)
}

func (s *QueueServiceTestSuite) TestAdvanceQueue_ClaimLostRollsBackAllocation() {
	s.mockCache.EXPECT().PopAndAllocate(s.ctx, "prod-1").Return(
		"user-1", 2, 0, false, models.MembershipStatusRightActive, 7.0, nil,
	)
	s.mockCache.EXPECT().ClaimMembership(
		gomock.Any(), "prod-1", "user-1", gomock.Any(), gomock.Any(),
	).Return(false, nil)
	s.mockCache.EXPECT().RestoreAvailableUnits(gomock.Any(), "prod-1", 2).Return(nil)
	s.mockCache.EXPECT().Requeue(gomock.Any(), "prod-1", "user-1", 7.0).Return(nil)

	err := s.srv.AdvanceQueue(s.ctx, "prod-1")

	require.ErrorIs(s.T(), err, models.ErrConcurrentJoin)
}

func (s *QueueServiceTestSuite) TestAdvanceQueue_ClaimLostReportsRollbackFailure() {
	rollbackErr := errors.New("redis stock rollback failed")
	s.mockCache.EXPECT().PopAndAllocate(s.ctx, "prod-1").Return(
		"user-1", 2, 0, false, models.MembershipStatusRightActive, 7.0, nil,
	)
	s.mockCache.EXPECT().ClaimMembership(
		gomock.Any(), "prod-1", "user-1", gomock.Any(), gomock.Any(),
	).Return(false, nil)
	s.mockCache.EXPECT().RestoreAvailableUnits(gomock.Any(), "prod-1", 2).Return(rollbackErr)
	s.mockCache.EXPECT().Requeue(gomock.Any(), "prod-1", "user-1", 7.0).Return(nil)

	err := s.srv.AdvanceQueue(s.ctx, "prod-1")

	require.ErrorIs(s.T(), err, models.ErrConcurrentJoin)
	require.ErrorIs(s.T(), err, rollbackErr)
}

// TestAdvanceQueue_GhostUser verifies that unprocessable or stale statuses
// are ignored and the engine continues to the next valid user in the queue.
func (s *QueueServiceTestSuite) TestAdvanceQueue_GhostUser() {
	s.mockAdvanceQueueSequence(popResult{
		UID: "user-1", Status: "GHOST", Score: 1.0,
	})

	err := s.srv.AdvanceQueue(s.ctx, "prod-1")

	require.NoError(s.T(), err)
}

// TestAdvanceQueue_MembershipFetchError verifies that a cache error while fetching user
// details triggers a safe rollback. It expects the rollback quantity to be the sum of
// alloc and avail (e.g., 2 + 1 = 3) and restores the user's exact queue position.
func (s *QueueServiceTestSuite) TestAdvanceQueue_MembershipFetchError() {
	s.mockAdvanceQueueSequence(popResult{
		UID: "user-1", Alloc: 2, Avail: 1, Status: models.MembershipStatusRightActive, Score: 5.0,
	})
	s.mockCache.EXPECT().GetMembership(s.ctx, "prod-1", "user-1").Return(nil, errors.New("not found"))

	s.mockRollbackHelper(3, 5.0)

	err := s.srv.AdvanceQueue(s.ctx, "prod-1")

	require.ErrorContains(s.T(), err, "get membership")
}

// TestAdvanceQueue_RightActive_Success verifies the happy path where a queued user
// is granted full allocation and receives a valid purchase right.
func (s *QueueServiceTestSuite) TestAdvanceQueue_RightActive_Success() {
	s.mockAdvanceQueueSequence(popResult{
		UID: "user-1", Alloc: 2, Status: models.MembershipStatusRightActive, Score: 1.0,
	})
	s.mockMembershipFetch(models.MembershipStatusQueued, nil)

	s.mockDurableIssue(2, nil)
	s.mockSyncCacheState(models.MembershipStatusRightActive, true, true)

	err := s.srv.AdvanceQueue(s.ctx, "prod-1")

	require.NoError(s.T(), err)
}

// TestAdvanceQueue_RightActive_SaveRightError verifies that a database crash during
// right generation triggers a rollback of the allocated units without stalling the engine.
func (s *QueueServiceTestSuite) TestAdvanceQueue_RightActive_SaveRightError() {
	s.mockAdvanceQueueSequence(popResult{
		UID: "user-1", Alloc: 2, Status: models.MembershipStatusRightActive, Score: 1.0,
	})
	s.mockMembershipFetch(models.MembershipStatusQueued, nil)

	dbErr := errors.New("db crash")
	s.mockDurableIssue(2, dbErr)
	s.mockRollbackHelper(2, 1.0)

	err := s.srv.AdvanceQueue(s.ctx, "prod-1")

	require.ErrorIs(s.T(), err, dbErr)
}

// TestAdvanceQueue_OfferPending_Success verifies the happy path where a queued user
// receives a partial offer when stock is insufficient for their full request.
func (s *QueueServiceTestSuite) TestAdvanceQueue_OfferPending_Success() {
	s.mockAdvanceQueueSequence(popResult{
		UID: "user-1", Avail: 3, Status: models.MembershipStatusOfferPending, Score: 1.0,
	})
	s.mockMembershipFetch(models.MembershipStatusQueued, nil)

	s.mockDurableUpsert(models.MembershipStatusOfferPending, ptr(3))
	s.mockSyncCacheState(models.MembershipStatusOfferPending, false, true)

	err := s.srv.AdvanceQueue(s.ctx, "prod-1")

	require.NoError(s.T(), err)
}

// TestAdvanceQueue_OfferPending_UpsertError verifies that a database failure while
// saving a pending offer triggers an immediate rollback of the held stock.
func (s *QueueServiceTestSuite) TestAdvanceQueue_OfferPending_UpsertError() {
	s.mockAdvanceQueueSequence(popResult{
		UID: "user-1", Avail: 3, Status: models.MembershipStatusOfferPending, Score: 1.0,
	})
	s.mockMembershipFetch(models.MembershipStatusQueued, nil)

	s.mockDurable.EXPECT().UpsertMembership(s.ctx, gomock.Any()).Return(errors.New("db error"))
	s.mockRollbackHelper(3, 1.0)

	err := s.srv.AdvanceQueue(s.ctx, "prod-1")

	require.ErrorContains(s.T(), err, "upsert partial membership")
}

// TestAdvanceQueue_SoldOut_Success verifies that the engine accurately flips a waiting user
// to a SOLD_OUT status when the physical inventory reaches absolute zero.
func (s *QueueServiceTestSuite) TestAdvanceQueue_SoldOut_Success() {
	s.mockAdvanceQueueSequence(popResult{
		UID: "user-1", Status: models.MembershipStatusSoldOut, Score: 1.0,
	})
	s.mockMembershipFetch(models.MembershipStatusQueued, nil)

	s.mockDurableUpsert(models.MembershipStatusSoldOut, nil)
	s.mockSyncCacheState(models.MembershipStatusSoldOut, false, false)

	err := s.srv.AdvanceQueue(s.ctx, "prod-1")

	require.NoError(s.T(), err)
}

// TestAdvanceQueue_SoldOut_UpsertError verifies that if the database fails to record a SOLD_OUT state,
// the user is requeued safely. It explicitly expects quantity 0 since no stock is restored.
func (s *QueueServiceTestSuite) TestAdvanceQueue_SoldOut_UpsertError() {
	s.mockAdvanceQueueSequence(popResult{
		UID: "user-1", Status: models.MembershipStatusSoldOut, Score: 1.0,
	})
	s.mockMembershipFetch(models.MembershipStatusQueued, nil)

	s.mockDurable.EXPECT().UpsertMembership(s.ctx, gomock.Any()).Return(errors.New("db timeout"))
	s.mockRollbackHelper(0, 1.0)

	err := s.srv.AdvanceQueue(s.ctx, "prod-1")

	require.ErrorContains(s.T(), err, "upsert sold_out membership")
}

// TestAdvanceQueue_MultipleSuccessfulIterations verifies that the engine successfully
// processes multiple valid users in a single run before gracefully stopping.
func (s *QueueServiceTestSuite) TestAdvanceQueue_MultipleSuccessfulIterations() {
	s.mockAdvanceQueueSequence(
		popResult{UID: "user-1", Alloc: 2, Status: models.MembershipStatusRightActive, Score: 1.0},
		popResult{UID: "user-2", Avail: 1, Status: models.MembershipStatusOfferPending, Score: 2.0},
	)

	s.mockCache.EXPECT().GetMembership(s.ctx, "prod-1", "user-1").Return(&models.QueueMembership{
		ProductID: "prod-1", UserID: "user-1", Status: models.MembershipStatusQueued,
	}, nil)

	s.mockDurable.EXPECT().IssueRightAndUpsertMembershipTx(
		s.ctx,
		gomock.Any(),
		gomock.Cond(func(x any) bool {
			m, ok := x.(*models.QueueMembership)
			return ok && m.UserID == "user-1" && m.Status == models.MembershipStatusRightActive
		}),
	).Return(nil)
	s.mockCache.EXPECT().SetRight(s.ctx, gomock.Any()).Return(nil)
	s.mockCache.EXPECT().SetMembership(s.ctx, gomock.Cond(func(x any) bool {
		return x.(*models.QueueMembership).UserID == "user-1"
	})).Return(nil)
	s.mockCache.EXPECT().AddToExpiryTimer(s.ctx, "prod-1", "user-1", gomock.Any()).Return(nil)
	s.mockCache.EXPECT().PublishEvent(s.ctx, "prod-1", "user-1", gomock.Any()).Return(nil)

	s.mockCache.EXPECT().GetMembership(s.ctx, "prod-1", "user-2").Return(&models.QueueMembership{
		ProductID: "prod-1", UserID: "user-2", Status: models.MembershipStatusQueued,
	}, nil)

	s.mockDurable.EXPECT().UpsertMembership(s.ctx, gomock.Cond(func(x any) bool {
		m, ok := x.(*models.QueueMembership)
		return ok && m.UserID == "user-2" && m.Status == models.MembershipStatusOfferPending
	})).Return(nil)
	s.mockCache.EXPECT().SetMembership(s.ctx, gomock.Cond(func(x any) bool {
		return x.(*models.QueueMembership).UserID == "user-2"
	})).Return(nil)
	s.mockCache.EXPECT().AddToExpiryTimer(s.ctx, "prod-1", "user-2", gomock.Any()).Return(nil)
	s.mockCache.EXPECT().PublishEvent(s.ctx, "prod-1", "user-2", gomock.Any()).Return(nil)

	err := s.srv.AdvanceQueue(s.ctx, "prod-1")

	require.NoError(s.T(), err)
}
