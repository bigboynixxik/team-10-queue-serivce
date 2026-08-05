// Package service_test provides behavioral tests for the queue service business logic.
package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"backend/internal/models"
	"backend/internal/service"
	"backend/internal/service/mocks"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"
)

// QueueServiceTestSuite encapsulates the test suite for QueueService.
type QueueServiceTestSuite struct {
	suite.Suite
	ctrl        *gomock.Controller
	mockDurable *mocks.MockDurableRepo
	mockCache   *mocks.MockCacheRepo
	mockAvito   *mocks.MockAvitoClient
	srv         *service.QueueService
	ctx         context.Context
}

// SetupTest initializes the mocks and the service under test before each test case.
func (s *QueueServiceTestSuite) SetupTest() {
	s.ctrl = gomock.NewController(s.T())
	s.mockDurable = mocks.NewMockDurableRepo(s.ctrl)
	s.mockCache = mocks.NewMockCacheRepo(s.ctrl)
	s.mockAvito = mocks.NewMockAvitoClient(s.ctrl)
	s.ctx = context.Background()

	s.srv = service.NewQueueService(
		s.mockDurable,
		s.mockCache,
		s.mockAvito,
		2*time.Minute,
		4*time.Minute,
	)
}

// TearDownTest cleans up the mock controller after each test case.
func (s *QueueServiceTestSuite) TearDownTest() {
	s.ctrl.Finish()
}

// TestJoinQueue_Idempotency verifies that an existing active membership is returned without modifications.
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

// TestJoinQueue_MembershipFetchError verifies that domain errors other than ErrTokenNotFound abort the process.
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

// TestJoinQueue_AvitoError verifies that the failure to fetch initial stock aborts the allocation.
func (s *QueueServiceTestSuite) TestJoinQueue_AvitoError() {
	s.mockCache.EXPECT().
		GetMembership(s.ctx, "prod-1", "user-1").
		Return(nil, models.ErrTokenNotFound)

	expectedErr := errors.New("avito client error")
	s.mockAvito.EXPECT().
		GetInitialStock(s.ctx, "prod-1").
		Return(0, expectedErr)

	mem, right, err := s.srv.JoinQueue(s.ctx, "prod-1", "user-1", 1)

	require.ErrorIs(s.T(), err, expectedErr)
	assert.Nil(s.T(), mem)
	assert.Nil(s.T(), right)
}

// TestJoinQueue_FullAllocation verifies the behavior when requested quantity is fully available.
func (s *QueueServiceTestSuite) TestJoinQueue_FullAllocation() {
	s.mockCache.EXPECT().GetMembership(s.ctx, "prod-1", "user-1").Return(nil, models.ErrTokenNotFound)
	s.mockAvito.EXPECT().GetInitialStock(s.ctx, "prod-1").Return(10, nil)
	s.mockCache.EXPECT().InitStock(s.ctx, "prod-1", 10).Return(nil)
	s.mockCache.EXPECT().TryAllocate(s.ctx, "prod-1", 2).Return(2, 0, false, nil)

	s.mockDurable.EXPECT().SaveRight(s.ctx, gomock.Any()).Return(nil)
	s.mockDurable.EXPECT().UpsertMembership(s.ctx, gomock.Any()).Return(nil)

	s.mockCache.EXPECT().SetRight(s.ctx, gomock.Any()).Return(nil).AnyTimes()
	s.mockCache.EXPECT().SetMembership(s.ctx, gomock.Any()).Return(nil).AnyTimes()
	s.mockCache.EXPECT().AddToExpiryTimer(s.ctx, "prod-1", "user-1", gomock.Any()).Return(nil).AnyTimes()
	s.mockCache.EXPECT().PublishEvent(s.ctx, "prod-1", "user-1", gomock.Any()).Return(nil).AnyTimes()

	mem, right, err := s.srv.JoinQueue(s.ctx, "prod-1", "user-1", 2)

	require.NoError(s.T(), err)
	require.NotNil(s.T(), right)
	assert.Equal(s.T(), models.MembershipStatusRightActive, mem.Status)
	assert.Equal(s.T(), 2, right.Quantity)
	assert.Equal(s.T(), models.RightStatusActive, right.Status)
}

