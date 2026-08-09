package service_test

import (
	"backend/internal/models"
	"errors"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// TestProcessExpirations_Empty verifies that an empty expiration set
// terminates early without invoking storage or queue advancement operations.
func (s *QueueServiceTestSuite) TestProcessExpirations_Empty() {
	s.expectExpirationClaim([]string{}, nil)

	err := s.srv.ProcessExpirations(s.ctx)

	require.NoError(s.T(), err)
}

// TestProcessExpirations_OfferPending verifies that an expired partial offer
// restores available units, updates the membership status, and advances the queue.
func (s *QueueServiceTestSuite) TestProcessExpirations_OfferPending() {
	s.expectExpirationClaim([]string{"prod-1:user-1"}, nil)

	mem := &models.QueueMembership{
		ProductID:         "prod-1",
		UserID:            "user-1",
		Status:            models.MembershipStatusOfferPending,
		AvailableQuantity: ptr(3),
	}

	s.mockCache.EXPECT().GetMembership(s.ctx, "prod-1", "user-1").Return(mem, nil)
	s.mockDurable.EXPECT().UpsertMembership(s.ctx, gomock.Cond(func(x any) bool {
		m, ok := x.(*models.QueueMembership)
		return ok && m.Status == models.MembershipStatusDeclined && m.AvailableQuantity == nil
	})).Return(nil)

	s.mockCache.EXPECT().SetMembership(s.ctx, gomock.Any()).Return(nil)
	s.mockCache.EXPECT().PublishEvent(s.ctx, "prod-1", "user-1", map[string]string{"status": string(models.MembershipStatusDeclined)}).Return(nil)
	s.mockCache.EXPECT().RestoreAvailableUnits(s.ctx, "prod-1", 3).Return(nil)

	s.mockCache.EXPECT().PopAndAllocate(gomock.Any(), "prod-1").Return("", 0, 0, false, models.MembershipStatus(""), 0.0, nil)

	err := s.srv.ProcessExpirations(s.ctx)

	require.NoError(s.T(), err)
}

// TestProcessExpirations_RightActive verifies that an expired payment right
// restores full product quantity, updates status, and advances the queue.
func (s *QueueServiceTestSuite) TestProcessExpirations_RightActive() {
	s.expectExpirationClaim([]string{"prod-1:user-1"}, nil)

	token := "right-token"
	mem := &models.QueueMembership{
		ProductID:    "prod-1",
		UserID:       "user-1",
		Status:       models.MembershipStatusRightActive,
		Quantity:     2,
		CurrentToken: &token,
	}
	expiredRight := &models.Right{
		Token:     token,
		UserID:    "user-1",
		ProductID: "prod-1",
		Quantity:  2,
		Status:    models.RightStatusExpired,
	}

	s.mockCache.EXPECT().GetMembership(s.ctx, "prod-1", "user-1").Return(mem, nil)
	s.mockDurable.EXPECT().ExpireRightAndUpsertMembershipTx(s.ctx, token, gomock.Cond(func(value any) bool {
		membership, ok := value.(*models.QueueMembership)
		return ok && membership.Status == models.MembershipStatusDeclined && membership.CurrentToken == nil
	})).Return(expiredRight, true, nil)
	s.mockCache.EXPECT().SetRight(s.ctx, expiredRight).Return(nil)
	s.mockCache.EXPECT().SetMembership(s.ctx, gomock.Any()).Return(nil)
	s.mockCache.EXPECT().PublishEvent(s.ctx, "prod-1", "user-1", map[string]string{"status": "DECLINED"}).Return(nil)
	s.mockCache.EXPECT().RestoreAvailableUnits(s.ctx, "prod-1", 2).Return(nil)
	s.mockCache.EXPECT().PopAndAllocate(gomock.Any(), "prod-1").Return("", 0, 0, false, models.MembershipStatus(""), 0.0, nil)

	err := s.srv.ProcessExpirations(s.ctx)

	require.NoError(s.T(), err)
}

// belonging to different products correctly route stock restoration and queue advancement per product.
func (s *QueueServiceTestSuite) TestProcessExpirations_MultipleProducts() {
	s.expectExpirationClaim([]string{"prod-1:user-1", "prod-2:user-2"}, nil)

	mem1 := &models.QueueMembership{
		ProductID:         "prod-1",
		UserID:            "user-1",
		Status:            models.MembershipStatusOfferPending,
		AvailableQuantity: ptr(1),
	}
	mem2 := &models.QueueMembership{
		ProductID:         "prod-2",
		UserID:            "user-2",
		Status:            models.MembershipStatusOfferPending,
		AvailableQuantity: ptr(4),
	}

	s.mockCache.EXPECT().GetMembership(s.ctx, "prod-1", "user-1").Return(mem1, nil)
	s.mockDurable.EXPECT().UpsertMembership(s.ctx, gomock.Any()).Return(nil)
	s.mockCache.EXPECT().SetMembership(s.ctx, gomock.Any()).Return(nil)
	s.mockCache.EXPECT().PublishEvent(s.ctx, "prod-1", "user-1", gomock.Any()).Return(nil)
	s.mockCache.EXPECT().RestoreAvailableUnits(s.ctx, "prod-1", 1).Return(nil)
	s.mockCache.EXPECT().PopAndAllocate(gomock.Any(), "prod-1").Return("", 0, 0, false, models.MembershipStatus(""), 0.0, nil)

	s.mockCache.EXPECT().GetMembership(s.ctx, "prod-2", "user-2").Return(mem2, nil)
	s.mockDurable.EXPECT().UpsertMembership(s.ctx, gomock.Any()).Return(nil)
	s.mockCache.EXPECT().SetMembership(s.ctx, gomock.Any()).Return(nil)
	s.mockCache.EXPECT().PublishEvent(s.ctx, "prod-2", "user-2", gomock.Any()).Return(nil)
	s.mockCache.EXPECT().RestoreAvailableUnits(s.ctx, "prod-2", 4).Return(nil)
	s.mockCache.EXPECT().PopAndAllocate(gomock.Any(), "prod-2").Return("", 0, 0, false, models.MembershipStatus(""), 0.0, nil)

	err := s.srv.ProcessExpirations(s.ctx)

	require.NoError(s.T(), err)
}

// TestProcessExpirations_MalformedKey verifies that corrupt or improperly formatted
// keys in the expiration index are safely ignored without crashing the worker.
func (s *QueueServiceTestSuite) TestProcessExpirations_MalformedKey() {
	s.expectExpirationClaim([]string{"malformedkeywithoutcolon"}, nil)

	err := s.srv.ProcessExpirations(s.ctx)

	require.NoError(s.T(), err)
}

// TestProcessExpirations_AlreadyHandled verifies that users who already completed
// their purchase are skipped during the expiration sweep.
func (s *QueueServiceTestSuite) TestProcessExpirations_AlreadyHandled() {
	s.expectExpirationClaim([]string{"prod-1:user-1"}, nil)

	mem := &models.QueueMembership{
		ProductID: "prod-1",
		UserID:    "user-1",
		Status:    models.MembershipStatus("PURCHASED"),
	}

	s.mockCache.EXPECT().GetMembership(s.ctx, "prod-1", "user-1").Return(mem, nil)

	err := s.srv.ProcessExpirations(s.ctx)

	require.NoError(s.T(), err)
}

// TestProcessExpirations_MembershipFetchError verifies that temporary infrastructure
// errors when fetching user details are safely caught, allowing the loop to continue.
func (s *QueueServiceTestSuite) TestProcessExpirations_MembershipFetchError() {
	s.expectExpirationClaim([]string{"prod-1:user-1"}, nil)

	s.mockCache.EXPECT().GetMembership(s.ctx, "prod-1", "user-1").Return(nil, errors.New("timeout"))
	// The item is not acknowledged: it goes back to the schedule so the next pass
	// retries it once the cache recovers.
	s.mockCache.EXPECT().
		NackExpired(gomock.Any(), []string{"prod-1:user-1"}, gomock.Any()).
		Return(nil)

	err := s.srv.ProcessExpirations(s.ctx)

	require.NoError(s.T(), err)
}
func (s *QueueServiceTestSuite) TestProcessExpirations_PaymentWonRace() {
	s.expectExpirationClaim([]string{"prod-1:user-1"}, nil)

	token := "right-token"
	mem := &models.QueueMembership{
		ProductID:    "prod-1",
		UserID:       "user-1",
		Status:       models.MembershipStatusRightActive,
		Quantity:     2,
		CurrentToken: &token,
	}
	usedRight := &models.Right{
		Token:     token,
		UserID:    "user-1",
		ProductID: "prod-1",
		Quantity:  2,
		Status:    models.RightStatusUsed,
	}

	s.mockCache.EXPECT().GetMembership(s.ctx, "prod-1", "user-1").Return(mem, nil)
	s.mockDurable.EXPECT().ExpireRightAndUpsertMembershipTx(s.ctx, token, gomock.Any()).Return(usedRight, false, nil)
	s.mockCache.EXPECT().SetRight(s.ctx, usedRight).Return(nil)

	err := s.srv.ProcessExpirations(s.ctx)

	require.NoError(s.T(), err)
}
func (s *QueueServiceTestSuite) TestProcessExpirations_RightTransactionErrorIsRescheduled() {
	s.expectExpirationClaim([]string{"prod-1:user-1"}, nil)

	token := "right-token"
	mem := &models.QueueMembership{
		ProductID:    "prod-1",
		UserID:       "user-1",
		Status:       models.MembershipStatusRightActive,
		Quantity:     2,
		CurrentToken: &token,
	}

	s.mockCache.EXPECT().GetMembership(s.ctx, "prod-1", "user-1").Return(mem, nil)
	s.mockDurable.EXPECT().ExpireRightAndUpsertMembershipTx(s.ctx, token, gomock.Any()).Return(nil, false, errors.New("postgres unavailable"))
	s.mockCache.EXPECT().
		NackExpired(gomock.Any(), []string{"prod-1:user-1"}, gomock.Any()).
		Return(nil)

	err := s.srv.ProcessExpirations(s.ctx)

	require.NoError(s.T(), err)
}
