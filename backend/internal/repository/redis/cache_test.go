// Package redis_test provides integration tests for the Redis repository.
// It uses testcontainers to spin up a Redis 7 instance.
package redis_test

import (
	"context"
	"encoding/json"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"backend/internal/models"
	repository "backend/internal/repository/redis"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// CacheTestSuite manages the test lifecycle and Redis container dependencies.
type CacheTestSuite struct {
	suite.Suite
	ctx       context.Context
	container testcontainers.Container
	client    *redis.Client
	repo      *repository.CacheRepo
}

// SetupSuite starts the Redis container matching the production configuration.
func (s *CacheTestSuite) SetupSuite() {
	s.ctx = context.Background()

	req := testcontainers.ContainerRequest{
		Image:        "redis:7-alpine",
		ExposedPorts: []string{"6379/tcp"},
		Cmd:          []string{"redis-server", "--appendonly", "yes"},
		WaitingFor: wait.ForAll(
			wait.ForLog("Ready to accept connections"),
			wait.ForListeningPort("6379/tcp"),
		),
	}

	container, err := testcontainers.GenericContainer(s.ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	require.NoError(s.T(), err, "failed to start redis container")
	s.container = container

	endpoint, err := container.Endpoint(s.ctx, "")
	require.NoError(s.T(), err)

	s.client = redis.NewClient(&redis.Options{
		Addr: endpoint,
	})

	err = s.client.Ping(s.ctx).Err()
	require.NoError(s.T(), err, "failed to ping redis")

	s.repo = repository.NewCacheRepo(s.client)
}

// TearDownSuite terminates the container and closes the client.
func (s *CacheTestSuite) TearDownSuite() {
	if s.client != nil {
		_ = s.client.Close()
	}
	if s.container != nil {
		require.NoError(s.T(), s.container.Terminate(s.ctx))
	}
}

// SetupTest cleans the Redis database before each test to ensure complete isolation.
func (s *CacheTestSuite) SetupTest() {
	err := s.client.FlushDB(s.ctx).Err()
	require.NoError(s.T(), err)
}

func (s *CacheTestSuite) TestInitStock_Idempotency() {
	err := s.repo.InitStock(s.ctx, "prod-1", 10)
	require.NoError(s.T(), err)

	err = s.repo.InitStock(s.ctx, "prod-1", 50)
	require.NoError(s.T(), err)

	res, err := s.client.HGetAll(s.ctx, "stock:prod-1").Result()
	require.NoError(s.T(), err)
	require.Equal(s.T(), "10", res["product_count"])
	require.Equal(s.T(), "10", res["available_units"])
}

func (s *CacheTestSuite) TestTryAllocate_Branches() {
	err := s.repo.InitStock(s.ctx, "prod-2", 5)
	require.NoError(s.T(), err)

	alloc, avail, soldOut, err := s.repo.TryAllocate(s.ctx, "prod-2", 3)
	require.NoError(s.T(), err)
	require.Equal(s.T(), 3, alloc)
	require.Equal(s.T(), 0, avail)
	require.False(s.T(), soldOut)

	alloc, avail, soldOut, err = s.repo.TryAllocate(s.ctx, "prod-2", 4)
	require.NoError(s.T(), err)
	require.Equal(s.T(), 0, alloc)
	require.Equal(s.T(), 2, avail)
	require.False(s.T(), soldOut)

	alloc, avail, soldOut, err = s.repo.TryAllocate(s.ctx, "prod-2", 2)
	require.NoError(s.T(), err)
	require.Equal(s.T(), 0, alloc)
	require.Equal(s.T(), 0, avail)
	require.False(s.T(), soldOut)

	alloc, avail, soldOut, err = s.repo.TryAllocate(s.ctx, "prod-2", 1)
	require.NoError(s.T(), err)
	require.Equal(s.T(), 0, alloc)
	require.Equal(s.T(), 0, avail)
	require.False(s.T(), soldOut)

	err = s.repo.CommitPurchase(s.ctx, "prod-2", 5)
	require.NoError(s.T(), err)

	alloc, avail, soldOut, err = s.repo.TryAllocate(s.ctx, "prod-2", 1)
	require.NoError(s.T(), err)
	require.Equal(s.T(), 0, alloc)
	require.Equal(s.T(), 0, avail)
	require.True(s.T(), soldOut)
}

func (s *CacheTestSuite) TestTryAllocate_PartialOfferReservesAvailableUnits() {
	err := s.repo.InitStock(s.ctx, "prod-partial-reservation", 5)
	require.NoError(s.T(), err)

	alloc, avail, soldOut, err := s.repo.TryAllocate(s.ctx, "prod-partial-reservation", 7)
	require.NoError(s.T(), err)
	require.Equal(s.T(), 0, alloc)
	require.Equal(s.T(), 5, avail)
	require.False(s.T(), soldOut)

	stock, err := s.client.HGetAll(s.ctx, "stock:prod-partial-reservation").Result()
	require.NoError(s.T(), err)
	require.Equal(s.T(), "5", stock["product_count"])
	require.Equal(s.T(), "0", stock["available_units"])

	alloc, avail, soldOut, err = s.repo.TryAllocate(s.ctx, "prod-partial-reservation", 1)
	require.NoError(s.T(), err)
	require.Equal(s.T(), 0, alloc)
	require.Equal(s.T(), 0, avail)
	require.False(s.T(), soldOut)
}

func (s *CacheTestSuite) TestTryAllocate_RaceCondition() {
	err := s.repo.InitStock(s.ctx, "prod-race", 5)
	require.NoError(s.T(), err)

	var wg sync.WaitGroup
	var successCount int32
	var queuedCount int32

	workers := 100
	wg.Add(workers)

	start := make(chan struct{})

	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			<-start

			alloc, _, soldOut, errAlloc := s.repo.TryAllocate(s.ctx, "prod-race", 1)
			if errAlloc != nil {
				return
			}
			if alloc == 1 && !soldOut {
				atomic.AddInt32(&successCount, 1)
			} else if alloc == 0 && !soldOut {
				atomic.AddInt32(&queuedCount, 1)
			}
		}()
	}

	close(start)
	wg.Wait()

	require.Equal(s.T(), int32(5), successCount)
	require.Equal(s.T(), int32(95), queuedCount)

	res, err := s.client.HGetAll(s.ctx, "stock:prod-race").Result()
	require.NoError(s.T(), err)
	require.Equal(s.T(), "5", res["product_count"])
	require.Equal(s.T(), "0", res["available_units"])
}

