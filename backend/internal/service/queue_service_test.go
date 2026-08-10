// Package service_test provides behavioral tests for the queue service business logic.
package service_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"backend/internal/models"
	"backend/internal/service"
	"backend/internal/service/mocks"

	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"
)

// QueueServiceTestSuite encapsulates the test suite for QueueService.
type QueueServiceTestSuite struct {
	suite.Suite
	ctrl        *gomock.Controller
	mockDurable *mocks.MockDurableRepo
	mockCache   *mocks.MockCacheRepo
	mockAvito   *mocks.MockAvitoClient
	srv         *service.QueueService
	ctx         context.Context
}

// SetupTest initializes the mocks and the service under test before each test case.
func (s *QueueServiceTestSuite) SetupTest() {
	s.ctrl = gomock.NewController(s.T())
	s.mockDurable = mocks.NewMockDurableRepo(s.ctrl)
	s.mockCache = mocks.NewMockCacheRepo(s.ctrl)
	s.mockAvito = mocks.NewMockAvitoClient(s.ctrl)
	s.ctx = context.Background()

	s.srv = service.NewQueueService(
		s.mockDurable,
		s.mockCache,
		s.mockAvito,
		2*time.Minute,
		4*time.Minute,
		75*time.Second,
		30*time.Second,
	)
}

// TearDownTest cleans up the mock controller after each test case.
func (s *QueueServiceTestSuite) TearDownTest() {
	s.ctrl.Finish()
}

// mockJoinQueueBase DRY helper for base dependencies.
func (s *QueueServiceTestSuite) mockJoinQueueBase(stock, reqQty, alloc, avail int, soldOut bool, allocErr error) {
	// The membership is read twice: once before taking the join claim and once
	// under it, since state may change while the claim is being acquired.
	s.mockCache.EXPECT().GetMembership(s.ctx, "prod-1", "user-1").
		Return(nil, models.ErrTokenNotFound).Times(2)
	s.mockCache.EXPECT().ClaimMembership(
		gomock.Any(), "prod-1", "user-1", gomock.Any(), gomock.Any(),
	).Return(true, nil)
	s.mockCache.EXPECT().ReleaseMembershipClaim(
		gomock.Any(), "prod-1", "user-1", gomock.Any(),
	).Return(nil)
	s.mockAvito.EXPECT().GetInitialStock(s.ctx, "prod-1").Return(stock, nil)
	s.mockCache.EXPECT().InitStock(s.ctx, "prod-1", stock).Return(nil)

	s.mockDurable.EXPECT().SaveInitialStock(s.ctx, gomock.Any()).Return(nil)

	s.mockCache.EXPECT().TryAllocate(s.ctx, "prod-1", reqQty).Return(alloc, avail, soldOut, allocErr)
}

// mockSyncCacheState DRY helper to mock Redis state synchronization.
func (s *QueueServiceTestSuite) mockSyncCacheState(status models.MembershipStatus, expectRight, expectTimer bool) {
	if expectRight {
		s.mockCache.EXPECT().SetRight(s.ctx, gomock.Any()).Return(nil)
	}
	s.mockCache.EXPECT().SetMembership(s.ctx, gomock.Any()).Return(nil)
	if expectTimer {
		var timerMatcher gomock.Matcher = gomock.Any()
		if status == models.MembershipStatusRightActive {
			earliest := time.Now().UTC().Add(29 * time.Second)
			latest := time.Now().UTC().Add(31 * time.Second)
			timerMatcher = gomock.Cond(func(deadline time.Time) bool {
				return !deadline.Before(earliest) && !deadline.After(latest)
			})
		}
		s.mockCache.EXPECT().AddToExpiryTimer(s.ctx, "prod-1", "user-1", timerMatcher).Return(nil)
	}
	s.mockCache.EXPECT().PublishEvent(s.ctx, "prod-1", "user-1", map[string]string{"status": string(status)}).Return(nil)
}

