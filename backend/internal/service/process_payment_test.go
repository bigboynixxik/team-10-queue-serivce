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

	s.mockDurable.EXPECT().UseRightTx(s.ctx, "token-1", "order-1", gomock.Any()).Return(right, true, nil)
	s.mockCache.EXPECT().CommitPurchase(gomock.Any(), "prod-1", 1).Return(nil)
	s.mockCache.EXPECT().SetRight(s.ctx, right).Return(nil)
	s.mockCache.EXPECT().MarkPurchasedIfCurrentToken(s.ctx, right, gomock.Any()).Return(true, nil)
	s.mockAdvanceQueueExit("prod-1")

	err := s.srv.ProcessPayment(s.ctx, "token-1", "order-1")

	require.NoError(s.T(), err)
}

func (s *QueueServiceTestSuite) TestProcessPayment_DuplicateDoesNotRepeatStockSideEffects() {
	right := usedRight("token-used", "order-1")

	s.mockDurable.EXPECT().UseRightTx(s.ctx, "token-used", "order-1", gomock.Any()).Return(right, false, nil)
	s.mockCache.EXPECT().SetRight(s.ctx, right).Return(nil)
	s.mockCache.EXPECT().MarkPurchasedIfCurrentToken(s.ctx, right, gomock.Any()).Return(false, nil)
	s.mockAdvanceQueueExit("prod-1")

	err := s.srv.ProcessPayment(s.ctx, "token-used", "order-1")

	require.NoError(s.T(), err)
}

func (s *QueueServiceTestSuite) TestProcessPayment_DuplicateRepairsMembership() {
	right := usedRight("token-used", "order-1")

	s.mockDurable.EXPECT().UseRightTx(s.ctx, "token-used", "order-1", gomock.Any()).Return(right, false, nil)
	s.mockCache.EXPECT().SetRight(s.ctx, right).Return(nil)
	s.mockCache.EXPECT().MarkPurchasedIfCurrentToken(s.ctx, right, gomock.Any()).Return(true, nil)
	s.mockAdvanceQueueExit("prod-1")

	err := s.srv.ProcessPayment(s.ctx, "token-used", "order-1")

	require.NoError(s.T(), err)
}

func (s *QueueServiceTestSuite) TestProcessPayment_StaleWebhookDoesNotReplaceCurrentMembership() {
	right := usedRight("old-token", "order-old")

	s.mockDurable.EXPECT().UseRightTx(s.ctx, "old-token", "order-old", gomock.Any()).Return(right, false, nil)
	s.mockCache.EXPECT().SetRight(s.ctx, right).Return(nil)
	s.mockCache.EXPECT().MarkPurchasedIfCurrentToken(s.ctx, right, gomock.Any()).Return(false, nil)
	s.mockAdvanceQueueExit("prod-1")

	err := s.srv.ProcessPayment(s.ctx, "old-token", "order-old")

	require.NoError(s.T(), err)
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

func (s *QueueServiceTestSuite) TestProcessPayment_Degraded_MembershipCacheUpdateFails() {
	right := usedRight("token-6", "order-6")

	s.mockDurable.EXPECT().UseRightTx(s.ctx, "token-6", "order-6", gomock.Any()).Return(right, true, nil)
	s.mockCache.EXPECT().CommitPurchase(gomock.Any(), "prod-1", 1).Return(nil)
	s.mockCache.EXPECT().SetRight(s.ctx, right).Return(nil)
	s.mockCache.EXPECT().MarkPurchasedIfCurrentToken(s.ctx, right, gomock.Any()).Return(false, errors.New("cache offline"))
	s.mockAdvanceQueueExit("prod-1")

	err := s.srv.ProcessPayment(s.ctx, "token-6", "order-6")

	require.ErrorContains(s.T(), err, "cache purchased membership")
}

func (s *QueueServiceTestSuite) TestProcessPayment_Degraded_AdvanceQueueFails() {
	right := usedRight("token-7", "order-7")

	s.mockDurable.EXPECT().UseRightTx(s.ctx, "token-7", "order-7", gomock.Any()).Return(right, true, nil)
	s.mockCache.EXPECT().CommitPurchase(gomock.Any(), "prod-1", 1).Return(nil)
	s.mockCache.EXPECT().SetRight(s.ctx, right).Return(nil)
	s.mockCache.EXPECT().MarkPurchasedIfCurrentToken(s.ctx, right, gomock.Any()).Return(true, nil)
	s.mockCache.EXPECT().PopAndAllocate(gomock.Any(), "prod-1").
		Return("", 0, 0, false, models.MembershipStatus(""), 0.0, errors.New("lua script timeout"))

	err := s.srv.ProcessPayment(s.ctx, "token-7", "order-7")

	require.ErrorContains(s.T(), err, "advance queue")
}