func (s *CacheTestSuite) TestTryAllocate_PartialOfferRaceCondition() {
	err := s.repo.InitStock(s.ctx, "prod-partial-race", 5)
	require.NoError(s.T(), err)

	const workers = 50

	var wg sync.WaitGroup
	var partialOfferCount int32
	var queuedCount int32
	var errorCount int32

	wg.Add(workers)
	start := make(chan struct{})

	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			<-start

			alloc, avail, soldOut, errAlloc := s.repo.TryAllocate(s.ctx, "prod-partial-race", 10)
			if errAlloc != nil {
				atomic.AddInt32(&errorCount, 1)
				return
			}

			switch {
			case alloc == 0 && avail == 5 && !soldOut:
				atomic.AddInt32(&partialOfferCount, 1)
			case alloc == 0 && avail == 0 && !soldOut:
				atomic.AddInt32(&queuedCount, 1)
			default:
				atomic.AddInt32(&errorCount, 1)
			}
		}()
	}

	close(start)
	wg.Wait()

	require.Equal(s.T(), int32(1), partialOfferCount)
	require.Equal(s.T(), int32(workers-1), queuedCount)
	require.Equal(s.T(), int32(0), errorCount)

	stock, err := s.client.HGetAll(s.ctx, "stock:prod-partial-race").Result()
	require.NoError(s.T(), err)
	require.Equal(s.T(), "5", stock["product_count"])
	require.Equal(s.T(), "0", stock["available_units"])
}

func (s *CacheTestSuite) TestQueue_FIFOBehavior() {
	err := s.repo.Enqueue(s.ctx, "prod-q", "user-1")
	require.NoError(s.T(), err)

	err = s.repo.Enqueue(s.ctx, "prod-q", "user-2")
	require.NoError(s.T(), err)

	err = s.repo.Enqueue(s.ctx, "prod-q", "user-3")
	require.NoError(s.T(), err)

	members, err := s.client.ZRangeWithScores(s.ctx, "queue:prod-q", 0, -1).Result()
	require.NoError(s.T(), err)
	require.Len(s.T(), members, 3)

	require.Equal(s.T(), "user-1", members[0].Member)
	require.Equal(s.T(), float64(1), members[0].Score)
	require.Equal(s.T(), "user-2", members[1].Member)
	require.Equal(s.T(), float64(2), members[1].Score)
	require.Equal(s.T(), "user-3", members[2].Member)
	require.Equal(s.T(), float64(3), members[2].Score)

	err = s.repo.RemoveFromQueue(s.ctx, "prod-q", "user-2")
	require.NoError(s.T(), err)

	members, err = s.client.ZRangeWithScores(s.ctx, "queue:prod-q", 0, -1).Result()
	require.NoError(s.T(), err)
	require.Len(s.T(), members, 2)
	require.Equal(s.T(), "user-1", members[0].Member)
	require.Equal(s.T(), "user-3", members[1].Member)
}

func (s *CacheTestSuite) TestMembership_SetAndGet_WithNils() {
	now := time.Now().UTC().Truncate(time.Millisecond)

	mem := &models.QueueMembership{
		ProductID: "prod-m",
		UserID:    "user-1",
		Status:    models.MembershipStatusQueued,
		Quantity:  2,
		CreatedAt: now,
		UpdatedAt: now,
	}

	err := s.repo.SetMembership(s.ctx, mem)
	require.NoError(s.T(), err)

	fetched, err := s.repo.GetMembership(s.ctx, "prod-m", "user-1")
	require.NoError(s.T(), err)

	require.Equal(s.T(), mem.ProductID, fetched.ProductID)
	require.Equal(s.T(), mem.Quantity, fetched.Quantity)
	require.Nil(s.T(), fetched.AvailableQuantity)
	require.Nil(s.T(), fetched.CurrentToken)
	require.Nil(s.T(), fetched.ExpiresAt)
	require.True(s.T(), mem.CreatedAt.Equal(fetched.CreatedAt))

	_, err = s.repo.GetMembership(s.ctx, "unknown", "unknown")
	require.ErrorIs(s.T(), err, models.ErrTokenNotFound)
}

func (s *CacheTestSuite) TestMarkPurchasedIfCurrentToken_MatchingTokenUpdatesMembershipAndTimer() {
	now := time.Now().UTC().Truncate(time.Millisecond)
	token := "tok-paid"
	expiresAt := now.Add(time.Minute)
	available := 1

	err := s.repo.SetMembership(s.ctx, &models.QueueMembership{
		ProductID:         "prod-paid",
		UserID:            "user-paid",
		Status:            models.MembershipStatusRightActive,
		Quantity:          1,
		AvailableQuantity: &available,
		CurrentToken:      &token,
		ExpiresAt:         &expiresAt,
		CreatedAt:         now,
		UpdatedAt:         now,
	})
	require.NoError(s.T(), err)
	err = s.repo.AddToExpiryTimer(s.ctx, "prod-paid", "user-paid", expiresAt)
	require.NoError(s.T(), err)

	pubsub := s.client.Subscribe(s.ctx, "updates:prod-paid:user-paid")
	defer func() {
		require.NoError(s.T(), pubsub.Close())
	}()
	_, err = pubsub.Receive(s.ctx)
	require.NoError(s.T(), err)

	applied, err := s.repo.MarkPurchasedIfCurrentToken(s.ctx, &models.Right{
		Token:     token,
		ProductID: "prod-paid",
		UserID:    "user-paid",
	}, now.Add(time.Second))
	require.NoError(s.T(), err)
	require.True(s.T(), applied)

	fetched, err := s.repo.GetMembership(s.ctx, "prod-paid", "user-paid")
	require.NoError(s.T(), err)
	require.Equal(s.T(), models.MembershipStatusPurchased, fetched.Status)
	require.Nil(s.T(), fetched.AvailableQuantity)
	require.Nil(s.T(), fetched.CurrentToken)
	require.Nil(s.T(), fetched.ExpiresAt)

	_, err = s.client.ZScore(s.ctx, "expiring:rights", "prod-paid:user-paid").Result()
	require.ErrorIs(s.T(), err, redis.Nil)

	select {
	case msg := <-pubsub.Channel():
		var payload map[string]string
		err = json.Unmarshal([]byte(msg.Payload), &payload)
		require.NoError(s.T(), err)
		require.Equal(s.T(), "PURCHASED", payload["status"])
	case <-time.After(2 * time.Second):
		s.T().Fatal("timeout waiting for purchase event")
	}
}