// TestJoinQueue_FullAllocation_Rollback verifies that reserved stock is restored if saving the right fails.
func (s *QueueServiceTestSuite) TestJoinQueue_FullAllocation_Rollback() {
	s.mockCache.EXPECT().GetMembership(s.ctx, "prod-1", "user-1").Return(nil, models.ErrTokenNotFound)
	s.mockAvito.EXPECT().GetInitialStock(s.ctx, "prod-1").Return(5, nil)
	s.mockCache.EXPECT().InitStock(s.ctx, "prod-1", 5).Return(nil)
	s.mockCache.EXPECT().TryAllocate(s.ctx, "prod-1", 1).Return(1, 0, false, nil)

	dbErr := errors.New("db connection lost")
	s.mockDurable.EXPECT().SaveRight(s.ctx, gomock.Any()).Return(dbErr)
	s.mockCache.EXPECT().RestoreAvailableUnits(gomock.Any(), "prod-1", 1).Return(nil)

	mem, right, err := s.srv.JoinQueue(s.ctx, "prod-1", "user-1", 1)

	require.ErrorIs(s.T(), err, dbErr)
	assert.Nil(s.T(), mem)
	assert.Nil(s.T(), right)
}

// TestJoinQueue_PartialAllocation verifies the behavior when only partial stock is available.
func (s *QueueServiceTestSuite) TestJoinQueue_PartialAllocation() {
	s.mockCache.EXPECT().GetMembership(s.ctx, "prod-1", "user-1").Return(nil, models.ErrTokenNotFound)
	s.mockAvito.EXPECT().GetInitialStock(s.ctx, "prod-1").Return(2, nil)
	s.mockCache.EXPECT().InitStock(s.ctx, "prod-1", 2).Return(nil)
	s.mockCache.EXPECT().TryAllocate(s.ctx, "prod-1", 5).Return(0, 2, false, nil)

	s.mockDurable.EXPECT().UpsertMembership(s.ctx, gomock.Any()).Return(nil)

	s.mockCache.EXPECT().SetMembership(s.ctx, gomock.Any()).Return(nil).AnyTimes()
	s.mockCache.EXPECT().AddToExpiryTimer(s.ctx, "prod-1", "user-1", gomock.Any()).Return(nil).AnyTimes()
	s.mockCache.EXPECT().PublishEvent(s.ctx, "prod-1", "user-1", gomock.Any()).Return(nil).AnyTimes()

	mem, right, err := s.srv.JoinQueue(s.ctx, "prod-1", "user-1", 5)

	require.NoError(s.T(), err)
	assert.Nil(s.T(), right)
	assert.Equal(s.T(), models.MembershipStatusOfferPending, mem.Status)
	assert.Equal(s.T(), 2, *mem.AvailableQuantity)
}

// TestJoinQueue_PartialAllocation_Rollback verifies that partial stock is restored if state update fails.
func (s *QueueServiceTestSuite) TestJoinQueue_PartialAllocation_Rollback() {
	s.mockCache.EXPECT().GetMembership(s.ctx, "prod-1", "user-1").Return(nil, models.ErrTokenNotFound)
	s.mockAvito.EXPECT().GetInitialStock(s.ctx, "prod-1").Return(2, nil)
	s.mockCache.EXPECT().InitStock(s.ctx, "prod-1", 2).Return(nil)
	s.mockCache.EXPECT().TryAllocate(s.ctx, "prod-1", 5).Return(0, 2, false, nil)

	dbErr := errors.New("db error")
	s.mockDurable.EXPECT().UpsertMembership(s.ctx, gomock.Any()).Return(dbErr)
	s.mockCache.EXPECT().RestoreAvailableUnits(gomock.Any(), "prod-1", 2).Return(nil)

	mem, right, err := s.srv.JoinQueue(s.ctx, "prod-1", "user-1", 5)

	require.ErrorIs(s.T(), err, dbErr)
	assert.Nil(s.T(), mem)
	assert.Nil(s.T(), right)
}

