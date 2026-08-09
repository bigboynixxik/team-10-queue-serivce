// Package redis provides high-speed, concurrency-safe storage operations.
// It acts as the hot-path cache and handles race conditions via atomic Lua scripts.
package redis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"time"

	"backend/internal/models"

	"github.com/redis/go-redis/v9"
)

// ErrInvalidResponse indicates that the Lua script returned an unexpected data format.
var ErrInvalidResponse = errors.New("invalid script response format")

var (
	initStockScript = redis.NewScript(`
		if redis.call('EXISTS', KEYS[1]) == 0 then
			redis.call('HSET', KEYS[1], 'product_count', ARGV[1], 'available_units', ARGV[1])
			redis.call('PUBLISH', KEYS[2], 'changed')
			return 1
		end
		return 0
	`)

	// allocateScript decides what a newcomer gets. The queue length is checked
	// here, inside the same atomic step as the allocation itself: if anyone is
	// already waiting, a newcomer must join the tail rather than take a unit that
	// belongs to the head of the queue (docs/design_context.md, FIFO is the only
	// fairness policy).
	allocateScript = redis.NewScript(`
		local stockKey = KEYS[1]
		local queueKey = KEYS[3]
		local reqQty = tonumber(ARGV[1])

		local avail = tonumber(redis.call('HGET', stockKey, 'available_units') or '0')
		local count = tonumber(redis.call('HGET', stockKey, 'product_count') or '0')

		if count == 0 then
			return {0, 0, 1}
		end

		if redis.call('ZCARD', queueKey) > 0 then
			return {0, 0, 0}
		end

		if avail >= reqQty then
			redis.call('HINCRBY', stockKey, 'available_units', -reqQty)
			redis.call('PUBLISH', KEYS[2], 'changed')
			return {reqQty, 0, 0}
		end

		if avail > 0 then
			redis.call('HINCRBY', stockKey, 'available_units', -avail)
			redis.call('PUBLISH', KEYS[2], 'changed')
			return {0, avail, 0}
		end

		return {0, 0, 0}
	`)

	enqueueScript = redis.NewScript(`
		local seq = redis.call('INCR', KEYS[1])
		redis.call('ZADD', KEYS[2], seq, ARGV[1])
		redis.call('PUBLISH', KEYS[3], 'changed')
		return seq
	`)

	removeFromQueueScript = redis.NewScript(`
		local removed = redis.call('ZREM', KEYS[1], ARGV[1])
		if removed > 0 then
			redis.call('PUBLISH', KEYS[2], 'changed')
		end
		return removed
	`)

	restoreAvailableUnitsScript = redis.NewScript(`
		redis.call('HINCRBY', KEYS[1], 'available_units', ARGV[1])
		redis.call('PUBLISH', KEYS[2], 'changed')
		return 1
	`)

	requeueScript = redis.NewScript(`
		redis.call('ZADD', KEYS[1], ARGV[1], ARGV[2])
		redis.call('PUBLISH', KEYS[2], 'changed')
		return 1
	`)

	// claimExpiredScript moves due timers from the scheduled set into the
	// processing set under a lease, in one atomic step.
	//
	// The previous version deleted them outright, so a process that died between
	// the read and the handling lost them for good: the right stayed ACTIVE with
	// nobody left to expire it, and its unit never returned to the pool. Under a
	// lease an unfinished item is merely late, not lost.
	claimExpiredScript = redis.NewScript(`
		local scheduled = KEYS[1]
		local processing = KEYS[2]
		local now = ARGV[1]
		local leaseUntil = ARGV[2]
		local limit = tonumber(ARGV[3])

		local due = redis.call('ZRANGE', scheduled, '-inf', now, 'BYSCORE', 'LIMIT', 0, limit)
		if #due == 0 then
			return {}
		end

		for _, member in ipairs(due) do
			redis.call('ZREM', scheduled, member)
			redis.call('ZADD', processing, leaseUntil, member)
		end

		return due
	`)

	// nackExpiredScript returns a claimed item to the schedule after a failure,
	// so the next pass picks it up instead of dropping it.
	nackExpiredScript = redis.NewScript(`
		local processing = KEYS[1]
		local scheduled = KEYS[2]
		local retryAt = ARGV[1]

		for i = 2, #ARGV do
			redis.call('ZREM', processing, ARGV[i])
			redis.call('ZADD', scheduled, retryAt, ARGV[i])
		end

		return #ARGV - 1
	`)

	// reclaimStaleExpiredScript rescues items whose lease ran out — the worker
	// holding them died. They go back to the schedule as due right now.
	reclaimStaleExpiredScript = redis.NewScript(`
		local processing = KEYS[1]
		local scheduled = KEYS[2]
		local now = ARGV[1]

		local stale = redis.call('ZRANGE', processing, '-inf', now, 'BYSCORE')
		if #stale == 0 then
			return 0
		end

		for _, member in ipairs(stale) do
			redis.call('ZREM', processing, member)
			redis.call('ZADD', scheduled, now, member)
		end

		return #stale
	`)

	refreshExpiryTimerScript = redis.NewScript(`
		local current = redis.call('ZSCORE', KEYS[1], ARGV[1])
		if not current then
			return 0
		end

		local deadline = tonumber(ARGV[2])
		local now = tonumber(ARGV[3])
		if tonumber(current) <= now then
			return 0
		end

		if deadline > tonumber(current) then
			redis.call('ZADD', KEYS[1], deadline, ARGV[1])
		end
		return 1
	`)

	popAndAllocateScript = redis.NewScript(`
		local queueKey = KEYS[1]
		local stockKey = KEYS[2]
		local pid = ARGV[1]

		local first = redis.call('ZRANGE', queueKey, 0, 0, 'WITHSCORES')
		if #first == 0 then
			return {"", 0, 0, 0, "", 0}
		end

		local uid = first[1]
		local score = tonumber(first[2])
		local memKey = "member:" .. pid .. ":" .. uid

		local status = redis.call('HGET', memKey, 'status')
		if not status or status ~= 'QUEUED' then
			redis.call('ZREM', queueKey, uid)
			redis.call('PUBLISH', KEYS[3], 'changed')
			return {uid, 0, 0, 0, status or "GHOST", score}
		end

		local reqQty = tonumber(redis.call('HGET', memKey, 'quantity') or '0')
		local avail = tonumber(redis.call('HGET', stockKey, 'available_units') or '0')
		local count = tonumber(redis.call('HGET', stockKey, 'product_count') or '0')

		if count == 0 then
			redis.call('ZREM', queueKey, uid)
			redis.call('PUBLISH', KEYS[3], 'changed')
			return {uid, 0, 0, 1, "SOLD_OUT", score}
		end

		if avail >= reqQty then
			redis.call('HINCRBY', stockKey, 'available_units', -reqQty)
			redis.call('ZREM', queueKey, uid)
			redis.call('PUBLISH', KEYS[3], 'changed')
			return {uid, reqQty, 0, 0, "RIGHT_ACTIVE", score}
		end

		if avail > 0 then
			redis.call('HINCRBY', stockKey, 'available_units', -avail)
			redis.call('ZREM', queueKey, uid)
			redis.call('PUBLISH', KEYS[3], 'changed')
			return {uid, 0, avail, 0, "OFFER_PENDING", score}
		end

		return {uid, 0, 0, 0, "QUEUED", score}
	`)
)