func (s *CacheTestSuite) TestMarkPurchasedIfCurrentToken_StaleTokenLeavesMembershipAndTimer() {
	now := time.Now().UTC().Truncate(time.Millisecond)
	newToken := "new-token"
	expiresAt := now.Add(time.Minute)

	err := s.repo.SetMembership(s.ctx, &models.QueueMembership{
		ProductID:    "prod-stale",
		UserID:       "user-stale",
		Status:       models.MembershipStatusRightActive,
		Quantity:     1,
		CurrentToken: &newToken,
		ExpiresAt:    &expiresAt,
		CreatedAt:    now,
		UpdatedAt:    now,
	})
	require.NoError(s.T(), err)
	err = s.repo.AddToExpiryTimer(s.ctx, "prod-stale", "user-stale", expiresAt)
	require.NoError(s.T(), err)

	applied, err := s.repo.MarkPurchasedIfCurrentToken(s.ctx, &models.Right{
		Token:     "old-token",
		ProductID: "prod-stale",
		UserID:    "user-stale",
	}, now.Add(time.Second))
	require.NoError(s.T(), err)
	require.False(s.T(), applied)

	fetched, err := s.repo.GetMembership(s.ctx, "prod-stale", "user-stale")
	require.NoError(s.T(), err)
	require.Equal(s.T(), models.MembershipStatusRightActive, fetched.Status)
	require.Equal(s.T(), newToken, *fetched.CurrentToken)
	require.NotNil(s.T(), fetched.ExpiresAt)

	score, err := s.client.ZScore(s.ctx, "expiring:rights", "prod-stale:user-stale").Result()
	require.NoError(s.T(), err)
	require.Equal(s.T(), float64(expiresAt.Unix()), score)
}

func (s *CacheTestSuite) TestMarkPurchasedIfCurrentToken_MissingMembershipReturnsFalse() {
	applied, err := s.repo.MarkPurchasedIfCurrentToken(s.ctx, &models.Right{
		Token:     "ghost-token",
		ProductID: "ghost-product",
		UserID:    "ghost-user",
	}, time.Now().UTC())
	require.NoError(s.T(), err)
	require.False(s.T(), applied)
}

func (s *CacheTestSuite) TestRight_SetAndGet() {
	now := time.Now().UTC().Truncate(time.Millisecond)

	right := &models.Right{
		Token:     "tok-123",
		UserID:    "user-1",
		ProductID: "prod-r",
		Quantity:  1,
		Status:    models.RightStatusActive,
		CreatedAt: now,
		ExpiresAt: now.Add(time.Minute),
	}

	err := s.repo.SetRight(s.ctx, right)
	require.NoError(s.T(), err)

	fetched, err := s.repo.GetRight(s.ctx, "tok-123")
	require.NoError(s.T(), err)
	require.Equal(s.T(), right.Token, fetched.Token)
	require.Equal(s.T(), right.Quantity, fetched.Quantity)
	require.Nil(s.T(), fetched.OrderID)
	require.Nil(s.T(), fetched.UsedAt)
	require.True(s.T(), right.CreatedAt.Equal(fetched.CreatedAt))

	_, err = s.repo.GetRight(s.ctx, "ghost-token")
	require.ErrorIs(s.T(), err, models.ErrTokenNotFound)
}

func (s *CacheTestSuite) TestPubSub() {
	pubsub := s.client.Subscribe(s.ctx, "updates:prod-ps:user-ps")
	defer func() {
		_ = pubsub.Close()
	}()

	_, err := pubsub.Receive(s.ctx)
	require.NoError(s.T(), err)

	payload := map[string]string{"status": "OFFER_PENDING"}
	err = s.repo.PublishEvent(s.ctx, "prod-ps", "user-ps", payload)
	require.NoError(s.T(), err)

	msgChan := pubsub.Channel()

	select {
	case msg := <-msgChan:
		var received map[string]string
		err = json.Unmarshal([]byte(msg.Payload), &received)
		require.NoError(s.T(), err)
		require.Equal(s.T(), "OFFER_PENDING", received["status"])
	case <-time.After(2 * time.Second):
		s.T().Fatal("timeout waiting for pubsub message")
	}
}

func (s *CacheTestSuite) TestSubscribeUpdates_UserAndQueueChannels() {
	events, closeSubscription, err := s.repo.SubscribeUpdates(s.ctx, "prod-live", "user-live")
	require.NoError(s.T(), err)

	err = s.repo.PublishEvent(s.ctx, "prod-live", "another-user", map[string]string{"status": "QUEUED"})
	require.NoError(s.T(), err)

	select {
	case <-events:
		s.T().Fatal("received another user's event")
	case <-time.After(100 * time.Millisecond):
	}

	err = s.repo.PublishEvent(s.ctx, "prod-live", "user-live", map[string]string{"status": "RIGHT_ACTIVE"})
	require.NoError(s.T(), err)

	select {
	case <-events:
	case <-time.After(2 * time.Second):
		s.T().Fatal("timeout waiting for user update")
	}

	err = s.repo.Enqueue(s.ctx, "prod-live", "queued-user")
	require.NoError(s.T(), err)

	select {
	case <-events:
	case <-time.After(2 * time.Second):
		s.T().Fatal("timeout waiting for product queue update")
	}

	err = s.repo.Enqueue(s.ctx, "another-product", "queued-user")
	require.NoError(s.T(), err)

	select {
	case <-events:
		s.T().Fatal("received another product's event")
	case <-time.After(100 * time.Millisecond):
	}

	require.NoError(s.T(), closeSubscription())
	require.NoError(s.T(), closeSubscription(), "subscription close must be idempotent")

	select {
	case _, ok := <-events:
		require.False(s.T(), ok, "events channel must close with the subscription")
	case <-time.After(2 * time.Second):
		s.T().Fatal("timeout waiting for events channel to close")
	}
}

func (s *CacheTestSuite) TestRestoreAvailableUnits_PublishesQueueUpdate() {
	events, closeSubscription, err := s.repo.SubscribeUpdates(s.ctx, "prod-stock-live", "user-live")
	require.NoError(s.T(), err)
	defer func() {
		require.NoError(s.T(), closeSubscription())
	}()

	err = s.client.HSet(s.ctx, "stock:prod-stock-live", "available_units", 0, "product_count", 5).Err()
	require.NoError(s.T(), err)

	err = s.repo.RestoreAvailableUnits(s.ctx, "prod-stock-live", 2)
	require.NoError(s.T(), err)

	select {
	case <-events:
	case <-time.After(2 * time.Second):
		s.T().Fatal("timeout waiting for stock update")
	}

	available, err := s.client.HGet(s.ctx, "stock:prod-stock-live", "available_units").Int()
	require.NoError(s.T(), err)
	require.Equal(s.T(), 2, available)
}

