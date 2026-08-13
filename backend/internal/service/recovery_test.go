package service_test

import (
	"errors"
	"time"

	"backend/internal/models"

	"go.uber.org/mock/gomock"
)

func (s *QueueServiceTestSuite) TestRecoverCache_RestoresDurableState() {
	now := time.Now().UTC().Truncate(time.Second)
	activeToken := "active-token"
	usedOrderID := "order-used"
	usedAt := now.Add(-time.Minute)
	activeExpiresAt := now.Add(5 * time.Minute)
	offerExpiresAt := now.Add(2 * time.Minute)
	offerAvailable := 3

	snapshot := &models.RecoverySnapshot{
		Stocks: []*models.ProductStock{
			{ProductID: "prod-1", ProductCount: 10, TotalStock: 10, UpdatedAt: now},
		},
		Memberships: []*models.QueueMembership{
			{ProductID: "prod-1", UserID: "queued-1", Status: models.MembershipStatusQueued, Quantity: 1, CreatedAt: now, UpdatedAt: now},
			{ProductID: "prod-1", UserID: "active-user", Status: models.MembershipStatusRightActive, Quantity: 2, CurrentToken: &activeToken, ExpiresAt: &activeExpiresAt, CreatedAt: now, UpdatedAt: now},
			{ProductID: "prod-1", UserID: "offer-user", Status: models.MembershipStatusOfferPending, Quantity: 5, AvailableQuantity: &offerAvailable, ExpiresAt: &offerExpiresAt, CreatedAt: now, UpdatedAt: now},
			{ProductID: "prod-1", UserID: "queued-2", Status: models.MembershipStatusQueued, Quantity: 1, CreatedAt: now, UpdatedAt: now.Add(time.Second)},
		},
		Rights: []*models.Right{
			{Token: activeToken, UserID: "active-user", ProductID: "prod-1", Quantity: 2, Status: models.RightStatusActive, CreatedAt: now, ExpiresAt: activeExpiresAt},
			{Token: "used-token", UserID: "paid-user", ProductID: "prod-1", Quantity: 1, Status: models.RightStatusUsed, OrderID: &usedOrderID, CreatedAt: now, ExpiresAt: now.Add(time.Minute), UsedAt: &usedAt},
		},
	}

	s.mockDurable.EXPECT().LoadRecoverySnapshot(s.ctx).Return(snapshot, nil)
	s.mockCache.EXPECT().ResetExpiryTimers(s.ctx).Return(nil)
	s.mockCache.EXPECT().AddToExpiryTimer(s.ctx, "prod-1", "queued-1", gomock.Any()).Return(nil)
	s.mockCache.EXPECT().AddToExpiryTimer(s.ctx, "prod-1", "queued-2", gomock.Any()).Return(nil)
	s.mockCache.EXPECT().ResetQueueSlots(s.ctx).Return(nil)
	s.mockCache.EXPECT().
		RestoreProductState(s.ctx, "prod-1", 10, 5, []string{"queued-1", "queued-2"}).
		Return(nil)
	s.mockCache.EXPECT().SetMembership(s.ctx, gomock.Any()).Return(nil).Times(len(snapshot.Memberships))
	s.mockCache.EXPECT().SetRight(s.ctx, gomock.Any()).Return(nil).Times(len(snapshot.Rights))
	s.mockCache.EXPECT().AddToExpiryTimer(
		s.ctx, "prod-1", "active-user", activeExpiresAt,
	).Return(nil)
	s.mockCache.EXPECT().AddToExpiryTimer(s.ctx, "prod-1", "offer-user", offerExpiresAt).Return(nil)

	s.Require().NoError(s.srv.RecoverCache(s.ctx))
}

// Held units exceeding the stock is a contradiction between durable rows. The
// service still has to start, so the pool is closed instead of going negative.
func (s *QueueServiceTestSuite) TestRecoverCache_ClampsNegativeAvailableUnits() {
	now := time.Now().UTC()
	token := "too-big-right"
	expiresAt := now.Add(time.Minute)
	snapshot := &models.RecoverySnapshot{
		Stocks: []*models.ProductStock{
			{ProductID: "prod-1", ProductCount: 1, TotalStock: 1, UpdatedAt: now},
		},
		Memberships: []*models.QueueMembership{
			{ProductID: "prod-1", UserID: "active-user", Status: models.MembershipStatusRightActive, Quantity: 2, CurrentToken: &token, ExpiresAt: &expiresAt, CreatedAt: now, UpdatedAt: now},
		},
		Rights: []*models.Right{
			{Token: token, UserID: "active-user", ProductID: "prod-1", Quantity: 2, Status: models.RightStatusActive, CreatedAt: now, ExpiresAt: expiresAt},
		},
	}

	s.mockDurable.EXPECT().LoadRecoverySnapshot(s.ctx).Return(snapshot, nil)
	s.mockCache.EXPECT().ResetExpiryTimers(s.ctx).Return(nil)
	s.mockCache.EXPECT().ResetQueueSlots(s.ctx).Return(nil)
	s.mockCache.EXPECT().RestoreProductState(s.ctx, "prod-1", 1, 0, gomock.Any()).Return(nil)
	s.mockCache.EXPECT().SetMembership(s.ctx, gomock.Any()).Return(nil)
	s.mockCache.EXPECT().SetRight(s.ctx, gomock.Any()).Return(nil)
	s.mockCache.EXPECT().AddToExpiryTimer(s.ctx, "prod-1", "active-user", gomock.Any()).Return(nil)

	s.Require().NoError(s.srv.RecoverCache(s.ctx))
}