// CacheRepo implements the service.CacheRepo interface using Redis.
type CacheRepo struct {
	client *redis.Client
}

// NewCacheRepo creates a new Redis repository instance.
func NewCacheRepo(client *redis.Client) *CacheRepo {
	return &CacheRepo{
		client: client,
	}
}

func userUpdatesChannel(productID, userID string) string {
	return fmt.Sprintf("updates:%s:%s", productID, userID)
}

func queueUpdatesChannel(productID string) string {
	return fmt.Sprintf("queue-updates:%s", productID)
}

// InitStock initializes the product stock in the cache if it doesn't already exist.
func (c *CacheRepo) InitStock(ctx context.Context, productID string, totalStock int) error {
	key := fmt.Sprintf("stock:%s", productID)
	err := initStockScript.Run(ctx, c.client, []string{key, queueUpdatesChannel(productID)}, totalStock).Err()
	if err != nil && !errors.Is(err, redis.Nil) {
		return fmt.Errorf("redis.CacheRepo.InitStock execute script: %w", err)
	}
	return nil
}

// TryAllocate attempts to reserve the requested quantity using a Lua script.
func (c *CacheRepo) TryAllocate(ctx context.Context, productID string, quantity int) (int, int, bool, error) {
	key := fmt.Sprintf("stock:%s", productID)

	res, err := allocateScript.Run(
		ctx,
		c.client,
		[]string{key, queueUpdatesChannel(productID), queueKey(productID)},
		quantity,
	).Result()
	if err != nil {
		return 0, 0, false, fmt.Errorf("redis.CacheRepo.TryAllocate execute script: %w", err)
	}

	resSlice, ok := res.([]interface{})
	if !ok || len(resSlice) != 3 {
		return 0, 0, false, fmt.Errorf("redis.CacheRepo.TryAllocate: %w", ErrInvalidResponse)
	}

	allocated := int(resSlice[0].(int64))
	available := int(resSlice[1].(int64))
	soldOut := resSlice[2].(int64) == 1

	return allocated, available, soldOut, nil
}