func (s *CacheTestSuite) TestExpiryTimers() {
	expiryTime := time.Now().UTC().Add(time.Hour)

	err := s.repo.AddToExpiryTimer(s.ctx, "prod-t", "user-1", expiryTime)
	require.NoError(s.T(), err)
	err = s.repo.AddToExpiryTimer(s.ctx, "prod-t", "user-2", expiryTime.Add(time.Minute))
	require.NoError(s.T(), err)

	members, err := s.client.ZRangeWithScores(s.ctx, "expiring:rights", 0, -1).Result()
	require.NoError(s.T(), err)
	require.Len(s.T(), members, 2)
	require.Equal(s.T(), "prod-t:user-1", members[0].Member)
	require.Equal(s.T(), float64(expiryTime.Unix()), members[0].Score)

	err = s.repo.RemoveFromExpiryTimer(s.ctx, "prod-t", "user-1")
	require.NoError(s.T(), err)

	members, err = s.client.ZRangeWithScores(s.ctx, "expiring:rights", 0, -1).Result()
	require.NoError(s.T(), err)
	require.Len(s.T(), members, 1)
	require.Equal(s.T(), "prod-t:user-2", members[0].Member)
}

// TestCacheTestSuite acts as the entry point for 'go test'
func TestCacheTestSuite(t *testing.T) {
	suite.Run(t, new(CacheTestSuite))
}

func (s *CacheTestSuite) TestRestoreAvailableUnits() {
	err := s.repo.InitStock(s.ctx, "prod-restore", 10)
	require.NoError(s.T(), err)

	alloc, _, _, err := s.repo.TryAllocate(s.ctx, "prod-restore", 3)
	require.NoError(s.T(), err)
	require.Equal(s.T(), 3, alloc)

	err = s.repo.RestoreAvailableUnits(s.ctx, "prod-restore", 3)
	require.NoError(s.T(), err)

	res, err := s.client.HGetAll(s.ctx, "stock:prod-restore").Result()
	require.NoError(s.T(), err)
	require.Equal(s.T(), "10", res["available_units"])
	require.Equal(s.T(), "10", res["product_count"])
}

func (s *CacheTestSuite) TestRestoreAvailableUnits_AfterPartialOffer() {
	err := s.repo.InitStock(s.ctx, "prod-partial-restore", 5)
	require.NoError(s.T(), err)

	alloc, avail, soldOut, err := s.repo.TryAllocate(s.ctx, "prod-partial-restore", 7)
	require.NoError(s.T(), err)
	require.Equal(s.T(), 0, alloc)
	require.Equal(s.T(), 5, avail)
	require.False(s.T(), soldOut)

	err = s.repo.RestoreAvailableUnits(s.ctx, "prod-partial-restore", avail)
	require.NoError(s.T(), err)

	stock, err := s.client.HGetAll(s.ctx, "stock:prod-partial-restore").Result()
	require.NoError(s.T(), err)
	require.Equal(s.T(), "5", stock["product_count"])
	require.Equal(s.T(), "5", stock["available_units"])

	alloc, avail, soldOut, err = s.repo.TryAllocate(s.ctx, "prod-partial-restore", 5)
	require.NoError(s.T(), err)
	require.Equal(s.T(), 5, alloc)
	require.Equal(s.T(), 0, avail)
	require.False(s.T(), soldOut)
}

func (s *CacheTestSuite) TestGetFirstInQueue() {
	_, err := s.repo.GetFirstInQueue(s.ctx, "prod-empty")
	require.ErrorIs(s.T(), err, models.ErrTokenNotFound)

	err = s.repo.Enqueue(s.ctx, "prod-first", "user-1")
	require.NoError(s.T(), err)

	err = s.repo.Enqueue(s.ctx, "prod-first", "user-2")
	require.NoError(s.T(), err)

	first, err := s.repo.GetFirstInQueue(s.ctx, "prod-first")
	require.NoError(s.T(), err)
	require.Equal(s.T(), "user-1", first)

	count, err := s.client.ZCard(s.ctx, "queue:prod-first").Result()
	require.NoError(s.T(), err)
	require.Equal(s.T(), int64(2), count)
}

func (s *CacheTestSuite) TestClaimExpired_TakesOnlyDueItems() {
	now := time.Now().UTC()

	require.NoError(s.T(), s.repo.AddToExpiryTimer(s.ctx, "prod-exp", "user-old1", now.Add(-2*time.Hour)))
	require.NoError(s.T(), s.repo.AddToExpiryTimer(s.ctx, "prod-exp", "user-old2", now.Add(-time.Hour)))
	require.NoError(s.T(), s.repo.AddToExpiryTimer(s.ctx, "prod-exp", "user-future", now.Add(time.Hour)))

	claimed, err := s.repo.ClaimExpired(s.ctx, now, time.Minute, 10)
	require.NoError(s.T(), err)
	require.Len(s.T(), claimed, 2)
	require.ElementsMatch(s.T(), []string{"prod-exp:user-old1", "prod-exp:user-old2"}, expiryClaimKeys(claimed))

	// The claimed items left the schedule; the one still in the future stays.
	count, err := s.client.ZCard(s.ctx, "expiring:rights").Result()
	require.NoError(s.T(), err)
	require.Equal(s.T(), int64(1), count)
}

func (s *CacheTestSuite) TestPopAndAllocate_EmptyQueue() {
	uid, _, _, _, _, _, err := s.repo.PopAndAllocate(s.ctx, "prod-empty")
	require.NoError(s.T(), err)
	require.Empty(s.T(), uid)
}

