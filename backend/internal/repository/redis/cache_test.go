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
	require.Equal(s.T(), 2, alloc)
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

func (s *CacheTestSuite) TestGetAndRemoveExpired() {
	now := time.Now().UTC()
	past1 := now.Add(-2 * time.Hour)
	past2 := now.Add(-1 * time.Hour)
	future := now.Add(1 * time.Hour)

	err := s.repo.AddToExpiryTimer(s.ctx, "prod-exp", "user-old1", past1)
	require.NoError(s.T(), err)

	err = s.repo.AddToExpiryTimer(s.ctx, "prod-exp", "user-old2", past2)
	require.NoError(s.T(), err)

	err = s.repo.AddToExpiryTimer(s.ctx, "prod-exp", "user-future", future)
	require.NoError(s.T(), err)

	expired, err := s.repo.GetAndRemoveExpired(s.ctx, now)
	require.NoError(s.T(), err)
	require.Len(s.T(), expired, 2)
	require.Contains(s.T(), expired, "prod-exp:user-old1")
	require.Contains(s.T(), expired, "prod-exp:user-old2")

	count, err := s.client.ZCard(s.ctx, "expiring:rights").Result()
	require.NoError(s.T(), err)
	require.Equal(s.T(), int64(1), count)

	remaining, err := s.client.ZRange(s.ctx, "expiring:rights", 0, -1).Result()
	require.NoError(s.T(), err)
	require.Equal(s.T(), "prod-exp:user-future", remaining[0])
}

// TestPopAndAllocate_EmptyQueue verifies that an empty queue returns an empty user ID without errors.
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
