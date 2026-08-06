package service_test

import (
	"errors"

	"backend/internal/models"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// mockAdvanceQueueExit safely halts the AdvanceQueue infinite loop
// by simulating an empty queue during background processing.
func (s *QueueServiceTestSuite) mockAdvanceQueueExit(productID string) {
	s.mockCache.EXPECT().PopAndAllocate(gomock.Any(), productID).
		Return("", 0, 0, false, models.MembershipStatus(""), 0.0, nil)
}

// TestProcessPayment_Success_CacheHit verifies the standard happy path where a token
// is found in the cache and the full database transaction and state sync succeed.
func (s *QueueServiceTestSuite) TestProcessPayment_Success_CacheHit() {
	right := &models.Right{Token: "token-1", UserID: "user-1", ProductID: "prod-1", Quantity: 1, Status: models.RightStatusActive}
	mem := &models.QueueMembership{ProductID: "prod-1", UserID: "user-1", Status: models.MembershipStatusRightActive}

	s.mockCache.EXPECT().GetRight(s.ctx, "token-1").Return(right, nil)
	s.mockDurable.EXPECT().UpdateStockAndRightTx(s.ctx, "token-1", "order-1", 1).Return(nil)
	s.mockCache.EXPECT().CommitPurchase(gomock.Any(), "prod-1", 1).Return(nil)

	s.mockCache.EXPECT().GetMembership(s.ctx, "prod-1", "user-1").Return(mem, nil)
	s.mockDurable.EXPECT().UpsertMembership(s.ctx, gomock.Cond(func(x any) bool {
		m, ok := x.(*models.QueueMembership)
		return ok && m.Status == models.MembershipStatusPurchased
	})).Return(nil)

	s.mockCache.EXPECT().SetRight(s.ctx, gomock.Any()).Return(nil)
	s.mockCache.EXPECT().SetMembership(s.ctx, gomock.Any()).Return(nil)
	s.mockCache.EXPECT().PublishEvent(s.ctx, "prod-1", "user-1", map[string]string{"status": "PURCHASED"}).Return(nil)
	s.mockCache.EXPECT().RemoveFromExpiryTimer(s.ctx, "prod-1", "user-1").Return(nil)

	s.mockAdvanceQueueExit("prod-1")

	err := s.srv.ProcessPayment(s.ctx, "token-1", "order-1")

	require.NoError(s.T(), err)
}

// TestProcessPayment_Success_CacheMiss_DBHit verifies that the system correctly
// falls back to the database for token lookup without failing the payment process.
func (s *QueueServiceTestSuite) TestProcessPayment_Success_CacheMiss_DBHit() {
	right := &models.Right{Token: "token-db", UserID: "user-1", ProductID: "prod-1", Quantity: 1, Status: models.RightStatusActive}
	mem := &models.QueueMembership{ProductID: "prod-1", UserID: "user-1", Status: models.MembershipStatusRightActive}

	s.mockCache.EXPECT().GetRight(s.ctx, "token-db").Return(nil, models.ErrTokenNotFound)
	s.mockDurable.EXPECT().GetRightByToken(s.ctx, "token-db").Return(right, nil)

	s.mockDurable.EXPECT().UpdateStockAndRightTx(s.ctx, "token-db", "order-2", 1).Return(nil)
	s.mockCache.EXPECT().CommitPurchase(gomock.Any(), "prod-1", 1).Return(nil)

	s.mockCache.EXPECT().GetMembership(s.ctx, "prod-1", "user-1").Return(mem, nil)
	s.mockDurable.EXPECT().UpsertMembership(s.ctx, gomock.Any()).Return(nil)
	s.mockCache.EXPECT().SetRight(s.ctx, gomock.Any()).Return(nil)
	s.mockCache.EXPECT().SetMembership(s.ctx, gomock.Any()).Return(nil)
	s.mockCache.EXPECT().PublishEvent(s.ctx, "prod-1", "user-1", gomock.Any()).Return(nil)
	s.mockCache.EXPECT().RemoveFromExpiryTimer(s.ctx, "prod-1", "user-1").Return(nil)

	s.mockAdvanceQueueExit("prod-1")

	err := s.srv.ProcessPayment(s.ctx, "token-db", "order-2")

	require.NoError(s.T(), err)
}

// TestProcessPayment_Idempotency_AlreadyUsed verifies that duplicate webhook requests
// for already processed tokens return success immediately without side effects.
func (s *QueueServiceTestSuite) TestProcessPayment_Idempotency_AlreadyUsed() {
	right := &models.Right{Token: "token-used", Status: models.RightStatusUsed}

	s.mockCache.EXPECT().GetRight(s.ctx, "token-used").Return(right, nil)

	err := s.srv.ProcessPayment(s.ctx, "token-used", "order-3")

	require.NoError(s.T(), err)
}

// TestProcessPayment_NotFound verifies that unrecognized tokens correctly
// abort the payment workflow and return an explicit error.
func (s *QueueServiceTestSuite) TestProcessPayment_NotFound() {
	s.mockCache.EXPECT().GetRight(s.ctx, "invalid").Return(nil, models.ErrTokenNotFound)
	s.mockDurable.EXPECT().GetRightByToken(s.ctx, "invalid").Return(nil, models.ErrTokenNotFound)

	err := s.srv.ProcessPayment(s.ctx, "invalid", "order-4")

	require.ErrorIs(s.T(), err, models.ErrTokenNotFound)
}

// TestProcessPayment_Overselling_StockDepleted verifies that if physical stock is
// exhausted at the database level, the transaction bubbles up the depletion error.
func (s *QueueServiceTestSuite) TestProcessPayment_Overselling_StockDepleted() {
	right := &models.Right{Token: "token-late", ProductID: "prod-1", Quantity: 1, Status: models.RightStatusActive}

	s.mockCache.EXPECT().GetRight(s.ctx, "token-late").Return(right, nil)
	s.mockDurable.EXPECT().UpdateStockAndRightTx(s.ctx, "token-late", "order-5", 1).Return(models.ErrStockDepleted)

	err := s.srv.ProcessPayment(s.ctx, "token-late", "order-5")

	require.ErrorIs(s.T(), err, models.ErrStockDepleted)
}

// TestProcessPayment_Degraded_MembershipFetchFails verifies that a failure to retrieve
// cached user state does not roll back an already committed payment.
func (s *QueueServiceTestSuite) TestProcessPayment_Degraded_MembershipFetchFails() {
	right := &models.Right{Token: "token-6", UserID: "user-1", ProductID: "prod-1", Quantity: 1, Status: models.RightStatusActive}

	s.mockCache.EXPECT().GetRight(s.ctx, "token-6").Return(right, nil)
	s.mockDurable.EXPECT().UpdateStockAndRightTx(s.ctx, "token-6", "order-6", 1).Return(nil)
	s.mockCache.EXPECT().CommitPurchase(gomock.Any(), "prod-1", 1).Return(nil)

	s.mockCache.EXPECT().GetMembership(s.ctx, "prod-1", "user-1").Return(nil, errors.New("cache offline"))
	s.mockCache.EXPECT().SetRight(s.ctx, gomock.Any()).Return(nil)

	s.mockAdvanceQueueExit("prod-1")

	err := s.srv.ProcessPayment(s.ctx, "token-6", "order-6")

	require.NoError(s.T(), err)
}

// TestProcessPayment_Degraded_AdvanceQueueFails verifies that if the asynchronous queue
// advancement engine fails, the user's successful payment is preserved.
func (s *QueueServiceTestSuite) TestProcessPayment_Degraded_AdvanceQueueFails() {
	right := &models.Right{Token: "token-7", UserID: "user-1", ProductID: "prod-1", Quantity: 1, Status: models.RightStatusActive}
	mem := &models.QueueMembership{ProductID: "prod-1", UserID: "user-1", Status: models.MembershipStatusRightActive}

	s.mockCache.EXPECT().GetRight(s.ctx, "token-7").Return(right, nil)
	s.mockDurable.EXPECT().UpdateStockAndRightTx(s.ctx, "token-7", "order-7", 1).Return(nil)
	s.mockCache.EXPECT().CommitPurchase(gomock.Any(), "prod-1", 1).Return(nil)

	s.mockCache.EXPECT().GetMembership(s.ctx, "prod-1", "user-1").Return(mem, nil)
	s.mockDurable.EXPECT().UpsertMembership(s.ctx, gomock.Any()).Return(nil)
	s.mockCache.EXPECT().SetRight(s.ctx, gomock.Any()).Return(nil)
	s.mockCache.EXPECT().SetMembership(s.ctx, gomock.Any()).Return(nil)
	s.mockCache.EXPECT().PublishEvent(s.ctx, "prod-1", "user-1", gomock.Any()).Return(nil)
	s.mockCache.EXPECT().RemoveFromExpiryTimer(s.ctx, "prod-1", "user-1").Return(nil)

	s.mockCache.EXPECT().PopAndAllocate(gomock.Any(), "prod-1").
		Return("", 0, 0, false, models.MembershipStatus(""), 0.0, errors.New("lua script timeout"))

	err := s.srv.ProcessPayment(s.ctx, "token-7", "order-7")

	require.NoError(s.T(), err)
}