// TestPopAndAllocate_GhostUser verifies that a user in the ZSET queue without a corresponding
// active membership in the HASH is identified as a "GHOST" and atomically removed from the queue.
func (s *CacheTestSuite) TestPopAndAllocate_GhostUser() {
	err := s.repo.Enqueue(s.ctx, "prod-ghost", "user-ghost")
	require.NoError(s.T(), err)

	uid, alloc, avail, soldOut, status, score, err := s.repo.PopAndAllocate(s.ctx, "prod-ghost")
	require.NoError(s.T(), err)
	require.Equal(s.T(), "user-ghost", uid)
	require.Equal(s.T(), 0, alloc)
	require.Equal(s.T(), 0, avail)
	require.False(s.T(), soldOut)
	require.Equal(s.T(), models.MembershipStatus("GHOST"), status)
	require.Greater(s.T(), score, float64(0))

	count, err := s.client.ZCard(s.ctx, "queue:prod-ghost").Result()
	require.NoError(s.T(), err)
	require.Equal(s.T(), int64(0), count)
}

// TestPopAndAllocate_FullAllocation verifies that if there is enough stock, the user receives
// RIGHT_ACTIVE status, the stock is decremented, and the user is removed from the queue.
func (s *CacheTestSuite) TestPopAndAllocate_FullAllocation() {
	err := s.repo.InitStock(s.ctx, "prod-full", 10)
	require.NoError(s.T(), err)
	err = s.repo.Enqueue(s.ctx, "prod-full", "user-full")
	require.NoError(s.T(), err)

	mem := &models.QueueMembership{ProductID: "prod-full", UserID: "user-full", Status: models.MembershipStatusQueued, Quantity: 2}
	err = s.repo.SetMembership(s.ctx, mem)
	require.NoError(s.T(), err)

	uid, alloc, avail, soldOut, status, score, err := s.repo.PopAndAllocate(s.ctx, "prod-full")
	require.NoError(s.T(), err)
	require.Equal(s.T(), "user-full", uid)
	require.Equal(s.T(), 2, alloc)
	require.Equal(s.T(), 0, avail)
	require.False(s.T(), soldOut)
	require.Equal(s.T(), models.MembershipStatusRightActive, status)
	require.Greater(s.T(), score, float64(0))

	count, err := s.client.ZCard(s.ctx, "queue:prod-full").Result()
	require.NoError(s.T(), err)
	require.Equal(s.T(), int64(0), count)
}

// TestPopAndAllocate_PartialAllocation verifies that if the requested quantity exceeds the available stock,
// the user receives OFFER_PENDING status, the available stock is exhausted, and the user is removed from the queue.
func (s *CacheTestSuite) TestPopAndAllocate_PartialAllocation() {
	err := s.repo.InitStock(s.ctx, "prod-part", 2)
	require.NoError(s.T(), err)
	err = s.repo.Enqueue(s.ctx, "prod-part", "user-part")
	require.NoError(s.T(), err)

	mem := &models.QueueMembership{ProductID: "prod-part", UserID: "user-part", Status: models.MembershipStatusQueued, Quantity: 5}
	err = s.repo.SetMembership(s.ctx, mem)
	require.NoError(s.T(), err)

	uid, alloc, avail, soldOut, status, _, err := s.repo.PopAndAllocate(s.ctx, "prod-part")
	require.NoError(s.T(), err)
	require.Equal(s.T(), "user-part", uid)
	require.Equal(s.T(), 0, alloc)
	require.Equal(s.T(), 2, avail)
	require.False(s.T(), soldOut)
	require.Equal(s.T(), models.MembershipStatusOfferPending, status)
}

// TestPopAndAllocate_SoldOut verifies that if the total product count is zero, the user receives
// SOLD_OUT status and is removed from the queue.
func (s *CacheTestSuite) TestPopAndAllocate_SoldOut() {
	err := s.repo.InitStock(s.ctx, "prod-sold", 0)
	require.NoError(s.T(), err)
	err = s.repo.Enqueue(s.ctx, "prod-sold", "user-sold")
	require.NoError(s.T(), err)

	mem := &models.QueueMembership{ProductID: "prod-sold", UserID: "user-sold", Status: models.MembershipStatusQueued, Quantity: 1}
	err = s.repo.SetMembership(s.ctx, mem)
	require.NoError(s.T(), err)

	uid, alloc, avail, soldOut, status, _, err := s.repo.PopAndAllocate(s.ctx, "prod-sold")
	require.NoError(s.T(), err)
	require.Equal(s.T(), "user-sold", uid)
	require.Equal(s.T(), 0, alloc)
	require.Equal(s.T(), 0, avail)
	require.True(s.T(), soldOut)
	require.Equal(s.T(), models.MembershipStatusSoldOut, status)
}

// TestPopAndAllocate_Queued_NoStock verifies that if available stock is zero but total product count is not,
// meaning stock is temporarily held by others, the user remains in the queue with QUEUED status and is not removed.
func (s *CacheTestSuite) TestPopAndAllocate_Queued_NoStock() {
	err := s.repo.InitStock(s.ctx, "prod-q", 1)
	require.NoError(s.T(), err)
	_, _, _, err = s.repo.TryAllocate(s.ctx, "prod-q", 1)
	require.NoError(s.T(), err)

	err = s.repo.Enqueue(s.ctx, "prod-q", "user-q")
	require.NoError(s.T(), err)

	mem := &models.QueueMembership{ProductID: "prod-q", UserID: "user-q", Status: models.MembershipStatusQueued, Quantity: 1}
	err = s.repo.SetMembership(s.ctx, mem)
	require.NoError(s.T(), err)

	uid, alloc, avail, soldOut, status, _, err := s.repo.PopAndAllocate(s.ctx, "prod-q")
	require.NoError(s.T(), err)
	require.Equal(s.T(), "user-q", uid)
	require.Equal(s.T(), 0, alloc)
	require.Equal(s.T(), 0, avail)
	require.False(s.T(), soldOut)
	require.Equal(s.T(), models.MembershipStatusQueued, status)

	count, err := s.client.ZCard(s.ctx, "queue:prod-q").Result()
	require.NoError(s.T(), err)
	require.Equal(s.T(), int64(1), count)
}

// TestRequeue verifies that a user is successfully inserted back into the ZSET queue with the exact specified score.
func (s *CacheTestSuite) TestRequeue() {
	err := s.repo.Requeue(s.ctx, "prod-req", "user-req", 42.5)
	require.NoError(s.T(), err)

	res, err := s.client.ZRangeWithScores(s.ctx, "queue:prod-req", 0, -1).Result()
	require.NoError(s.T(), err)
	require.Len(s.T(), res, 1)
	require.Equal(s.T(), "user-req", res[0].Member)
	require.Equal(s.T(), 42.5, res[0].Score)
}

