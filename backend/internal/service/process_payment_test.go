package service_test

import (
	"errors"
	"time"

	"backend/internal/models"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// mockAdvanceQueueExit safely halts the AdvanceQueue loop.
func (s *QueueServiceTestSuite) mockAdvanceQueueExit(productID string) {
	s.mockCache.EXPECT().PopAndAllocate(gomock.Any(), productID).
		Return("", 0, 0, false, models.MembershipStatus(""), 0.0, nil)
}

func usedRight(token, orderID string) *models.Right {
	usedAt := time.Now().UTC()
	return &models.Right{
		Token:     token,
		UserID:    "user-1",
		ProductID: "prod-1",
		Quantity:  1,
		Status:    models.RightStatusUsed,
		OrderID:   &orderID,
		ExpiresAt: usedAt.Add(time.Minute),
		UsedAt:    &usedAt,
	}
}

func (s *QueueServiceTestSuite) TestProcessPayment_Success() {
	right := usedRight("token-1", "order-1")
	token := right.Token
	expiresAt := right.ExpiresAt
	mem := &models.QueueMembership{
		ProductID:    "prod-1",
		UserID:       "user-1",
		Status:       models.MembershipStatusRightActive,
		Quantity:     1,
		CurrentToken: &token,
		ExpiresAt:    &expiresAt,
	}

	s.mockDurable.EXPECT().UseRightTx(s.ctx, "token-1", "order-1", gomock.Any()).Return(right, true, nil)
	s.mockCache.EXPECT().CommitPurchase(gomock.Any(), "prod-1", 1).Return(nil)
	s.mockAvito.EXPECT().DecrementStock(s.ctx, "prod-1", 1).Return(nil)
	s.mockCache.EXPECT().GetMembership(s.ctx, "prod-1", "user-1").Return(mem, nil)
	s.mockDurable.EXPECT().UpsertMembership(s.ctx, gomock.Cond(func(value any) bool {
		membership, ok := value.(*models.QueueMembership)
		return ok && membership.Status == models.MembershipStatusPurchased &&
			membership.CurrentToken == nil && membership.ExpiresAt == nil
	})).Return(nil)
	s.mockCache.EXPECT().SetRight(s.ctx, right).Return(nil)
	s.mockCache.EXPECT().SetMembership(s.ctx, gomock.Any()).Return(nil)
	s.mockCache.EXPECT().PublishEvent(s.ctx, "prod-1", "user-1", map[string]string{"status": "PURCHASED"}).Return(nil)
	s.mockCache.EXPECT().RemoveFromExpiryTimer(s.ctx, "prod-1", "user-1").Return(nil)
	s.mockAdvanceQueueExit("prod-1")

	err := s.srv.ProcessPayment(s.ctx, "token-1", "order-1")

	require.NoError(s.T(), err)
}

func (s *QueueServiceTestSuite) TestProcessPayment_DuplicateDoesNotRepeatStockSideEffects() {
	right := usedRight("token-used", "order-1")
	mem := &models.QueueMembership{
		ProductID: "prod-1",
		UserID:    "user-1",
		Status:    models.MembershipStatusPurchased,
	}

	s.mockDurable.EXPECT().UseRightTx(s.ctx, "token-used", "order-1", gomock.Any()).Return(right, false, nil)
	s.mockCache.EXPECT().GetMembership(s.ctx, "prod-1", "user-1").Return(mem, nil)
	s.mockCache.EXPECT().SetRight(s.ctx, right).Return(nil)
	s.mockCache.EXPECT().RemoveFromExpiryTimer(s.ctx, "prod-1", "user-1").Return(nil)

	err := s.srv.ProcessPayment(s.ctx, "token-used", "order-1")

	require.NoError(s.T(), err)
}

func (s *QueueServiceTestSuite) TestProcessPayment_DuplicateRepairsMembership() {
	right := usedRight("token-used", "order-1")
	token := right.Token
	mem := &models.QueueMembership{
		ProductID:    "prod-1",
		UserID:       "user-1",
		Status:       models.MembershipStatusRightActive,
		CurrentToken: &token,
	}

	s.mockDurable.EXPECT().UseRightTx(s.ctx, "token-used", "order-1", gomock.Any()).Return(right, false, nil)
	s.mockCache.EXPECT().GetMembership(s.ctx, "prod-1", "user-1").Return(mem, nil)
	s.mockDurable.EXPECT().UpsertMembership(s.ctx, gomock.Any()).Return(nil)
	s.mockCache.EXPECT().SetRight(s.ctx, right).Return(nil)
	s.mockCache.EXPECT().SetMembership(s.ctx, gomock.Any()).Return(nil)
	s.mockCache.EXPECT().PublishEvent(s.ctx, "prod-1", "user-1", gomock.Any()).Return(nil)
	s.mockCache.EXPECT().RemoveFromExpiryTimer(s.ctx, "prod-1", "user-1").Return(nil)

	err := s.srv.ProcessPayment(s.ctx, "token-used", "order-1")

	require.NoError(s.T(), err)
	require.Equal(s.T(), models.MembershipStatusPurchased, mem.Status)
	require.Nil(s.T(), mem.CurrentToken)
}

func (s *QueueServiceTestSuite) TestProcessPayment_NotFound() {
	s.mockDurable.EXPECT().UseRightTx(s.ctx, "invalid", "order-4", gomock.Any()).Return(nil, false, models.ErrTokenNotFound)

	err := s.srv.ProcessPayment(s.ctx, "invalid", "order-4")

	require.ErrorIs(s.T(), err, models.ErrTokenNotFound)
}

func (s *QueueServiceTestSuite) TestProcessPayment_Expired() {
	s.mockDurable.EXPECT().UseRightTx(s.ctx, "expired", "order-4", gomock.Any()).Return(nil, false, models.ErrTokenExpired)

	err := s.srv.ProcessPayment(s.ctx, "expired", "order-4")

	require.ErrorIs(s.T(), err, models.ErrTokenExpired)
}

func (s *QueueServiceTestSuite) TestProcessPayment_StockDepleted() {
	s.mockDurable.EXPECT().UseRightTx(s.ctx, "token-late", "order-5", gomock.Any()).Return(nil, false, models.ErrStockDepleted)

	err := s.srv.ProcessPayment(s.ctx, "token-late", "order-5")

	require.ErrorIs(s.T(), err, models.ErrStockDepleted)
}

func (s *QueueServiceTestSuite) TestProcessPayment_Degraded_MembershipFetchFails() {
	right := usedRight("token-6", "order-6")

	s.mockDurable.EXPECT().UseRightTx(s.ctx, "token-6", "order-6", gomock.Any()).Return(right, true, nil)
	s.mockCache.EXPECT().CommitPurchase(gomock.Any(), "prod-1", 1).Return(nil)
	s.mockAvito.EXPECT().DecrementStock(s.ctx, "prod-1", 1).Return(nil)
	s.mockCache.EXPECT().GetMembership(s.ctx, "prod-1", "user-1").Return(nil, errors.New("cache offline"))
	s.mockCache.EXPECT().SetRight(s.ctx, right).Return(nil)
	s.mockAdvanceQueueExit("prod-1")

	err := s.srv.ProcessPayment(s.ctx, "token-6", "order-6")

	require.NoError(s.T(), err)
}

func (s *QueueServiceTestSuite) TestProcessPayment_Degraded_AdvanceQueueFails() {
	right := usedRight("token-7", "order-7")
	mem := &models.QueueMembership{ProductID: "prod-1", UserID: "user-1", Status: models.MembershipStatusRightActive}

	s.mockDurable.EXPECT().UseRightTx(s.ctx, "token-7", "order-7", gomock.Any()).Return(right, true, nil)
	s.mockCache.EXPECT().CommitPurchase(gomock.Any(), "prod-1", 1).Return(nil)
	s.mockAvito.EXPECT().DecrementStock(s.ctx, "prod-1", 1).Return(nil)
	s.mockCache.EXPECT().GetMembership(s.ctx, "prod-1", "user-1").Return(mem, nil)
	s.mockDurable.EXPECT().UpsertMembership(s.ctx, gomock.Any()).Return(nil)
	s.mockCache.EXPECT().SetRight(s.ctx, right).Return(nil)
	s.mockCache.EXPECT().SetMembership(s.ctx, gomock.Any()).Return(nil)
	s.mockCache.EXPECT().PublishEvent(s.ctx, "prod-1", "user-1", gomock.Any()).Return(nil)
	s.mockCache.EXPECT().RemoveFromExpiryTimer(s.ctx, "prod-1", "user-1").Return(nil)
	s.mockCache.EXPECT().PopAndAllocate(gomock.Any(), "prod-1").
		Return("", 0, 0, false, models.MembershipStatus(""), 0.0, errors.New("lua script timeout"))

	err := s.srv.ProcessPayment(s.ctx, "token-7", "order-7")

	require.NoError(s.T(), err)
}
