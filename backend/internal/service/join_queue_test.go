package service_test

import (
	"errors"

	"backend/internal/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func (s *QueueServiceTestSuite) TestJoinQueue_Idempotency() {
	existingMem := &models.QueueMembership{
		ProductID: "prod-1",
		UserID:    "user-1",
		Status:    models.MembershipStatusQueued,
	}

	s.mockCache.EXPECT().
		GetMembership(s.ctx, "prod-1", "user-1").
		Return(existingMem, nil)

	mem, right, err := s.srv.JoinQueue(s.ctx, "prod-1", "user-1", 1)

	require.NoError(s.T(), err)
	assert.Nil(s.T(), right)
	assert.Equal(s.T(), existingMem, mem)
}

// TestJoinQueue_Idempotency_RightActive verifies that if a user is already in RIGHT_ACTIVE state,
// the method fetches their existing token from the cache and returns it without side effects.
func (s *QueueServiceTestSuite) TestJoinQueue_Idempotency_RightActive() {
	token := "existing-token-123"
	existingMem := &models.QueueMembership{
		ProductID:    "prod-1",
		UserID:       "user-1",
		Status:       models.MembershipStatusRightActive,
		CurrentToken: &token,
	}
	existingRight := &models.Right{
		Token: token,
	}

	s.mockCache.EXPECT().GetMembership(s.ctx, "prod-1", "user-1").Return(existingMem, nil)
	s.mockCache.EXPECT().GetRight(s.ctx, token).Return(existingRight, nil)

	mem, right, err := s.srv.JoinQueue(s.ctx, "prod-1", "user-1", 1)

	require.NoError(s.T(), err)
	assert.Equal(s.T(), existingMem, mem)
	assert.Equal(s.T(), existingRight, right)
}

func (s *QueueServiceTestSuite) TestJoinQueue_Idempotency_RightCacheErrorIsReturned() {
	token := "existing-token-123"
	existingMem := &models.QueueMembership{
		ProductID: "prod-1", UserID: "user-1",
		Status: models.MembershipStatusRightActive, CurrentToken: &token,
	}
	cacheErr := errors.New("right cache unavailable")
	s.mockCache.EXPECT().GetMembership(s.ctx, "prod-1", "user-1").Return(existingMem, nil)
	s.mockCache.EXPECT().GetRight(s.ctx, token).Return(nil, cacheErr)

	mem, right, err := s.srv.JoinQueue(s.ctx, "prod-1", "user-1", 1)

	require.ErrorIs(s.T(), err, cacheErr)
	assert.Nil(s.T(), mem)
	assert.Nil(s.T(), right)
}

func (s *QueueServiceTestSuite) TestJoinQueue_MembershipFetchError() {
	unexpectedErr := errors.New("redis timeout")
	s.mockCache.EXPECT().
		GetMembership(s.ctx, "prod-1", "user-1").
		Return(nil, unexpectedErr)

	mem, right, err := s.srv.JoinQueue(s.ctx, "prod-1", "user-1", 1)

	require.ErrorIs(s.T(), err, unexpectedErr)
	assert.Nil(s.T(), mem)
	assert.Nil(s.T(), right)
}

func (s *QueueServiceTestSuite) TestJoinQueue_AvitoError() {
	s.mockCache.EXPECT().GetMembership(s.ctx, "prod-1", "user-1").
		Return(nil, models.ErrTokenNotFound).Times(2)
	s.mockCache.EXPECT().ClaimMembership(
		gomock.Any(), "prod-1", "user-1", gomock.Any(), gomock.Any(),
	).Return(true, nil)
	s.mockCache.EXPECT().ReleaseMembershipClaim(
		gomock.Any(), "prod-1", "user-1", gomock.Any(),
	).Return(nil)
	expectedErr := errors.New("avito client error")
	s.mockAvito.EXPECT().GetInitialStock(s.ctx, "prod-1").Return(0, expectedErr)

	mem, right, err := s.srv.JoinQueue(s.ctx, "prod-1", "user-1", 1)

	require.ErrorIs(s.T(), err, expectedErr)
	assert.Nil(s.T(), mem)
	assert.Nil(s.T(), right)
}

func (s *QueueServiceTestSuite) TestJoinQueue_FullAllocation() {
	s.mockJoinQueueBase(10, 2, 2, 0, false, nil)

	s.mockDurableIssue(2, nil)
	s.mockSyncCacheState(models.MembershipStatusRightActive, true, true)

	mem, right, err := s.srv.JoinQueue(s.ctx, "prod-1", "user-1", 2)

	require.NoError(s.T(), err)
	require.NotNil(s.T(), right)
	assert.Equal(s.T(), models.MembershipStatusRightActive, mem.Status)
}

func (s *QueueServiceTestSuite) TestJoinQueue_FullAllocation_Rollback() {
	s.mockJoinQueueBase(5, 1, 1, 0, false, nil)

	dbErr := errors.New("db connection lost")
	s.mockDurableIssue(1, dbErr)
	s.mockCache.EXPECT().RestoreAvailableUnits(gomock.Any(), "prod-1", 1).Return(nil)

	mem, right, err := s.srv.JoinQueue(s.ctx, "prod-1", "user-1", 1)

	require.ErrorIs(s.T(), err, dbErr)
	assert.Nil(s.T(), mem)
	assert.Nil(s.T(), right)
}