func (s *CacheTestSuite) TestRestoreProductState_ReplacesStockQueueAndSeq() {
	require.NoError(s.T(), s.client.HSet(s.ctx, "stock:prod-recovery", "product_count", 99, "available_units", 99).Err())
	require.NoError(s.T(), s.repo.Enqueue(s.ctx, "prod-recovery", "stale-user"))

	require.NoError(s.T(), s.repo.RestoreProductState(
		s.ctx,
		"prod-recovery",
		5,
		2,
		[]string{"user-1", "user-2"},
	))

	stock, err := s.client.HGetAll(s.ctx, "stock:prod-recovery").Result()
	require.NoError(s.T(), err)
	require.Equal(s.T(), "5", stock["product_count"])
	require.Equal(s.T(), "2", stock["available_units"])

	queue, err := s.client.ZRangeWithScores(s.ctx, "queue:prod-recovery", 0, -1).Result()
	require.NoError(s.T(), err)
	require.Len(s.T(), queue, 2)
	require.Equal(s.T(), "user-1", queue[0].Member)
	require.Equal(s.T(), float64(1), queue[0].Score)
	require.Equal(s.T(), "user-2", queue[1].Member)
	require.Equal(s.T(), float64(2), queue[1].Score)

	seq, err := s.client.Get(s.ctx, "queue:prod-recovery:seq").Int()
	require.NoError(s.T(), err)
	require.Equal(s.T(), 2, seq)

	require.NoError(s.T(), s.repo.Enqueue(s.ctx, "prod-recovery", "user-3"))
	score, err := s.client.ZScore(s.ctx, "queue:prod-recovery", "user-3").Result()
	require.NoError(s.T(), err)
	require.Equal(s.T(), float64(3), score)
}

func (s *CacheTestSuite) TestResetExpiryTimers_OnlyClearsExpirationIndexes() {
	require.NoError(s.T(), s.repo.AddToExpiryTimer(s.ctx, "prod-expiry", "user-expiry", time.Now().UTC().Add(time.Minute)))
	require.NoError(s.T(), s.client.ZAdd(s.ctx, "expiring:processing", redis.Z{Score: 1, Member: "prod-expiry:user-expiry"}).Err())
	require.NoError(s.T(), s.client.HSet(s.ctx, "expiring:processing-deadlines", "prod-expiry:user-expiry", 1).Err())
	require.NoError(s.T(), s.client.Set(s.ctx, "unrelated:key", "keep", 0).Err())

	require.NoError(s.T(), s.repo.ResetExpiryTimers(s.ctx))

	for _, key := range []string{"expiring:rights", "expiring:processing", "expiring:processing-deadlines"} {
		exists, err := s.client.Exists(s.ctx, key).Result()
		require.NoError(s.T(), err)
		require.Zero(s.T(), exists)
	}

	value, err := s.client.Get(s.ctx, "unrelated:key").Result()
	require.NoError(s.T(), err)
	require.Equal(s.T(), "keep", value)
}

// TestGetQueueMetrics_Success verifies that the method correctly retrieves
// the user's rank and the available stock when both exist in the database.
func (s *CacheTestSuite) TestGetQueueMetrics_Success() {
	err := s.repo.InitStock(s.ctx, "prod-metrics-1", 10)
	require.NoError(s.T(), err)

	_, _, _, err = s.repo.TryAllocate(s.ctx, "prod-metrics-1", 3)
	require.NoError(s.T(), err)

	err = s.repo.Enqueue(s.ctx, "prod-metrics-1", "user-1")
	require.NoError(s.T(), err)

	err = s.repo.Enqueue(s.ctx, "prod-metrics-1", "user-2")
	require.NoError(s.T(), err)

	rank, avail, err := s.repo.GetQueueMetrics(s.ctx, "prod-metrics-1", "user-2")

	require.NoError(s.T(), err)
	require.Equal(s.T(), 1, rank)
	require.Equal(s.T(), 7, avail)
}

// TestGetQueueMetrics_NotInQueue verifies that querying metrics for a user
// who is not in the queue returns an appropriate domain error.
func (s *CacheTestSuite) TestGetQueueMetrics_NotInQueue() {
	err := s.repo.InitStock(s.ctx, "prod-metrics-2", 10)
	require.NoError(s.T(), err)

	rank, avail, err := s.repo.GetQueueMetrics(s.ctx, "prod-metrics-2", "ghost-user")

	require.ErrorIs(s.T(), err, models.ErrMembershipNotFound)
	require.Equal(s.T(), 0, rank)
	require.Equal(s.T(), 0, avail)
}

// TestGetQueueMetrics_NoStockData verifies that if the stock hash is not initialized,
// the method safely defaults available units to zero without failing.
func (s *CacheTestSuite) TestGetQueueMetrics_NoStockData() {
	err := s.repo.Enqueue(s.ctx, "prod-metrics-3", "user-1")
	require.NoError(s.T(), err)

	rank, avail, err := s.repo.GetQueueMetrics(s.ctx, "prod-metrics-3", "user-1")

	require.NoError(s.T(), err)
	require.Equal(s.T(), 0, rank)
	require.Equal(s.T(), 0, avail)
}

// TestGetQueueMetrics_CorruptedStockData verifies that if the available units field
// contains unparseable string data, it is safely treated as zero.
func (s *CacheTestSuite) TestGetQueueMetrics_CorruptedStockData() {
	err := s.repo.Enqueue(s.ctx, "prod-metrics-4", "user-1")
	require.NoError(s.T(), err)

	err = s.client.HSet(s.ctx, "stock:prod-metrics-4", "available_units", "NaN").Err()
	require.NoError(s.T(), err)

	rank, avail, err := s.repo.GetQueueMetrics(s.ctx, "prod-metrics-4", "user-1")

	require.NoError(s.T(), err)
	require.Equal(s.T(), 0, rank)
	require.Equal(s.T(), 0, avail)
}

// TestGetQueueMetrics_InfrastructureError verifies that a network timeout or context
// cancellation correctly interrupts the pipeline and propagates the failure upwards.
func (s *CacheTestSuite) TestGetQueueMetrics_InfrastructureError() {
	ctx, cancel := context.WithCancel(s.ctx)
	cancel()

	rank, avail, err := s.repo.GetQueueMetrics(ctx, "prod-metrics-5", "user-1")

	require.Error(s.T(), err)
	require.Equal(s.T(), 0, rank)
	require.Equal(s.T(), 0, avail)
}

