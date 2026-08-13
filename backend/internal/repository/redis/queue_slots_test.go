package redis_test

import (
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"backend/internal/models"

	"github.com/stretchr/testify/require"
)

func (s *CacheTestSuite) TestTryOccupyQueueSlot_GrantsUpToTheLimit() {
	for i, productID := range []string{"p-1", "p-2", "p-3"} {
		granted, fresh, err := s.repo.TryOccupyQueueSlot(s.ctx, "limit-user", productID, 3)
		require.NoError(s.T(), err)
		require.True(s.T(), granted, "slot %d must be granted below the limit", i+1)
		require.True(s.T(), fresh, "a queue not yet occupied must yield a fresh slot")
	}

	granted, fresh, err := s.repo.TryOccupyQueueSlot(s.ctx, "limit-user", "p-4", 3)
	require.NoError(s.T(), err)
	require.False(s.T(), granted, "the fourth queue must be refused at a limit of three")
	require.False(s.T(), fresh)

	count, err := s.repo.CountQueueSlots(s.ctx, "limit-user")
	require.NoError(s.T(), err)
	require.Equal(s.T(), 3, count, "a refused join must not consume a slot")
}

// Re-entering a queue the user already occupies costs nothing. Without this a
// double click, a retry or a reconnect would eat slots until the user locked
// themselves out of queues they are already in.
func (s *CacheTestSuite) TestTryOccupyQueueSlot_ReentryIsFree() {
	granted, fresh, err := s.repo.TryOccupyQueueSlot(s.ctx, "reentry-user", "p-1", 1)
	require.NoError(s.T(), err)
	require.True(s.T(), granted)
	require.True(s.T(), fresh)

	// The limit is already spent, yet the same queue must still be admitted.
	granted, fresh, err = s.repo.TryOccupyQueueSlot(s.ctx, "reentry-user", "p-1", 1)
	require.NoError(s.T(), err)
	require.True(s.T(), granted, "re-entering an occupied queue must be admitted")
	require.False(s.T(), fresh, "re-entry must not report a fresh slot")

	count, err := s.repo.CountQueueSlots(s.ctx, "reentry-user")
	require.NoError(s.T(), err)
	require.Equal(s.T(), 1, count)
}

// The check and the insert are one atomic step, so parallel joins into distinct
// products cannot all read a count below the limit and all pass.
func (s *CacheTestSuite) TestTryOccupyQueueSlot_ParallelJoinsRespectTheLimit() {
	const (
		attempts = 20
		limit    = 5
	)

	var (
		wg     sync.WaitGroup
		grants atomic.Int32
		start  = make(chan struct{})
		userID = "race-user"
	)

	for i := 0; i < attempts; i++ {
		wg.Add(1)

		go func(i int) {
			defer wg.Done()
			<-start

			productID := "race-p-" + strconv.Itoa(i)

			granted, _, err := s.repo.TryOccupyQueueSlot(s.ctx, userID, productID, limit)
			require.NoError(s.T(), err)
			if granted {
				grants.Add(1)
			}
		}(i)
	}

	close(start)
	wg.Wait()

	require.Equal(s.T(), int32(limit), grants.Load(),
		"exactly the limit must be granted, no matter how many requests raced")

	count, err := s.repo.CountQueueSlots(s.ctx, userID)
	require.NoError(s.T(), err)
	require.Equal(s.T(), limit, count)
}

func (s *CacheTestSuite) TestReleaseQueueSlot_FreesTheSlot() {
	_, _, err := s.repo.TryOccupyQueueSlot(s.ctx, "release-user", "p-1", 1)
	require.NoError(s.T(), err)

	require.NoError(s.T(), s.repo.ReleaseQueueSlot(s.ctx, "release-user", "p-1"))

	granted, fresh, err := s.repo.TryOccupyQueueSlot(s.ctx, "release-user", "p-2", 1)
	require.NoError(s.T(), err)
	require.True(s.T(), granted, "a released slot must be reusable")
	require.True(s.T(), fresh)
}

// SetMembership is the single place every transition passes through, so the slot
// set follows the state machine without any caller having to remember it.
func (s *CacheTestSuite) TestSetMembership_TerminalStatusFreesTheSlot() {
	now := time.Now().UTC()
	membership := &models.QueueMembership{
		ProductID: "p-term",
		UserID:    "term-user",
		Status:    models.MembershipStatusQueued,
		Quantity:  1,
		CreatedAt: now,
		UpdatedAt: now,
	}

	require.NoError(s.T(), s.repo.SetMembership(s.ctx, membership))

	count, err := s.repo.CountQueueSlots(s.ctx, "term-user")
	require.NoError(s.T(), err)
	require.Equal(s.T(), 1, count, "a live membership must hold a slot")

	membership.Status = models.MembershipStatusDeclined
	require.NoError(s.T(), s.repo.SetMembership(s.ctx, membership))

	count, err = s.repo.CountQueueSlots(s.ctx, "term-user")
	require.NoError(s.T(), err)
	require.Equal(s.T(), 0, count, "a terminal membership must release its slot")
}

// The paid-webhook path updates the membership inside Lua and never reaches
// SetMembership, so it has to release the slot on its own.
func (s *CacheTestSuite) TestMarkPurchasedIfCurrentToken_FreesTheSlot() {
	now := time.Now().UTC()
	token := "paid-token"
	membership := &models.QueueMembership{
		ProductID:    "p-paid",
		UserID:       "paid-user",
		Status:       models.MembershipStatusRightActive,
		Quantity:     1,
		CurrentToken: &token,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	require.NoError(s.T(), s.repo.SetMembership(s.ctx, membership))

	count, err := s.repo.CountQueueSlots(s.ctx, "paid-user")
	require.NoError(s.T(), err)
	require.Equal(s.T(), 1, count)

	applied, err := s.repo.MarkPurchasedIfCurrentToken(s.ctx, &models.Right{
		Token:     token,
		UserID:    "paid-user",
		ProductID: "p-paid",
		Quantity:  1,
		Status:    models.RightStatusUsed,
	}, now)
	require.NoError(s.T(), err)
	require.True(s.T(), applied)

	count, err = s.repo.CountQueueSlots(s.ctx, "paid-user")
	require.NoError(s.T(), err)
	require.Equal(s.T(), 0, count, "a purchase must free the slot it held")
}

// Recovery rebuilds slot sets from PostgreSQL, so whatever a crash left behind
// has to be dropped first.
func (s *CacheTestSuite) TestResetQueueSlots_ClearsEveryUser() {
	for _, userID := range []string{"reset-a", "reset-b"} {
		_, _, err := s.repo.TryOccupyQueueSlot(s.ctx, userID, "p-1", 5)
		require.NoError(s.T(), err)
	}

	require.NoError(s.T(), s.repo.ResetQueueSlots(s.ctx))

	for _, userID := range []string{"reset-a", "reset-b"} {
		count, err := s.repo.CountQueueSlots(s.ctx, userID)
		require.NoError(s.T(), err)
		require.Zero(s.T(), count, "%s must have no slots after a reset", userID)
	}
}