func (s *QueueServiceTestSuite) TestJoinQueue_FullAllocation_ReportsRollbackFailure() {
	s.mockJoinQueueBase(5, 1, 1, 0, false, nil)
	dbErr := errors.New("db connection lost")
	rollbackErr := errors.New("redis rollback failed")
	s.mockDurableIssue(1, dbErr)
	s.mockCache.EXPECT().RestoreAvailableUnits(gomock.Any(), "prod-1", 1).Return(rollbackErr)

	mem, right, err := s.srv.JoinQueue(s.ctx, "prod-1", "user-1", 1)

	require.ErrorIs(s.T(), err, dbErr)
	require.ErrorIs(s.T(), err, rollbackErr)
	assert.Nil(s.T(), mem)
	assert.Nil(s.T(), right)
}

func (s *QueueServiceTestSuite) TestJoinQueue_PartialAllocation() {
	s.mockJoinQueueBase(2, 5, 0, 2, false, nil)

	s.mockDurableUpsert(models.MembershipStatusOfferPending, ptr(2))
	s.mockSyncCacheState(models.MembershipStatusOfferPending, false, true)

	mem, right, err := s.srv.JoinQueue(s.ctx, "prod-1", "user-1", 5)

	require.NoError(s.T(), err)
	assert.Nil(s.T(), right)
	assert.Equal(s.T(), models.MembershipStatusOfferPending, mem.Status)
}

func (s *QueueServiceTestSuite) TestJoinQueue_PartialAllocation_Rollback() {
	s.mockJoinQueueBase(2, 5, 0, 2, false, nil)

	dbErr := errors.New("db error")
	s.mockDurable.EXPECT().UpsertMembership(s.ctx, gomock.Any()).Return(dbErr)
	s.mockCache.EXPECT().RestoreAvailableUnits(gomock.Any(), "prod-1", 2).Return(nil)

	mem, right, err := s.srv.JoinQueue(s.ctx, "prod-1", "user-1", 5)

	require.ErrorIs(s.T(), err, dbErr)
	assert.Nil(s.T(), mem)
	assert.Nil(s.T(), right)
}

func (s *QueueServiceTestSuite) TestJoinQueue_SoldOut() {
	s.mockJoinQueueBase(0, 1, 0, 0, true, nil)

	s.mockDurableUpsert(models.MembershipStatusSoldOut, nil)
	s.mockSyncCacheState(models.MembershipStatusSoldOut, false, false)

	mem, right, err := s.srv.JoinQueue(s.ctx, "prod-1", "user-1", 1)

	require.NoError(s.T(), err)
	assert.Nil(s.T(), right)
	assert.Equal(s.T(), models.MembershipStatusSoldOut, mem.Status)
}

func (s *QueueServiceTestSuite) TestJoinQueue_Queued() {
	s.mockJoinQueueBase(10, 1, 0, 0, false, nil)

	s.mockDurableUpsert(models.MembershipStatusQueued, nil)
	s.mockCache.EXPECT().Enqueue(s.ctx, "prod-1", "user-1").Return(nil)
	s.mockSyncCacheState(models.MembershipStatusQueued, false, false)

	mem, right, err := s.srv.JoinQueue(s.ctx, "prod-1", "user-1", 1)

	require.NoError(s.T(), err)
	assert.Nil(s.T(), right)
	assert.Equal(s.T(), models.MembershipStatusQueued, mem.Status)
}

func (s *QueueServiceTestSuite) TestJoinQueue_TryAllocateError() {
	expectedErr := errors.New("redis script error")
	s.mockJoinQueueBase(10, 1, 0, 0, false, expectedErr)

	mem, right, err := s.srv.JoinQueue(s.ctx, "prod-1", "user-1", 1)

	require.ErrorIs(s.T(), err, expectedErr)
	assert.Nil(s.T(), mem)
	assert.Nil(s.T(), right)
}

func (s *QueueServiceTestSuite) TestJoinQueue_InitStockErrorIsReturned() {
	s.mockCache.EXPECT().GetMembership(s.ctx, "prod-1", "user-1").
		Return(nil, models.ErrTokenNotFound).Times(2)
	s.expectMembershipClaim("prod-1", "user-1")
	s.mockAvito.EXPECT().GetInitialStock(s.ctx, "prod-1").Return(10, nil)
	cacheErr := errors.New("redis unavailable")
	s.mockCache.EXPECT().InitStock(s.ctx, "prod-1", 10).Return(cacheErr)

	mem, right, err := s.srv.JoinQueue(s.ctx, "prod-1", "user-1", 1)

	require.ErrorIs(s.T(), err, cacheErr)
	assert.Nil(s.T(), mem)
	assert.Nil(s.T(), right)
}

