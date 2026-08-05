package service_test

import (
	"errors"

	"backend/internal/models"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func (s *QueueServiceTestSuite) TestDeclineOffer_Success() {
	s.mockMembershipFetch(models.MembershipStatusOfferPending, ptr(3))

	s.mockDurableUpsert(models.MembershipStatusDeclined, nil)
	s.mockSyncCacheState(models.MembershipStatusDeclined, false, false)

	s.mockCache.EXPECT().RemoveFromExpiryTimer(s.ctx, "prod-1", "user-1").Return(nil)
	s.mockCache.EXPECT().RestoreAvailableUnits(s.ctx, "prod-1", 3).Return(nil)

	err := s.srv.DeclineOffer(s.ctx, "prod-1", "user-1")

	require.NoError(s.T(), err)
}

func (s *QueueServiceTestSuite) TestDeclineOffer_InvalidStatus() {
	s.mockMembershipFetch(models.MembershipStatusQueued, nil)

	err := s.srv.DeclineOffer(s.ctx, "prod-1", "user-1")

	require.ErrorIs(s.T(), err, models.ErrInvalidStatus)
}

func (s *QueueServiceTestSuite) TestDeclineOffer_NilAvailableQuantity() {
	s.mockMembershipFetch(models.MembershipStatusOfferPending, nil)

	err := s.srv.DeclineOffer(s.ctx, "prod-1", "user-1")

	require.ErrorIs(s.T(), err, models.ErrInvalidStatus)
}

func (s *QueueServiceTestSuite) TestDeclineOffer_UpsertError() {
	s.mockMembershipFetch(models.MembershipStatusOfferPending, ptr(3))

	dbErr := errors.New("db timeout")
	s.mockDurable.EXPECT().UpsertMembership(s.ctx, gomock.Any()).Return(dbErr)

	err := s.srv.DeclineOffer(s.ctx, "prod-1", "user-1")

	require.ErrorIs(s.T(), err, dbErr)
}

func (s *QueueServiceTestSuite) TestDeclineOffer_MembershipFetchError() {
	unexpectedErr := errors.New("redis timeout")
	s.mockCache.EXPECT().GetMembership(s.ctx, "prod-1", "user-1").Return(nil, unexpectedErr)

	err := s.srv.DeclineOffer(s.ctx, "prod-1", "user-1")

	require.ErrorIs(s.T(), err, unexpectedErr)
}