// mockDurableUpsert DRY helper to mock Postgres state persistence.
func (s *QueueServiceTestSuite) mockDurableUpsert(status models.MembershipStatus, expectedAvail *int) {
	s.mockDurable.EXPECT().UpsertMembership(s.ctx, gomock.Cond(func(x any) bool {
		m, ok := x.(*models.QueueMembership)
		if !ok || m.Status != status {
			return false
		}
		if expectedAvail != nil {
			return m.AvailableQuantity != nil && *m.AvailableQuantity == *expectedAvail
		}
		return true
	})).Return(nil)
}

// mockDurableIssue verifies the atomic durable write of a right and its owner.
func (s *QueueServiceTestSuite) mockDurableIssue(quantity int, err error) {
	s.mockDurable.EXPECT().IssueRightAndUpsertMembershipTx(
		s.ctx,
		gomock.Cond(func(x any) bool {
			right, ok := x.(*models.Right)
			return ok && right.Status == models.RightStatusActive && right.Quantity == quantity
		}),
		gomock.Cond(func(x any) bool {
			membership, ok := x.(*models.QueueMembership)
			return ok && membership.Status == models.MembershipStatusRightActive &&
				membership.CurrentToken != nil
		}),
	).Return(err)
}

// mockMembershipFetch DRY helper to mock a specific membership state retrieval.
func (s *QueueServiceTestSuite) mockMembershipFetch(status models.MembershipStatus, avail *int) {
	mem := &models.QueueMembership{
		ProductID:         "prod-1",
		UserID:            "user-1",
		Status:            status,
		AvailableQuantity: avail,
	}
	s.mockCache.EXPECT().GetMembership(s.ctx, "prod-1", "user-1").Return(mem, nil)
}

func (s *QueueServiceTestSuite) expectMembershipClaim(productID, userID string) {
	s.mockCache.EXPECT().ClaimMembership(
		gomock.Any(), productID, userID, gomock.Any(), gomock.Any(),
	).Return(true, nil)
	s.mockCache.EXPECT().ReleaseMembershipClaim(
		gomock.Any(), productID, userID, gomock.Any(),
	).Return(nil)
}

// mockAcceptOfferFetch is mockMembershipFetch plus the membership claim that
// AcceptOffer takes before deciding anything — the same guard JoinQueue uses.
func (s *QueueServiceTestSuite) mockAcceptOfferFetch(status models.MembershipStatus, avail *int) {
	s.mockCache.EXPECT().
		ClaimMembership(gomock.Any(), "prod-1", "user-1", gomock.Any(), gomock.Any()).
		Return(true, nil)
	s.mockCache.EXPECT().ReleaseMembershipClaim(
		gomock.Any(), "prod-1", "user-1", gomock.Any(),
	).Return(nil)
	s.mockMembershipFetch(status, avail)
}

func ptr[T any](v T) *T {
	return &v
}

// TestQueueServiceSuite acts as the entry point for running the test suite.
func TestQueueServiceSuite(t *testing.T) {
	suite.Run(t, new(QueueServiceTestSuite))
}

// expectExpirationClaim sets up one pass of the expiration worker: reclaiming
// abandoned work and claiming what is due. Acknowledgement is allowed rather than
// required; a failed item must be nacked explicitly by the test that expects it.
func (s *QueueServiceTestSuite) expectExpirationClaim(keys []string, err error) {
	deadline := time.Now().UTC().Add(-time.Minute).Truncate(time.Second)
	leaseUntil := time.Now().UTC().Add(time.Minute)
	claims := make([]models.ExpiryClaim, 0, len(keys))
	for _, key := range keys {
		claims = append(claims, models.ExpiryClaim{
			Key: key, Deadline: deadline, LeaseUntil: leaseUntil,
		})

		productID, userID, found := strings.Cut(key, ":")
		if !found || productID == "" || userID == "" {
			continue
		}
		s.mockCache.EXPECT().ClaimMembership(
			gomock.Any(), productID, userID, gomock.Any(), gomock.Any(),
		).Return(true, nil)
		s.mockCache.EXPECT().ReleaseMembershipClaim(
			gomock.Any(), productID, userID, gomock.Any(),
		).Return(nil)
	}

	s.mockCache.EXPECT().ReclaimStaleExpired(gomock.Any(), gomock.Any()).Return(0, nil).AnyTimes()
	s.mockCache.EXPECT().
		ClaimExpired(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(claims, err)
	s.mockCache.EXPECT().AckExpired(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
}