func (s *QueueServiceTestSuite) TestJoinQueue_SaveInitialStockErrorIsReturned() {
	s.mockCache.EXPECT().GetMembership(s.ctx, "prod-1", "user-1").
		Return(nil, models.ErrTokenNotFound).Times(2)
	s.expectMembershipClaim("prod-1", "user-1")
	s.mockAvito.EXPECT().GetInitialStock(s.ctx, "prod-1").Return(10, nil)
	s.mockCache.EXPECT().InitStock(s.ctx, "prod-1", 10).Return(nil)
	dbErr := errors.New("postgres unavailable")
	s.mockDurable.EXPECT().SaveInitialStock(s.ctx, gomock.Any()).Return(dbErr)

	mem, right, err := s.srv.JoinQueue(s.ctx, "prod-1", "user-1", 1)

	require.ErrorIs(s.T(), err, dbErr)
	assert.Nil(s.T(), mem)
	assert.Nil(s.T(), right)
}

func (s *QueueServiceTestSuite) TestJoinQueue_EnqueueErrorIsReturned() {
	s.mockJoinQueueBase(10, 1, 0, 0, false, nil)
	s.mockDurableUpsert(models.MembershipStatusQueued, nil)
	queueErr := errors.New("redis queue unavailable")
	s.mockCache.EXPECT().Enqueue(s.ctx, "prod-1", "user-1").Return(queueErr)

	mem, right, err := s.srv.JoinQueue(s.ctx, "prod-1", "user-1", 1)

	require.ErrorIs(s.T(), err, queueErr)
	assert.Nil(s.T(), mem)
	assert.Nil(s.T(), right)
}

func (s *QueueServiceTestSuite) TestJoinQueue_ExpiryTimerErrorIsReturned() {
	s.mockJoinQueueBase(10, 1, 1, 0, false, nil)
	s.mockDurableIssue(1, nil)
	s.mockCache.EXPECT().SetRight(s.ctx, gomock.Any()).Return(nil)
	timerErr := errors.New("redis timer unavailable")
	s.mockCache.EXPECT().AddToExpiryTimer(s.ctx, "prod-1", "user-1", gomock.Any()).Return(timerErr)

	mem, right, err := s.srv.JoinQueue(s.ctx, "prod-1", "user-1", 1)

	require.ErrorIs(s.T(), err, timerErr)
	assert.Nil(s.T(), mem)
	assert.Nil(s.T(), right)
}

func (s *QueueServiceTestSuite) TestJoinQueue_FinalStateUpsertError() {
	s.mockJoinQueueBase(0, 1, 0, 0, true, nil)

	dbErr := errors.New("db timeout on final upsert")
	s.mockDurable.EXPECT().UpsertMembership(s.ctx, gomock.Any()).Return(dbErr)

	mem, right, err := s.srv.JoinQueue(s.ctx, "prod-1", "user-1", 1)

	require.ErrorIs(s.T(), err, dbErr)
	assert.Nil(s.T(), mem)
	assert.Nil(s.T(), right)
}

// TestJoinQueue_ConcurrentClaimLost verifies that a request losing the join claim
// waits for the winner and returns the membership the winner created, instead of
// allocating a second one. This is what stops N parallel requests from a single
// user from walking away with N rights.
func (s *QueueServiceTestSuite) TestJoinQueue_ConcurrentClaimLost() {
	created := &models.QueueMembership{
		ProductID: "prod-1",
		UserID:    "user-1",
		Status:    models.MembershipStatusQueued,
	}

	gomock.InOrder(
		// First look: the winner has not published anything yet.
		s.mockCache.EXPECT().GetMembership(s.ctx, "prod-1", "user-1").
			Return(nil, models.ErrTokenNotFound),
		// The claim is already held by the concurrent request.
		s.mockCache.EXPECT().ClaimMembership(
			gomock.Any(), "prod-1", "user-1", gomock.Any(), gomock.Any(),
		).
			Return(false, nil),
		// While waiting, the winner finishes and the membership appears.
		s.mockCache.EXPECT().GetMembership(s.ctx, "prod-1", "user-1").
			Return(created, nil),
	)

	mem, right, err := s.srv.JoinQueue(s.ctx, "prod-1", "user-1", 1)

	require.NoError(s.T(), err)
	assert.Nil(s.T(), right)
	assert.Equal(s.T(), created, mem)
}

// TestJoinQueue_ConcurrentClaimNeverResolves verifies that a losing request gives
// up with a retryable conflict rather than hanging or creating a duplicate when
// the winner leaves no membership behind.
func (s *QueueServiceTestSuite) TestJoinQueue_ConcurrentClaimNeverResolves() {
	s.mockCache.EXPECT().GetMembership(s.ctx, "prod-1", "user-1").
		Return(nil, models.ErrTokenNotFound).AnyTimes()
	s.mockCache.EXPECT().ClaimMembership(
		gomock.Any(), "prod-1", "user-1", gomock.Any(), gomock.Any(),
	).
		Return(false, nil)

	mem, right, err := s.srv.JoinQueue(s.ctx, "prod-1", "user-1", 1)

	require.ErrorIs(s.T(), err, models.ErrConcurrentJoin)
	assert.Nil(s.T(), mem)
	assert.Nil(s.T(), right)
}
