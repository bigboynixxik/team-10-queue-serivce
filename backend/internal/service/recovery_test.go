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
	s.mockCache.EXPECT().
		RestoreProductState(s.ctx, "prod-1", 10, 5, []string{"queued-1", "queued-2"}).
		Return(nil)
	s.mockCache.EXPECT().SetMembership(s.ctx, gomock.Any()).Return(nil).Times(len(snapshot.Memberships))
	s.mockCache.EXPECT().SetRight(s.ctx, gomock.Any()).Return(nil).Times(len(snapshot.Rights))
	s.mockCache.EXPECT().AddToExpiryTimer(
		s.ctx, "prod-1", "active-user", gomock.Cond(func(deadline time.Time) bool {
			earliest := time.Now().UTC().Add(29 * time.Second)
			latest := time.Now().UTC().Add(31 * time.Second)
			return !deadline.Before(earliest) && !deadline.After(latest)
		}),
	).Return(nil)
	s.mockCache.EXPECT().AddToExpiryTimer(s.ctx, "prod-1", "offer-user", offerExpiresAt).Return(nil)

	s.Require().NoError(s.srv.RecoverCache(s.ctx))
}

func (s *QueueServiceTestSuite) TestRecoverCache_RejectsNegativeAvailableUnits() {
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

	err := s.srv.RecoverCache(s.ctx)
	s.Require().Error(err)
	s.True(errors.Is(err, models.ErrStockDepleted))
}

func (s *QueueServiceTestSuite) TestRecoverCache_RejectsOrphanActiveRight() {
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

	err := s.srv.RecoverCache(s.ctx)
	s.Require().Error(err)
	s.True(errors.Is(err, models.ErrInvalidStatus))
}