// TestJoinQueue_SoldOut verifies that the user is marked as sold out when no stock or queue slots exist.
func (s *QueueServiceTestSuite) TestJoinQueue_SoldOut() {
	s.mockCache.EXPECT().GetMembership(s.ctx, "prod-1", "user-1").Return(nil, models.ErrTokenNotFound)
	s.mockAvito.EXPECT().GetInitialStock(s.ctx, "prod-1").Return(0, nil)
	s.mockCache.EXPECT().InitStock(s.ctx, "prod-1", 0).Return(nil)
	s.mockCache.EXPECT().TryAllocate(s.ctx, "prod-1", 1).Return(0, 0, true, nil)

	s.mockDurable.EXPECT().UpsertMembership(s.ctx, gomock.Any()).Return(nil)

	s.mockCache.EXPECT().SetMembership(s.ctx, gomock.Any()).Return(nil).AnyTimes()
	s.mockCache.EXPECT().PublishEvent(s.ctx, "prod-1", "user-1", gomock.Any()).Return(nil).AnyTimes()

	mem, right, err := s.srv.JoinQueue(s.ctx, "prod-1", "user-1", 1)

	require.NoError(s.T(), err)
	assert.Nil(s.T(), right)
	assert.Equal(s.T(), models.MembershipStatusSoldOut, mem.Status)
}

// TestJoinQueue_Queued verifies that the user is correctly enqueued when stock is blocked by active offers.
func (s *QueueServiceTestSuite) TestJoinQueue_Queued() {
	s.mockCache.EXPECT().GetMembership(s.ctx, "prod-1", "user-1").Return(nil, models.ErrTokenNotFound)
	s.mockAvito.EXPECT().GetInitialStock(s.ctx, "prod-1").Return(10, nil)
	s.mockCache.EXPECT().InitStock(s.ctx, "prod-1", 10).Return(nil)
	s.mockCache.EXPECT().TryAllocate(s.ctx, "prod-1", 1).Return(0, 0, false, nil)

	s.mockDurable.EXPECT().UpsertMembership(s.ctx, gomock.Any()).Return(nil)
	s.mockCache.EXPECT().Enqueue(s.ctx, "prod-1", "user-1").Return(nil)

	s.mockCache.EXPECT().SetMembership(s.ctx, gomock.Any()).Return(nil).AnyTimes()
	s.mockCache.EXPECT().PublishEvent(s.ctx, "prod-1", "user-1", gomock.Any()).Return(nil).AnyTimes()

	mem, right, err := s.srv.JoinQueue(s.ctx, "prod-1", "user-1", 1)

	require.NoError(s.T(), err)
	assert.Nil(s.T(), right)
	assert.Equal(s.T(), models.MembershipStatusQueued, mem.Status)
}

// TestQueueServiceSuite acts as the entry point for running the test suite.
func TestQueueServiceSuite(t *testing.T) {
	suite.Run(t, new(QueueServiceTestSuite))
}

// TestJoinQueue_TryAllocateError verifies that allocation cache errors abort the process.
func (s *QueueServiceTestSuite) TestJoinQueue_TryAllocateError() {
	s.mockCache.EXPECT().GetMembership(s.ctx, "prod-1", "user-1").Return(nil, models.ErrTokenNotFound)
	s.mockAvito.EXPECT().GetInitialStock(s.ctx, "prod-1").Return(10, nil)
	s.mockCache.EXPECT().InitStock(s.ctx, "prod-1", 10).Return(nil)

	expectedErr := errors.New("redis script error")
	s.mockCache.EXPECT().TryAllocate(s.ctx, "prod-1", 1).Return(0, 0, false, expectedErr)

	mem, right, err := s.srv.JoinQueue(s.ctx, "prod-1", "user-1", 1)

	require.ErrorIs(s.T(), err, expectedErr)
	assert.Nil(s.T(), mem)
	assert.Nil(s.T(), right)
}

// TestJoinQueue_FinalStateUpsertError verifies that failing to save the Queued/SoldOut state returns an error.
func (s *QueueServiceTestSuite) TestJoinQueue_FinalStateUpsertError() {
	s.mockCache.EXPECT().GetMembership(s.ctx, "prod-1", "user-1").Return(nil, models.ErrTokenNotFound)
	s.mockAvito.EXPECT().GetInitialStock(s.ctx, "prod-1").Return(0, nil)
	s.mockCache.EXPECT().InitStock(s.ctx, "prod-1", 0).Return(nil)
	s.mockCache.EXPECT().TryAllocate(s.ctx, "prod-1", 1).Return(0, 0, true, nil)

	dbErr := errors.New("db timeout on final upsert")
	s.mockDurable.EXPECT().UpsertMembership(s.ctx, gomock.Any()).Return(dbErr)

	mem, right, err := s.srv.JoinQueue(s.ctx, "prod-1", "user-1", 1)

	require.ErrorIs(s.T(), err, dbErr)
	assert.Nil(s.T(), mem)
	assert.Nil(s.T(), right)
}
