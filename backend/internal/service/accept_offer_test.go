package service_test

import (
	"errors"

	"backend/internal/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// TestAcceptOffer_Success_Full verifies that accepting the exact offered quantity
// successfully creates an active right and updates the user's status to RIGHT_ACTIVE.
func (s *QueueServiceTestSuite) TestAcceptOffer_Success_Full() {
	s.mockAcceptOfferFetch(models.MembershipStatusOfferPending, ptr(2))

	s.mockDurableIssue(2, nil)
	s.mockSyncCacheState(models.MembershipStatusRightActive, true, true)

	right, err := s.srv.AcceptOffer(s.ctx, "prod-1", "user-1", 2)

	require.NoError(s.T(), err)
	require.NotNil(s.T(), right)
	assert.Equal(s.T(), 2, right.Quantity)
	assert.Equal(s.T(), models.RightStatusActive, right.Status)
}

// TestAcceptOffer_Success_Partial verifies that accepting less than the offered quantity
// creates an active right, restores the unused stock to the pool, and advances the queue.
func (s *QueueServiceTestSuite) TestAcceptOffer_Success_Partial() {
	s.mockAcceptOfferFetch(models.MembershipStatusOfferPending, ptr(5))

	s.mockDurableIssue(2, nil)
	s.mockSyncCacheState(models.MembershipStatusRightActive, true, true)

	s.mockCache.EXPECT().RestoreAvailableUnits(s.ctx, "prod-1", 3).Return(nil)
	s.mockCache.EXPECT().PopAndAllocate(s.ctx, "prod-1").Return("", 0, 0, false, models.MembershipStatus(""), 0.0, nil)

	right, err := s.srv.AcceptOffer(s.ctx, "prod-1", "user-1", 2)

	require.NoError(s.T(), err)
	require.NotNil(s.T(), right)
	assert.Equal(s.T(), 2, right.Quantity)
}

// TestAcceptOffer_MembershipFetchError verifies that a failure to retrieve the user's
// current membership state from the cache aborts the acceptance process.
func (s *QueueServiceTestSuite) TestAcceptOffer_MembershipFetchError() {
	expectedErr := errors.New("redis timeout")
	s.mockCache.EXPECT().
		ClaimMembership(gomock.Any(), "prod-1", "user-1", gomock.Any(), gomock.Any()).
		Return(true, nil)
	s.mockCache.EXPECT().ReleaseMembershipClaim(
		gomock.Any(), "prod-1", "user-1", gomock.Any(),
	).Return(nil)
	s.mockCache.EXPECT().GetMembership(s.ctx, "prod-1", "user-1").Return(nil, expectedErr)

	right, err := s.srv.AcceptOffer(s.ctx, "prod-1", "user-1", 2)

	require.ErrorIs(s.T(), err, expectedErr)
	assert.Nil(s.T(), right)
}

// TestAcceptOffer_InvalidQuantity verifies that attempting to accept zero or a negative
// quantity of items returns an appropriate validation error.
func (s *QueueServiceTestSuite) TestAcceptOffer_InvalidQuantity() {
	right, err := s.srv.AcceptOffer(s.ctx, "prod-1", "user-1", 0)

	require.ErrorIs(s.T(), err, models.ErrQuantityInvalid)
	assert.Nil(s.T(), right)
}

// TestAcceptOffer_InvalidStatus verifies that accepting an offer is rejected if the user
// is not currently in the OFFER_PENDING state.
func (s *QueueServiceTestSuite) TestAcceptOffer_InvalidStatus() {
	s.mockAcceptOfferFetch(models.MembershipStatusQueued, nil)

	right, err := s.srv.AcceptOffer(s.ctx, "prod-1", "user-1", 2)

	require.ErrorIs(s.T(), err, models.ErrInvalidStatus)
	assert.Nil(s.T(), right)
}

// TestAcceptOffer_NilAvailableQuantity verifies that corrupted membership data with a missing
// available quantity pointer aborts the operation to prevent nil pointer dereferences.
func (s *QueueServiceTestSuite) TestAcceptOffer_NilAvailableQuantity() {
	s.mockAcceptOfferFetch(models.MembershipStatusOfferPending, nil)

	right, err := s.srv.AcceptOffer(s.ctx, "prod-1", "user-1", 2)

	require.ErrorIs(s.T(), err, models.ErrInvalidStatus)
	assert.Nil(s.T(), right)
}

// TestAcceptOffer_ExceedsAvailable verifies that requesting more items than initially
// offered is rejected with a strict boundary error.
func (s *QueueServiceTestSuite) TestAcceptOffer_ExceedsAvailable() {
	s.mockAcceptOfferFetch(models.MembershipStatusOfferPending, ptr(2))

	right, err := s.srv.AcceptOffer(s.ctx, "prod-1", "user-1", 5)

	require.ErrorIs(s.T(), err, models.ErrQuantityExceeded)
	assert.Nil(s.T(), right)
}

// TestAcceptOffer_SaveRightError verifies that a database failure during the creation
// of the purchase right correctly propagates the error upward.
func (s *QueueServiceTestSuite) TestAcceptOffer_SaveRightError() {
	s.mockAcceptOfferFetch(models.MembershipStatusOfferPending, ptr(2))

	dbErr := errors.New("db save right error")
	s.mockDurableIssue(2, dbErr)

	right, err := s.srv.AcceptOffer(s.ctx, "prod-1", "user-1", 2)

	require.ErrorIs(s.T(), err, dbErr)
	assert.Nil(s.T(), right)
}

// TestAcceptOffer_ConcurrentClaimLost verifies that a second click on «take N»
// returns the right the first one produced, instead of issuing another. Without
// the claim, ten parallel accepts turned an offer of two units into ten rights.
func (s *QueueServiceTestSuite) TestAcceptOffer_ConcurrentClaimLost() {
	token := "winner-token"
	right := &models.Right{Token: token, UserID: "user-1", ProductID: "prod-1", Quantity: 2}
	mem := &models.QueueMembership{
		ProductID:    "prod-1",
		UserID:       "user-1",
		Status:       models.MembershipStatusRightActive,
		CurrentToken: &token,
	}

	s.mockCache.EXPECT().
		ClaimMembership(gomock.Any(), "prod-1", "user-1", gomock.Any(), gomock.Any()).
		Return(false, nil)
	s.mockCache.EXPECT().GetMembership(s.ctx, "prod-1", "user-1").Return(mem, nil)
	s.mockCache.EXPECT().GetRight(s.ctx, token).Return(right, nil)

	got, err := s.srv.AcceptOffer(s.ctx, "prod-1", "user-1", 2)

	require.NoError(s.T(), err)
	assert.Equal(s.T(), right, got, "the loser must return the winner's right, not a new one")
}

// TestAcceptOffer_ConcurrentOfferGone verifies that losing the claim to a decline
// reports a conflict rather than inventing a right out of nothing.
func (s *QueueServiceTestSuite) TestAcceptOffer_ConcurrentOfferGone() {
	mem := &models.QueueMembership{
		ProductID: "prod-1",
		UserID:    "user-1",
		Status:    models.MembershipStatusDeclined,
	}

	s.mockCache.EXPECT().
		ClaimMembership(gomock.Any(), "prod-1", "user-1", gomock.Any(), gomock.Any()).
		Return(false, nil)
	s.mockCache.EXPECT().GetMembership(s.ctx, "prod-1", "user-1").Return(mem, nil)

	got, err := s.srv.AcceptOffer(s.ctx, "prod-1", "user-1", 2)

	require.ErrorIs(s.T(), err, models.ErrInvalidStatus)
	assert.Nil(s.T(), got)
}
