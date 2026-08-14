package service_test

import (
	"errors"

	"backend/internal/models"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// TestDeclineOffer_Success verifies that rejecting an offer restores the full available
// quantity to the pool, removes the expiry timer, and advances the queue.
func (s *QueueServiceTestSuite) TestDeclineOffer_Success() {
	s.expectMembershipClaim("prod-1", "user-1")
	s.mockMembershipFetch(models.MembershipStatusOfferPending, ptr(3))

	s.mockDurableUpsert(models.MembershipStatusDeclined, nil)
	s.mockSyncCacheState(models.MembershipStatusDeclined, false, false)

	s.mockCache.EXPECT().RemoveFromExpiryTimer(s.ctx, "prod-1", "user-1").Return(nil)
	s.mockCache.EXPECT().RestoreAvailableUnits(s.ctx, "prod-1", 3).Return(nil)
	s.mockCache.EXPECT().PopAndAllocate(s.ctx, "prod-1").Return("", 0, 0, false, models.MembershipStatus(""), 0.0, nil)

	err := s.srv.DeclineOffer(s.ctx, "prod-1", "user-1")

	require.NoError(s.T(), err)
}

// TestDeclineOffer_InvalidStatus verifies that attempting to decline an offer
// is rejected if the user is not in the OFFER_PENDING state.
func (s *QueueServiceTestSuite) TestDeclineOffer_InvalidStatus() {
	s.expectMembershipClaim("prod-1", "user-1")
	s.mockMembershipFetch(models.MembershipStatusQueued, nil)

	err := s.srv.DeclineOffer(s.ctx, "prod-1", "user-1")

	require.ErrorIs(s.T(), err, models.ErrInvalidStatus)
}

// TestDeclineOffer_NilAvailableQuantity verifies that corrupted cache data missing the
// available quantity safely aborts the operation instead of causing a panic.
func (s *QueueServiceTestSuite) TestDeclineOffer_NilAvailableQuantity() {
	s.expectMembershipClaim("prod-1", "user-1")
	s.mockMembershipFetch(models.MembershipStatusOfferPending, nil)

	err := s.srv.DeclineOffer(s.ctx, "prod-1", "user-1")

	require.ErrorIs(s.T(), err, models.ErrInvalidStatus)
}

// TestDeclineOffer_UpsertError verifies that a database failure while updating the final
// declined state is correctly propagated back to the caller.
func (s *QueueServiceTestSuite) TestDeclineOffer_UpsertError() {
	s.expectMembershipClaim("prod-1", "user-1")
	s.mockMembershipFetch(models.MembershipStatusOfferPending, ptr(3))

	dbErr := errors.New("db timeout")
	s.mockDurable.EXPECT().UpsertMembership(s.ctx, gomock.Any()).Return(dbErr)

	err := s.srv.DeclineOffer(s.ctx, "prod-1", "user-1")

	require.ErrorIs(s.T(), err, dbErr)
}

// TestDeclineOffer_MembershipFetchError verifies that a cache connectivity issue
// during the initial state validation safely aborts the decline process.
func (s *QueueServiceTestSuite) TestDeclineOffer_MembershipFetchError() {
	s.expectMembershipClaim("prod-1", "user-1")
	unexpectedErr := errors.New("redis timeout")
	s.mockCache.EXPECT().GetMembership(s.ctx, "prod-1", "user-1").Return(nil, unexpectedErr)

	err := s.srv.DeclineOffer(s.ctx, "prod-1", "user-1")

	require.ErrorIs(s.T(), err, unexpectedErr)
}