// CommitPurchase decrements the physical product_count in the cache after a successful payment.
func (c *CacheRepo) CommitPurchase(ctx context.Context, productID string, quantity int) error {
	key := fmt.Sprintf("stock:%s", productID)
	err := c.client.HIncrBy(ctx, key, "product_count", int64(-quantity)).Err()
	if err != nil {
		return fmt.Errorf("redis.CacheRepo.CommitPurchase: %w", err)
	}
	return nil
}

// Enqueue places a user at the end of the FIFO queue using a monotonic counter.
func (c *CacheRepo) Enqueue(ctx context.Context, productID string, userID string) error {
	seqKey := fmt.Sprintf("queue:%s:seq", productID)
	queueKey := fmt.Sprintf("queue:%s", productID)

	err := enqueueScript.Run(ctx, c.client, []string{seqKey, queueKey, queueUpdatesChannel(productID)}, userID).Err()
	if err != nil {
		return fmt.Errorf("redis.CacheRepo.Enqueue: %w", err)
	}
	return nil
}

// RemoveFromQueue completely removes a user from the product's queue.
func (c *CacheRepo) RemoveFromQueue(ctx context.Context, productID string, userID string) error {
	queueKey := fmt.Sprintf("queue:%s", productID)
	err := removeFromQueueScript.Run(ctx, c.client, []string{queueKey, queueUpdatesChannel(productID)}, userID).Err()
	if err != nil {
		return fmt.Errorf("redis.CacheRepo.RemoveFromQueue: %w", err)
	}
	return nil
}

// SetMembership quickly caches the user's current state.
func (c *CacheRepo) SetMembership(ctx context.Context, membership *models.QueueMembership) error {
	key := fmt.Sprintf("member:%s:%s", membership.ProductID, membership.UserID)

	fields := map[string]interface{}{
		"product_id": membership.ProductID,
		"user_id":    membership.UserID,
		"status":     string(membership.Status),
		"quantity":   membership.Quantity,
		"created_at": membership.CreatedAt.Format(time.RFC3339Nano),
		"updated_at": membership.UpdatedAt.Format(time.RFC3339Nano),
	}

	if membership.AvailableQuantity != nil {
		fields["available_quantity"] = *membership.AvailableQuantity
	} else {
		fields["available_quantity"] = ""
	}

	if membership.CurrentToken != nil {
		fields["current_token"] = *membership.CurrentToken
	} else {
		fields["current_token"] = ""
	}

	if membership.ExpiresAt != nil {
		fields["expires_at"] = membership.ExpiresAt.Format(time.RFC3339Nano)
	} else {
		fields["expires_at"] = ""
	}

	err := c.client.HSet(ctx, key, fields).Err()
	if err != nil {
		return fmt.Errorf("redis.CacheRepo.SetMembership: %w", err)
	}
	return nil
}