// An ACTIVE right no membership points at holds nothing: whatever ended that
// membership already returned its units. Recovery settles it instead of
// refusing to start, which would leave the service down for good.
func (s *QueueServiceTestSuite) TestRecoverCache_ExpiresOrphanActiveRight() {
	now := time.Now().UTC()
	snapshot := &models.RecoverySnapshot{
		Stocks: []*models.ProductStock{
			{ProductID: "prod-1", ProductCount: 3, TotalStock: 3, UpdatedAt: now},
		},
		Rights: []*models.Right{
			{Token: "orphan-token", UserID: "user-1", ProductID: "prod-1", Quantity: 1, Status: models.RightStatusActive, CreatedAt: now, ExpiresAt: now.Add(time.Minute)},
		},
	}

	s.mockDurable.EXPECT().LoadRecoverySnapshot(s.ctx).Return(snapshot, nil)
	s.mockDurable.EXPECT().ExpireRights(s.ctx, []string{"orphan-token"}).Return(nil)
	s.mockCache.EXPECT().ResetExpiryTimers(s.ctx).Return(nil)
	s.mockCache.EXPECT().ResetQueueSlots(s.ctx).Return(nil)
	s.mockCache.EXPECT().RestoreProductState(s.ctx, "prod-1", 3, 3, gomock.Any()).Return(nil)
	s.mockCache.EXPECT().SetRight(s.ctx, gomock.Cond(func(right *models.Right) bool {
		return right.Token == "orphan-token" && right.Status == models.RightStatusExpired
	})).Return(nil)

	s.Require().NoError(s.srv.RecoverCache(s.ctx))
}

// A RIGHT_ACTIVE membership whose right is already USED must not keep holding
// stock: it is settled as DECLINED, the state an expiry would have produced.
func (s *QueueServiceTestSuite) TestRecoverCache_SettlesMembershipWithoutUsableRight() {
	now := time.Now().UTC()
	staleToken := "used-token"
	expiresAt := now.Add(time.Minute)
	orderID := "order-1"
	snapshot := &models.RecoverySnapshot{
		Stocks: []*models.ProductStock{
			{ProductID: "prod-1", ProductCount: 2, TotalStock: 2, UpdatedAt: now},
		},
		Memberships: []*models.QueueMembership{
			{ProductID: "prod-1", UserID: "u1", Status: models.MembershipStatusRightActive, Quantity: 1, CurrentToken: &staleToken, ExpiresAt: &expiresAt, CreatedAt: now, UpdatedAt: now},
		},
		Rights: []*models.Right{
			{Token: staleToken, UserID: "u1", ProductID: "prod-1", Quantity: 1, Status: models.RightStatusUsed, OrderID: &orderID, CreatedAt: now, ExpiresAt: expiresAt},
		},
	}

	s.mockDurable.EXPECT().LoadRecoverySnapshot(s.ctx).Return(snapshot, nil)
	s.mockDurable.EXPECT().UpsertMembership(s.ctx, gomock.Cond(func(m *models.QueueMembership) bool {
		return m.Status == models.MembershipStatusDeclined && m.CurrentToken == nil && m.ExpiresAt == nil
	})).Return(nil)
	s.mockCache.EXPECT().ResetExpiryTimers(s.ctx).Return(nil)
	s.mockCache.EXPECT().ResetQueueSlots(s.ctx).Return(nil)
	// The membership holds nothing any more, so all two units stay available.
	s.mockCache.EXPECT().RestoreProductState(s.ctx, "prod-1", 2, 2, gomock.Any()).Return(nil)
	s.mockCache.EXPECT().SetMembership(s.ctx, gomock.Any()).Return(nil)
	s.mockCache.EXPECT().SetRight(s.ctx, gomock.Any()).Return(nil)

	s.Require().NoError(s.srv.RecoverCache(s.ctx))
}

// A failing store is still fatal: starting on a half-written cache would hand
// out stock that PostgreSQL believes is held.
func (s *QueueServiceTestSuite) TestRecoverCache_FailsOnStoreError() {
	now := time.Now().UTC()
	snapshot := &models.RecoverySnapshot{
		Stocks: []*models.ProductStock{
			{ProductID: "prod-1", ProductCount: 1, TotalStock: 1, UpdatedAt: now},
		},
	}

	s.mockDurable.EXPECT().LoadRecoverySnapshot(s.ctx).Return(snapshot, nil)
	s.mockCache.EXPECT().ResetExpiryTimers(s.ctx).Return(nil)
	s.mockCache.EXPECT().ResetQueueSlots(s.ctx).Return(nil)
	s.mockCache.EXPECT().RestoreProductState(s.ctx, "prod-1", 1, 1, gomock.Any()).Return(errors.New("redis down"))

	s.Require().Error(s.srv.RecoverCache(s.ctx))
}
