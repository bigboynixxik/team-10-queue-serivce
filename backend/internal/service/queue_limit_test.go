package service_test

import (
	"errors"
	"testing"

	"backend/internal/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A refused join stops before it touches anything: no external call, no stock,
// no membership. The user is told the limit so the client can name it.
func (s *QueueServiceTestSuite) TestJoinQueue_RefusedAtQueueLimit() {
	s.mockCache.EXPECT().GetMembership(s.ctx, "prod-1", "user-1").
		Return(nil, models.ErrTokenNotFound).Times(2)
	s.expectMembershipClaim("prod-1", "user-1")
	s.mockCache.EXPECT().TryOccupyQueueSlot(s.ctx, "user-1", "prod-1", 5).
		Return(false, false, nil)

	mem, right, err := s.srv.JoinQueue(s.ctx, "prod-1", "user-1", 1)

	require.ErrorIs(s.T(), err, models.ErrQueueLimitReached)
	assert.Nil(s.T(), mem)
	assert.Nil(s.T(), right)

	var limitErr *models.QueueLimitError
	require.ErrorAs(s.T(), err, &limitErr)
	assert.Equal(s.T(), 5, limitErr.Limit)
}

// A slot the user already held belongs to an earlier membership. A failed join
// must leave it alone, or one bad attempt would evict them from a queue they
// are legitimately waiting in.
func (s *QueueServiceTestSuite) TestJoinQueue_KeepsAlreadyHeldSlotOnFailure() {
	s.mockCache.EXPECT().GetMembership(s.ctx, "prod-1", "user-1").
		Return(nil, models.ErrTokenNotFound).Times(2)
	s.expectMembershipClaim("prod-1", "user-1")
	s.mockCache.EXPECT().TryOccupyQueueSlot(s.ctx, "user-1", "prod-1", 5).
		Return(true, false, nil)

	avitoErr := errors.New("avito client error")
	s.mockAvito.EXPECT().GetInitialStock(s.ctx, "prod-1").Return(0, avitoErr)

	// No ReleaseQueueSlot expectation: calling it here would be the bug.
	_, _, err := s.srv.JoinQueue(s.ctx, "prod-1", "user-1", 1)

	require.ErrorIs(s.T(), err, avitoErr)
}

// A completed join keeps its slot: the membership now justifies it, and only a
// terminal transition may give it back.
func (s *QueueServiceTestSuite) TestJoinQueue_KeepsSlotOnSuccess() {
	s.mockJoinQueueBase(10, 1, 0, 0, false, nil)
	s.mockDurableUpsert(models.MembershipStatusQueued, nil)
	s.mockCache.EXPECT().Enqueue(s.ctx, "prod-1", "user-1").Return(nil)
	s.mockSyncCacheState(models.MembershipStatusQueued, false, false)

	mem, _, err := s.srv.JoinQueue(s.ctx, "prod-1", "user-1", 1)

	require.NoError(s.T(), err)
	assert.Equal(s.T(), models.MembershipStatusQueued, mem.Status)
}

// A configured limit reaches the refusal, so the number the client is shown is
// the number actually enforced.
func (s *QueueServiceTestSuite) TestJoinQueue_HonoursConfiguredLimit() {
	srv := s.newServiceWithQueueLimit(2)

	s.mockCache.EXPECT().GetMembership(s.ctx, "prod-1", "user-1").
		Return(nil, models.ErrTokenNotFound).Times(2)
	s.expectMembershipClaim("prod-1", "user-1")
	s.mockCache.EXPECT().TryOccupyQueueSlot(s.ctx, "user-1", "prod-1", 2).
		Return(false, false, nil)

	_, _, err := srv.JoinQueue(s.ctx, "prod-1", "user-1", 1)

	var limitErr *models.QueueLimitError
	require.ErrorAs(s.T(), err, &limitErr)
	assert.Equal(s.T(), 2, limitErr.Limit)
}

// Terminal statuses free the slot; the rest hold it. This is the whole rule the
// cache layer relies on, so it is asserted directly rather than through Redis.
func TestMembershipStatusIsTerminal(t *testing.T) {
	holds := []models.MembershipStatus{
		models.MembershipStatusQueued,
		models.MembershipStatusRightActive,
		models.MembershipStatusOfferPending,
	}
	frees := []models.MembershipStatus{
		models.MembershipStatusDeclined,
		models.MembershipStatusPurchased,
		models.MembershipStatusSoldOut,
	}

	for _, status := range holds {
		assert.False(t, status.IsTerminal(), "%s must keep holding its slot", status)
	}
	for _, status := range frees {
		assert.True(t, status.IsTerminal(), "%s must free its slot", status)
	}
}