// GetMembership retrieves the cached state of a user.
func (c *CacheRepo) GetMembership(ctx context.Context, productID string, userID string) (*models.QueueMembership, error) {
	key := fmt.Sprintf("member:%s:%s", productID, userID)

	res, err := c.client.HGetAll(ctx, key).Result()
	if err != nil {
		return nil, fmt.Errorf("redis.CacheRepo.GetMembership execute: %w", err)
	}
	if len(res) == 0 {
		return nil, fmt.Errorf("redis.CacheRepo.GetMembership not found: %w", models.ErrTokenNotFound)
	}

	membership := &models.QueueMembership{
		ProductID: res["product_id"],
		UserID:    res["user_id"],
		Status:    models.MembershipStatus(res["status"]),
		CreatedAt: parseTime(res["created_at"]),
		UpdatedAt: parseTime(res["updated_at"]),
		ExpiresAt: parseTimePtr(res["expires_at"]),
	}

	if qty, err := strconv.Atoi(res["quantity"]); err == nil {
		membership.Quantity = qty
	}
	if availStr := res["available_quantity"]; availStr != "" {
		if avail, err := strconv.Atoi(availStr); err == nil {
			membership.AvailableQuantity = &avail
		}
	}
	if tkn := res["current_token"]; tkn != "" {
		membership.CurrentToken = &tkn
	}

	return membership, nil
}

// SetRight caches an issued right for fast validation before checkout.
func (c *CacheRepo) SetRight(ctx context.Context, right *models.Right) error {
	key := fmt.Sprintf("right:%s", right.Token)

	fields := map[string]interface{}{
		"token":      right.Token,
		"user_id":    right.UserID,
		"product_id": right.ProductID,
		"quantity":   right.Quantity,
		"status":     string(right.Status),
		"created_at": right.CreatedAt.Format(time.RFC3339Nano),
		"expires_at": right.ExpiresAt.Format(time.RFC3339Nano),
	}

	if right.OrderID != nil {
		fields["order_id"] = *right.OrderID
	} else {
		fields["order_id"] = ""
	}

	if right.UsedAt != nil {
		fields["used_at"] = right.UsedAt.Format(time.RFC3339Nano)
	} else {
		fields["used_at"] = ""
	}

	err := c.client.HSet(ctx, key, fields).Err()
	if err != nil {
		return fmt.Errorf("redis.CacheRepo.SetRight: %w", err)
	}
	return nil
}

// GetRight retrieves a cached right by its token.
func (c *CacheRepo) GetRight(ctx context.Context, token string) (*models.Right, error) {
	key := fmt.Sprintf("right:%s", token)

	res, err := c.client.HGetAll(ctx, key).Result()
	if err != nil {
		return nil, fmt.Errorf("redis.CacheRepo.GetRight execute: %w", err)
	}
	if len(res) == 0 {
		return nil, fmt.Errorf("redis.CacheRepo.GetRight not found: %w", models.ErrTokenNotFound)
	}

	right := &models.Right{
		Token:     res["token"],
		UserID:    res["user_id"],
		ProductID: res["product_id"],
		Status:    models.RightStatus(res["status"]),
		CreatedAt: parseTime(res["created_at"]),
		ExpiresAt: parseTime(res["expires_at"]),
		UsedAt:    parseTimePtr(res["used_at"]),
	}

	if qty, err := strconv.Atoi(res["quantity"]); err == nil {
		right.Quantity = qty
	}
	if ord := res["order_id"]; ord != "" {
		right.OrderID = &ord
	}

	return right, nil
}

// PublishEvent broadcasts a status change to connected WebSocket clients.
func (c *CacheRepo) PublishEvent(ctx context.Context, productID string, userID string, payload interface{}) error {
	channel := userUpdatesChannel(productID, userID)

	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("redis.CacheRepo.PublishEvent marshal payload: %w", err)
	}

	err = c.client.Publish(ctx, channel, data).Err()
	if err != nil {
		return fmt.Errorf("redis.CacheRepo.PublishEvent publish: %w", err)
	}
	return nil
}