func (s *CacheTestSuite) TestRefreshExpiryTimer_ExtendsExistingTimerOnlyForward() {
	productID := "prod-heartbeat"
	userID := "user-heartbeat"
	initial := time.Now().UTC().Add(30 * time.Second).Truncate(time.Second)
	later := initial.Add(10 * time.Second)
	earlier := initial.Add(-10 * time.Second)

	err := s.repo.AddToExpiryTimer(s.ctx, productID, userID, initial)
	require.NoError(s.T(), err)

	refreshed, err := s.repo.RefreshExpiryTimer(s.ctx, productID, userID, later)
	require.NoError(s.T(), err)
	require.True(s.T(), refreshed)

	member := productID + ":" + userID
	score, err := s.client.ZScore(s.ctx, "expiring:rights", member).Result()
	require.NoError(s.T(), err)
	require.Equal(s.T(), float64(later.Unix()), score)

	refreshed, err = s.repo.RefreshExpiryTimer(s.ctx, productID, userID, earlier)
	require.NoError(s.T(), err)
	require.True(s.T(), refreshed, "the lease still exists even when its score is already newer")

	score, err = s.client.ZScore(s.ctx, "expiring:rights", member).Result()
	require.NoError(s.T(), err)
	require.Equal(s.T(), float64(later.Unix()), score, "a late heartbeat must not shorten the lease")

	expired := time.Now().UTC().Add(-time.Second).Truncate(time.Second)
	err = s.repo.AddToExpiryTimer(s.ctx, productID, userID, expired)
	require.NoError(s.T(), err)
	refreshed, err = s.repo.RefreshExpiryTimer(s.ctx, productID, userID, later.Add(time.Minute))
	require.NoError(s.T(), err)
	require.False(s.T(), refreshed, "an expired lease must not be revived before the worker claims it")

	score, err = s.client.ZScore(s.ctx, "expiring:rights", member).Result()
	require.NoError(s.T(), err)
	require.Equal(s.T(), float64(expired.Unix()), score)

	err = s.repo.RemoveFromExpiryTimer(s.ctx, productID, userID)
	require.NoError(s.T(), err)

	refreshed, err = s.repo.RefreshExpiryTimer(s.ctx, productID, userID, later.Add(time.Minute))
	require.NoError(s.T(), err)
	require.False(s.T(), refreshed, "a worker-claimed lease must not be recreated")

	_, err = s.client.ZScore(s.ctx, "expiring:rights", member).Result()
	require.ErrorIs(s.T(), err, redis.Nil)
}

// TestTryAllocate_RespectsQueue verifies that a newcomer cannot take a freed unit
// while somebody is already waiting for it. Allocation and the queue check happen
// in one atomic step, so no window exists in which the newcomer could win.
func (s *CacheTestSuite) TestTryAllocate_RespectsQueue() {
	ctx := s.ctx
	productID := "queued-product"

	s.Require().NoError(s.repo.InitStock(ctx, productID, 1))

	// Somebody is waiting in line.
	s.Require().NoError(s.repo.Enqueue(ctx, productID, "waiting-user"))

	// A unit is free, but it belongs to the head of the queue.
	s.Require().NoError(s.repo.RestoreAvailableUnits(ctx, productID, 1))

	allocated, available, soldOut, err := s.repo.TryAllocate(ctx, productID, 1)

	s.Require().NoError(err)
	s.Equal(0, allocated, "newcomer must not receive a right")
	s.Equal(0, available, "newcomer must not receive a partial offer either")
	s.False(soldOut)
}

// TestTryAllocate_EmptyQueueAllocates verifies the opposite case: with nobody
// waiting, a newcomer is served immediately and no needless queueing happens.
func (s *CacheTestSuite) TestTryAllocate_EmptyQueueAllocates() {
	ctx := s.ctx
	productID := "free-product"

	s.Require().NoError(s.repo.InitStock(ctx, productID, 2))

	allocated, _, soldOut, err := s.repo.TryAllocate(ctx, productID, 2)

	s.Require().NoError(err)
	s.Equal(2, allocated)
	s.False(soldOut)
}

// TestClaimExpired_LeaseSurvivesCrash verifies the point of the lease: a claimed
// timer that is never acknowledged comes back instead of disappearing. Without
// it, a worker dying mid-pass left the right ACTIVE forever, with its unit out of
// circulation.
func (s *CacheTestSuite) TestClaimExpired_LeaseSurvivesCrash() {
	past := time.Now().UTC().Add(-time.Minute)

	require.NoError(s.T(), s.repo.AddToExpiryTimer(s.ctx, "crash-prod", "crash-user", past))

	claimed, err := s.repo.ClaimExpired(s.ctx, time.Now().UTC(), time.Second, 10)
	require.NoError(s.T(), err)
	require.Equal(s.T(), []string{"crash-prod:crash-user"}, expiryClaimKeys(claimed))

	// While the lease holds, the work is invisible to another worker.
	again, err := s.repo.ClaimExpired(s.ctx, time.Now().UTC(), time.Second, 10)
	require.NoError(s.T(), err)
	require.Empty(s.T(), again, "a claimed item must not be handed out twice")

	// The worker dies without acknowledging. Once the lease runs out, the item
	// returns to the schedule — due as of the moment it was reclaimed.
	afterLease := time.Now().UTC().Add(2 * time.Second)

	rescued, err := s.repo.ReclaimStaleExpired(s.ctx, afterLease)
	require.NoError(s.T(), err)
	require.Equal(s.T(), 1, rescued)

	recovered, err := s.repo.ClaimExpired(s.ctx, afterLease, time.Minute, 10)
	require.NoError(s.T(), err)
	require.Equal(s.T(), []string{"crash-prod:crash-user"}, expiryClaimKeys(recovered), "abandoned work must be picked up again")
	require.Equal(s.T(), past.Truncate(time.Second), recovered[0].Deadline,
		"reclaim must preserve the original timer deadline")
}

// TestAckExpired_DropsWork verifies that acknowledged work is gone for good and
// is not replayed by a later reclaim.
func (s *CacheTestSuite) TestAckExpired_DropsWork() {
	past := time.Now().UTC().Add(-time.Minute)

	require.NoError(s.T(), s.repo.AddToExpiryTimer(s.ctx, "ack-prod", "ack-user", past))

	claimed, err := s.repo.ClaimExpired(s.ctx, time.Now().UTC(), time.Second, 10)
	require.NoError(s.T(), err)
	require.Len(s.T(), claimed, 1)

	require.NoError(s.T(), s.repo.AckExpired(s.ctx, claimed))

	rescued, err := s.repo.ReclaimStaleExpired(s.ctx, time.Now().UTC().Add(time.Hour))
	require.NoError(s.T(), err)
	require.Zero(s.T(), rescued, "acknowledged work must not come back")
}

