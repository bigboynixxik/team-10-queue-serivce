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
	s.mockCache.EXPECT().GetMembership(s.ctx, "prod-1", "user-1").Return(nil, models.ErrTokenNotFound)
	expectedErr := errors.New("avito client error")
	s.mockAvito.EXPECT().GetInitialStock(s.ctx, "prod-1").Return(0, expectedErr)

	mem, right, err := s.srv.JoinQueue(s.ctx, "prod-1", "user-1", 1)

	require.ErrorIs(s.T(), err, expectedErr)
	assert.Nil(s.T(), mem)
	assert.Nil(s.T(), right)
}

func (s *QueueServiceTestSuite) TestJoinQueue_FullAllocation() {
	s.mockJoinQueueBase(10, 2, 2, 0, false, nil)

	s.mockDurable.EXPECT().SaveRight(s.ctx, gomock.Cond(func(x any) bool {
		r, ok := x.(*models.Right)
		return ok && r.Status == models.RightStatusActive && r.Quantity == 2
	})).Return(nil)

	s.mockDurableUpsert(models.MembershipStatusRightActive, nil)
	s.mockSyncCacheState(models.MembershipStatusRightActive, true, true)

	mem, right, err := s.srv.JoinQueue(s.ctx, "prod-1", "user-1", 2)

	require.NoError(s.T(), err)
	require.NotNil(s.T(), right)
	assert.Equal(s.T(), models.MembershipStatusRightActive, mem.Status)
}

func (s *QueueServiceTestSuite) TestJoinQueue_FullAllocation_Rollback() {
	s.mockJoinQueueBase(5, 1, 1, 0, false, nil)

	dbErr := errors.New("db connection lost")
	s.mockDurable.EXPECT().SaveRight(s.ctx, gomock.Any()).Return(dbErr)
	s.mockCache.EXPECT().RestoreAvailableUnits(gomock.Any(), "prod-1", 1).Return(nil)

	mem, right, err := s.srv.JoinQueue(s.ctx, "prod-1", "user-1", 1)

	require.ErrorIs(s.T(), err, dbErr)
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

func (s *QueueServiceTestSuite) TestJoinQueue_FinalStateUpsertError() {
	s.mockJoinQueueBase(0, 1, 0, 0, true, nil)

	dbErr := errors.New("db timeout on final upsert")
	s.mockDurable.EXPECT().UpsertMembership(s.ctx, gomock.Any()).Return(dbErr)

	mem, right, err := s.srv.JoinQueue(s.ctx, "prod-1", "user-1", 1)

	require.ErrorIs(s.T(), err, dbErr)
	assert.Nil(s.T(), mem)
	assert.Nil(s.T(), right)
}