// SubscribeUpdates listens for both user-specific state changes and product-wide
// queue changes. Events are invalidation signals; consumers should re-read the
// current membership and queue metrics instead of trusting event payloads.
func (c *CacheRepo) SubscribeUpdates(
	ctx context.Context,
	productID string,
	userID string,
) (<-chan struct{}, func() error, error) {
	pubsub := c.client.Subscribe(ctx, userUpdatesChannel(productID, userID), queueUpdatesChannel(productID))
	if _, err := pubsub.Receive(ctx); err != nil {
		_ = pubsub.Close()
		return nil, nil, fmt.Errorf("redis.CacheRepo.SubscribeUpdates subscribe: %w", err)
	}

	subscriptionCtx, cancel := context.WithCancel(ctx)
	messages := pubsub.Channel()
	events := make(chan struct{})

	var closeOnce sync.Once
	var closeErr error
	closeSubscription := func() error {
		closeOnce.Do(func() {
			cancel()
			closeErr = pubsub.Close()
		})
		return closeErr
	}

	go func() {
		defer close(events)

		for {
			select {
			case <-subscriptionCtx.Done():
				return
			case _, ok := <-messages:
				if !ok {
					return
				}

				select {
				case events <- struct{}{}:
				case <-subscriptionCtx.Done():
					return
				}
			}
		}
	}()

	return events, closeSubscription, nil
}

// AddToExpiryTimer sets up background tracking for a time-bound right or offer.
func (c *CacheRepo) AddToExpiryTimer(ctx context.Context, productID string, userID string, expiresAt time.Time) error {
	member := fmt.Sprintf("%s:%s", productID, userID)

	err := c.client.ZAdd(ctx, "expiring:rights", redis.Z{
		Score:  float64(expiresAt.Unix()),
		Member: member,
	}).Err()
	if err != nil {
		return fmt.Errorf("redis.CacheRepo.AddToExpiryTimer: %w", err)
	}
	return nil
}

// RefreshExpiryTimer extends an existing timer without recreating one already
// claimed by the expiration worker. Concurrent refreshes can only move it forward.
func (c *CacheRepo) RefreshExpiryTimer(
	ctx context.Context,
	productID string,
	userID string,
	expiresAt time.Time,
) (bool, error) {
	member := fmt.Sprintf("%s:%s", productID, userID)

	refreshed, err := refreshExpiryTimerScript.Run(
		ctx, c.client, []string{"expiring:rights"}, member, expiresAt.Unix(), time.Now().UTC().Unix(),
	).Int()
	if err != nil {
		return false, fmt.Errorf("redis.CacheRepo.RefreshExpiryTimer: %w", err)
	}

	return refreshed == 1, nil
}

// RemoveFromExpiryTimer removes a user's timer if they complete an action before expiration.
func (c *CacheRepo) RemoveFromExpiryTimer(ctx context.Context, productID string, userID string) error {
	member := fmt.Sprintf("%s:%s", productID, userID)

	err := c.client.ZRem(ctx, "expiring:rights", member).Err()
	if err != nil {
		return fmt.Errorf("redis.CacheRepo.RemoveFromExpiryTimer: %w", err)
	}
	return nil
}

// parseTime is a helper to securely parse time from strings stored in Redis.
func parseTime(s string) time.Time {
	t, _ := time.Parse(time.RFC3339Nano, s)
	return t
}

// parseTimePtr is a helper to securely parse optional time pointers from strings stored in Redis.
func parseTimePtr(s string) *time.Time {
	if s == "" {
		return nil
	}
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		return nil
	}
	return &t
}

// RestoreAvailableUnits returns unused or rolled-back stock to the available pool.
func (c *CacheRepo) RestoreAvailableUnits(ctx context.Context, productID string, quantity int) error {
	key := fmt.Sprintf("stock:%s", productID)
	err := restoreAvailableUnitsScript.Run(
		ctx,
		c.client,
		[]string{key, queueUpdatesChannel(productID)},
		quantity,
	).Err()
	if err != nil {
		return fmt.Errorf("redis.CacheRepo.RestoreAvailableUnits: %w", err)
	}
	return nil
}

// GetFirstInQueue retrieves the first user ID from the queue without removing it.
func (c *CacheRepo) GetFirstInQueue(ctx context.Context, productID string) (string, error) {
	key := fmt.Sprintf("queue:%s", productID)

	res, err := c.client.ZRange(ctx, key, 0, 0).Result()
	if err != nil {
		return "", fmt.Errorf("redis.CacheRepo.GetFirstInQueue: %w", err)
	}
	if len(res) == 0 {
		return "", models.ErrTokenNotFound
	}

	return res[0], nil
}