// TestNackExpired_ReschedulesWork verifies that a failed attempt returns the item
// to the schedule at the requested time, rather than losing it or retrying in a
// tight loop.
func (s *CacheTestSuite) TestNackExpired_ReschedulesWork() {
	past := time.Now().UTC().Add(-time.Minute)

	require.NoError(s.T(), s.repo.AddToExpiryTimer(s.ctx, "nack-prod", "nack-user", past))

	claimed, err := s.repo.ClaimExpired(s.ctx, time.Now().UTC(), time.Minute, 10)
	require.NoError(s.T(), err)
	require.Len(s.T(), claimed, 1)

	retryAt := time.Now().UTC().Add(30 * time.Second)
	require.NoError(s.T(), s.repo.NackExpired(s.ctx, claimed, retryAt))

	tooEarly, err := s.repo.ClaimExpired(s.ctx, time.Now().UTC(), time.Minute, 10)
	require.NoError(s.T(), err)
	require.Empty(s.T(), tooEarly, "a rescheduled item must not be due immediately")

	due, err := s.repo.ClaimExpired(s.ctx, retryAt.Add(time.Second), time.Minute, 10)
	require.NoError(s.T(), err)
	require.Equal(s.T(), []string{"nack-prod:nack-user"}, expiryClaimKeys(due))
}

// TestClaimExpired_RespectsBatchLimit verifies that one pass takes a bounded
// amount of work, so a large backlog cannot be claimed under a single lease that
// would expire halfway through.
func (s *CacheTestSuite) TestClaimExpired_RespectsBatchLimit() {
	past := time.Now().UTC().Add(-time.Minute)

	for _, user := range []string{"u1", "u2", "u3"} {
		require.NoError(s.T(), s.repo.AddToExpiryTimer(s.ctx, "batch-prod", user, past))
	}

	claimed, err := s.repo.ClaimExpired(s.ctx, time.Now().UTC(), time.Minute, 2)

	require.NoError(s.T(), err)
	require.Len(s.T(), claimed, 2)
}

func (s *CacheTestSuite) TestMembershipClaim_ReleaseRequiresCurrentOwner() {
	won, err := s.repo.ClaimMembership(s.ctx, "claim-prod", "claim-user", "owner-a", time.Minute)
	require.NoError(s.T(), err)
	require.True(s.T(), won)

	won, err = s.repo.ClaimMembership(s.ctx, "claim-prod", "claim-user", "owner-b", time.Minute)
	require.NoError(s.T(), err)
	require.False(s.T(), won)

	require.NoError(s.T(), s.repo.ReleaseMembershipClaim(
		s.ctx, "claim-prod", "claim-user", "owner-b",
	))

	won, err = s.repo.ClaimMembership(s.ctx, "claim-prod", "claim-user", "owner-b", time.Minute)
	require.NoError(s.T(), err)
	require.False(s.T(), won, "a non-owner must not release the current owner's claim")

	require.NoError(s.T(), s.repo.ReleaseMembershipClaim(
		s.ctx, "claim-prod", "claim-user", "owner-a",
	))
	won, err = s.repo.ClaimMembership(s.ctx, "claim-prod", "claim-user", "owner-b", time.Minute)
	require.NoError(s.T(), err)
	require.True(s.T(), won)
}

func (s *CacheTestSuite) TestExpirationClaim_StaleWorkerCannotAckOrNackNewLease() {
	base := time.Now().UTC().Truncate(time.Second)
	require.NoError(s.T(), s.repo.AddToExpiryTimer(
		s.ctx, "fence-prod", "fence-user", base.Add(-time.Minute),
	))

	first, err := s.repo.ClaimExpired(s.ctx, base, time.Second, 10)
	require.NoError(s.T(), err)
	require.Len(s.T(), first, 1)

	reclaimedAt := base.Add(2 * time.Second)
	rescued, err := s.repo.ReclaimStaleExpired(s.ctx, reclaimedAt)
	require.NoError(s.T(), err)
	require.Equal(s.T(), 1, rescued)

	second, err := s.repo.ClaimExpired(s.ctx, reclaimedAt, time.Minute, 10)
	require.NoError(s.T(), err)
	require.Len(s.T(), second, 1)

	require.NoError(s.T(), s.repo.AckExpired(s.ctx, first))
	require.NoError(s.T(), s.repo.NackExpired(s.ctx, first, reclaimedAt.Add(time.Minute)))

	score, err := s.client.ZScore(s.ctx, "expiring:processing", second[0].Key).Result()
	require.NoError(s.T(), err)
	require.Equal(s.T(), float64(second[0].LeaseUntil.UnixMilli()), score,
		"a stale worker must not alter a lease owned by a newer worker")

	require.NoError(s.T(), s.repo.AckExpired(s.ctx, second))
}

func (s *CacheTestSuite) TestReclaimStaleExpired_PreservesNewerScheduledTimer() {
	base := time.Now().UTC().Truncate(time.Second)
	oldDeadline := base.Add(-time.Minute)
	newDeadline := base.Add(time.Hour)
	require.NoError(s.T(), s.repo.AddToExpiryTimer(
		s.ctx, "newer-prod", "newer-user", oldDeadline,
	))

	oldClaim, err := s.repo.ClaimExpired(s.ctx, base, time.Second, 10)
	require.NoError(s.T(), err)
	require.Len(s.T(), oldClaim, 1)

	// A new lifecycle schedules its own timer while the old one is processing.
	require.NoError(s.T(), s.repo.AddToExpiryTimer(
		s.ctx, "newer-prod", "newer-user", newDeadline,
	))

	rescued, err := s.repo.ReclaimStaleExpired(s.ctx, base.Add(2*time.Second))
	require.NoError(s.T(), err)
	require.Equal(s.T(), 1, rescued)

	score, err := s.client.ZScore(s.ctx, "expiring:rights", "newer-prod:newer-user").Result()
	require.NoError(s.T(), err)
	require.Equal(s.T(), float64(newDeadline.Unix()), score,
		"reclaiming an old lifecycle must not overwrite its newer timer")
}

func expiryClaimKeys(claims []models.ExpiryClaim) []string {
	keys := make([]string, 0, len(claims))
	for _, claim := range claims {
		keys = append(keys, claim.Key)
	}

	return keys
}
