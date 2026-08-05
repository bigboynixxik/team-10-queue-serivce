package service_test

import (
	"errors"

	"backend/internal/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func (s *QueueServiceTestSuite) TestAcceptOffer_Success_Full() {
	s.mockMembershipFetch(models.MembershipStatusOfferPending, ptr(2))

	s.mockDurable.EXPECT().SaveRight(s.ctx, gomock.Cond(func(x any) bool {
		r, ok := x.(*models.Right)
		return ok && r.Status == models.RightStatusActive && r.Quantity == 2
	})).Return(nil)

	s.mockDurableUpsert(models.MembershipStatusRightActive, nil)
	s.mockSyncCacheState(models.MembershipStatusRightActive, true, true)

	right, err := s.srv.AcceptOffer(s.ctx, "prod-1", "user-1", 2)

	require.NoError(s.T(), err)
	require.NotNil(s.T(), right)
	assert.Equal(s.T(), 2, right.Quantity)
	assert.Equal(s.T(), models.RightStatusActive, right.Status)
}

func (s *QueueServiceTestSuite) TestAcceptOffer_Success_Partial() {
	s.mockMembershipFetch(models.MembershipStatusOfferPending, ptr(5))

	s.mockDurable.EXPECT().SaveRight(s.ctx, gomock.Cond(func(x any) bool {
		r, ok := x.(*models.Right)
		return ok && r.Status == models.RightStatusActive && r.Quantity == 2
	})).Return(nil)

	s.mockDurableUpsert(models.MembershipStatusRightActive, nil)
	s.mockSyncCacheState(models.MembershipStatusRightActive, true, true)

	s.mockCache.EXPECT().RestoreAvailableUnits(s.ctx, "prod-1", 3).Return(nil)

	right, err := s.srv.AcceptOffer(s.ctx, "prod-1", "user-1", 2)

	require.NoError(s.T(), err)
	require.NotNil(s.T(), right)
	assert.Equal(s.T(), 2, right.Quantity)
}

func (s *QueueServiceTestSuite) TestAcceptOffer_MembershipFetchError() {
	expectedErr := errors.New("redis timeout")
	s.mockCache.EXPECT().GetMembership(s.ctx, "prod-1", "user-1").Return(nil, expectedErr)

	right, err := s.srv.AcceptOffer(s.ctx, "prod-1", "user-1", 2)

	require.ErrorIs(s.T(), err, expectedErr)
	assert.Nil(s.T(), right)
}

func (s *QueueServiceTestSuite) TestAcceptOffer_InvalidQuantity() {
	right, err := s.srv.AcceptOffer(s.ctx, "prod-1", "user-1", 0)

	require.ErrorIs(s.T(), err, models.ErrQuantityInvalid)
	assert.Nil(s.T(), right)
}

func (s *QueueServiceTestSuite) TestAcceptOffer_InvalidStatus() {
	s.mockMembershipFetch(models.MembershipStatusQueued, nil)

	right, err := s.srv.AcceptOffer(s.ctx, "prod-1", "user-1", 2)

	require.ErrorIs(s.T(), err, models.ErrInvalidStatus)
	assert.Nil(s.T(), right)
}

func (s *QueueServiceTestSuite) TestAcceptOffer_NilAvailableQuantity() {
	s.mockMembershipFetch(models.MembershipStatusOfferPending, nil)

	right, err := s.srv.AcceptOffer(s.ctx, "prod-1", "user-1", 2)

	require.ErrorIs(s.T(), err, models.ErrInvalidStatus)
	assert.Nil(s.T(), right)
}

func (s *QueueServiceTestSuite) TestAcceptOffer_ExceedsAvailable() {
	s.mockMembershipFetch(models.MembershipStatusOfferPending, ptr(2))

	right, err := s.srv.AcceptOffer(s.ctx, "prod-1", "user-1", 5)

	require.ErrorIs(s.T(), err, models.ErrQuantityInvalid)
	assert.Nil(s.T(), right)
}

func (s *QueueServiceTestSuite) TestAcceptOffer_SaveRightError() {
	s.mockMembershipFetch(models.MembershipStatusOfferPending, ptr(2))

	dbErr := errors.New("db save right error")
	s.mockDurable.EXPECT().SaveRight(s.ctx, gomock.Any()).Return(dbErr)

	right, err := s.srv.AcceptOffer(s.ctx, "prod-1", "user-1", 2)

	require.ErrorIs(s.T(), err, dbErr)
	assert.Nil(s.T(), right)
}