// PopAndAllocate atomically reads the first user, removes them if applicable, and allocates stock.
func (c *CacheRepo) PopAndAllocate(ctx context.Context, productID string) (string, int, int, bool, models.MembershipStatus, float64, error) {
	queueKey := fmt.Sprintf("queue:%s", productID)
	stockKey := fmt.Sprintf("stock:%s", productID)

	res, err := popAndAllocateScript.Run(
		ctx,
		c.client,
		[]string{queueKey, stockKey, queueUpdatesChannel(productID)},
		productID,
	).Result()
	if err != nil {
		return "", 0, 0, false, "", 0, fmt.Errorf("redis.CacheRepo.PopAndAllocate execute script: %w", err)
	}

	resSlice, ok := res.([]interface{})
	if !ok || len(resSlice) != 6 {
		return "", 0, 0, false, "", 0, fmt.Errorf("redis.CacheRepo.PopAndAllocate: %w", ErrInvalidResponse)
	}

	uid := resSlice[0].(string)
	if uid == "" {
		return "", 0, 0, false, "", 0, nil
	}

	allocated := int(resSlice[1].(int64))
	available := int(resSlice[2].(int64))
	soldOut := resSlice[3].(int64) == 1
	status := models.MembershipStatus(resSlice[4].(string))
	score := float64(resSlice[5].(int64))

	return uid, allocated, available, soldOut, status, score, nil
}

// Requeue atomically puts a user back into the queue at their original position (used for rollbacks).
func (c *CacheRepo) Requeue(ctx context.Context, productID string, userID string, score float64) error {
	queueKey := fmt.Sprintf("queue:%s", productID)

	err := requeueScript.Run(
		ctx,
		c.client,
		[]string{queueKey, queueUpdatesChannel(productID)},
		score,
		userID,
	).Err()
	if err != nil {
		return fmt.Errorf("redis.CacheRepo.Requeue: %w", err)
	}
	return nil
}

// GetQueueMetrics retrieves the user's 0-indexed rank in the queue and the currently available stock.
// It uses a pipeline to fetch both values in a single network round-trip.
func (c *CacheRepo) GetQueueMetrics(ctx context.Context, productID string, userID string) (int, int, error) {
	queueKey := fmt.Sprintf("queue:%s", productID)
	stockKey := fmt.Sprintf("stock:%s", productID)

	pipe := c.client.Pipeline()
	rankCmd := pipe.ZRank(ctx, queueKey, userID)
	availCmd := pipe.HGet(ctx, stockKey, "available_units")

	// Exec returns redis.Nil if ANY of the pipeline commands return redis.Nil.
	// We safely ignore it here and check the specific command results below.
	_, err := pipe.Exec(ctx)
	if err != nil && !errors.Is(err, redis.Nil) {
		return 0, 0, fmt.Errorf("redis.CacheRepo.GetQueueMetrics pipeline exec: %w", err)
	}

	rank, err := rankCmd.Result()
	if errors.Is(err, redis.Nil) {
		// User is completely out of the ZSET queue.
		return 0, 0, models.ErrMembershipNotFound
	} else if err != nil {
		return 0, 0, fmt.Errorf("redis.CacheRepo.GetQueueMetrics rank: %w", err)
	}

	var available int
	availStr, err := availCmd.Result()
	if err == nil && availStr != "" {
		if parsed, parseErr := strconv.Atoi(availStr); parseErr == nil {
			available = parsed
		}
	}

	// rank is 0-indexed. The mathematical offset is handled in the service layer.
	return int(rank), available, nil
}

// GetStock reads the cached stock counters of a product. It returns
// models.ErrTokenNotFound when the product has never been touched — Queue
// Service only learns about a product when someone first tries to buy it.
func (c *CacheRepo) GetStock(ctx context.Context, productID string) (productCount, available int, err error) {
	key := fmt.Sprintf("stock:%s", productID)

	res, err := c.client.HGetAll(ctx, key).Result()
	if err != nil {
		return 0, 0, fmt.Errorf("redis.CacheRepo.GetStock execute: %w", err)
	}
	if len(res) == 0 {
		return 0, 0, fmt.Errorf("redis.CacheRepo.GetStock not found: %w", models.ErrTokenNotFound)
	}

	productCount, _ = strconv.Atoi(res["product_count"])
	available, _ = strconv.Atoi(res["available_units"])

	return productCount, available, nil
}

// queueKey names the FIFO queue of a product.
func queueKey(productID string) string {
	return fmt.Sprintf("queue:%s", productID)
}

// joinClaimKey guards a single (product, user) pair while their entry into the
// queue is being decided.
func joinClaimKey(productID, userID string) string {
	return fmt.Sprintf("join-claim:%s:%s", productID, userID)
}

// ClaimJoin marks the start of an entry attempt and reports whether the caller
// won the claim. Losing means a concurrent request for the same user is already
// mid-flight.
//
// The claim carries a short TTL so a process that dies mid-entry cannot lock the
// user out: the key expires on its own and the next attempt proceeds.
func (c *CacheRepo) ClaimJoin(ctx context.Context, productID, userID string, ttl time.Duration) (bool, error) {
	won, err := c.client.SetNX(ctx, joinClaimKey(productID, userID), "1", ttl).Result()
	if err != nil {
		return false, fmt.Errorf("redis.CacheRepo.ClaimJoin: %w", err)
	}

	return won, nil
}

// ReleaseJoinClaim frees the claim once the entry is decided, so a legitimate
// repeat request does not have to wait out the whole TTL.
func (c *CacheRepo) ReleaseJoinClaim(ctx context.Context, productID, userID string) error {
	if err := c.client.Del(ctx, joinClaimKey(productID, userID)).Err(); err != nil {
		return fmt.Errorf("redis.CacheRepo.ReleaseJoinClaim: %w", err)
	}

	return nil
}

// expiryScheduledKey holds timers waiting for their turn; expiryProcessingKey
// holds the ones a worker has taken, scored by when its lease runs out.
const (
	expiryScheduledKey  = "expiring:rights"
	expiryProcessingKey = "expiring:processing"
)

// ClaimExpired takes up to limit due timers under a lease and returns them.
// Claimed items disappear from the schedule but are not lost: if the caller never
// acknowledges them, ReclaimStaleExpired puts them back once the lease expires.
func (c *CacheRepo) ClaimExpired(
	ctx context.Context, now time.Time, lease time.Duration, limit int,
) ([]string, error) {
	res, err := claimExpiredScript.Run(
		ctx,
		c.client,
		[]string{expiryScheduledKey, expiryProcessingKey},
		now.Unix(), now.Add(lease).Unix(), limit,
	).StringSlice()
	if err != nil && !errors.Is(err, redis.Nil) {
		return nil, fmt.Errorf("redis.CacheRepo.ClaimExpired: %w", err)
	}

	return res, nil
}

// AckExpired confirms that claimed timers were handled and drops them for good.
func (c *CacheRepo) AckExpired(ctx context.Context, keys []string) error {
	if len(keys) == 0 {
		return nil
	}

	members := make([]any, 0, len(keys))
	for _, key := range keys {
		members = append(members, key)
	}

	if err := c.client.ZRem(ctx, expiryProcessingKey, members...).Err(); err != nil {
		return fmt.Errorf("redis.CacheRepo.AckExpired: %w", err)
	}

	return nil
}

// NackExpired returns claimed timers to the schedule after a failed attempt.
func (c *CacheRepo) NackExpired(ctx context.Context, keys []string, retryAt time.Time) error {
	if len(keys) == 0 {
		return nil
	}

	args := make([]any, 0, len(keys)+1)
	args = append(args, retryAt.Unix())
	for _, key := range keys {
		args = append(args, key)
	}

	err := nackExpiredScript.Run(
		ctx, c.client, []string{expiryProcessingKey, expiryScheduledKey}, args...,
	).Err()
	if err != nil && !errors.Is(err, redis.Nil) {
		return fmt.Errorf("redis.CacheRepo.NackExpired: %w", err)
	}

	return nil
}

// ReclaimStaleExpired returns timers whose lease ran out — the worker that held
// them is gone — and reports how many were rescued.
func (c *CacheRepo) ReclaimStaleExpired(ctx context.Context, now time.Time) (int, error) {
	count, err := reclaimStaleExpiredScript.Run(
		ctx, c.client, []string{expiryProcessingKey, expiryScheduledKey}, now.Unix(),
	).Int()
	if err != nil && !errors.Is(err, redis.Nil) {
		return 0, fmt.Errorf("redis.CacheRepo.ReclaimStaleExpired: %w", err)
	}

	return count, nil
}
