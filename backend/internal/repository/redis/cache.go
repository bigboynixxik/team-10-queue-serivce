// Package redis provides high-speed, concurrency-safe storage operations.
// It acts as the hot-path cache and handles race conditions via atomic Lua scripts.
package redis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
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
			return 1
		end
		return 0
	`)

	allocateScript = redis.NewScript(`
		local stockKey = KEYS[1]
		local reqQty = tonumber(ARGV[1])
		
		local avail = tonumber(redis.call('HGET', stockKey, 'available_units') or '0')
		local count = tonumber(redis.call('HGET', stockKey, 'product_count') or '0')

		if count == 0 then
			return {0, 0, 1}
		end

		if avail >= reqQty then
			redis.call('HINCRBY', stockKey, 'available_units', -reqQty)
			return {reqQty, 0, 0}
		end

		if avail > 0 then
			redis.call('HINCRBY', stockKey, 'available_units', -avail)
			return {0, avail, 0}
		end

		return {0, 0, 0}
	`)

	enqueueScript = redis.NewScript(`
		local seq = redis.call('INCR', KEYS[1])
		redis.call('ZADD', KEYS[2], seq, ARGV[1])
		return seq
	`)

	getAndRemoveExpiredScript = redis.NewScript(`
		local expired = redis.call('ZRANGE', KEYS[1], '-inf', ARGV[1], 'BYSCORE')
		if #expired > 0 then
			redis.call('ZREMRANGEBYSCORE', KEYS[1], '-inf', ARGV[1])
		end
		return expired
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
			return {uid, 0, 0, 0, status or "GHOST", score}
		end

		local reqQty = tonumber(redis.call('HGET', memKey, 'quantity') or '0')
		local avail = tonumber(redis.call('HGET', stockKey, 'available_units') or '0')
		local count = tonumber(redis.call('HGET', stockKey, 'product_count') or '0')

		if count == 0 then
			redis.call('ZREM', queueKey, uid)
			return {uid, 0, 0, 1, "SOLD_OUT", score}
		end

		if avail >= reqQty then
			redis.call('HINCRBY', stockKey, 'available_units', -reqQty)
			redis.call('ZREM', queueKey, uid)
			return {uid, reqQty, 0, 0, "RIGHT_ACTIVE", score}
		end

		if avail > 0 then
			redis.call('HINCRBY', stockKey, 'available_units', -avail)
			redis.call('ZREM', queueKey, uid)
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

// InitStock initializes the product stock in the cache if it doesn't already exist.
func (c *CacheRepo) InitStock(ctx context.Context, productID string, totalStock int) error {
	key := fmt.Sprintf("stock:%s", productID)
	err := initStockScript.Run(ctx, c.client, []string{key}, totalStock).Err()
	if err != nil && !errors.Is(err, redis.Nil) {
		return fmt.Errorf("redis.CacheRepo.InitStock execute script: %w", err)
	}
	return nil
}

// TryAllocate attempts to reserve the requested quantity using a Lua script.
func (c *CacheRepo) TryAllocate(ctx context.Context, productID string, quantity int) (int, int, bool, error) {
	key := fmt.Sprintf("stock:%s", productID)

	res, err := allocateScript.Run(ctx, c.client, []string{key}, quantity).Result()
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

	err := enqueueScript.Run(ctx, c.client, []string{seqKey, queueKey}, userID).Err()
	if err != nil {
		return fmt.Errorf("redis.CacheRepo.Enqueue: %w", err)
	}
	return nil
}

// RemoveFromQueue completely removes a user from the product's queue.
func (c *CacheRepo) RemoveFromQueue(ctx context.Context, productID string, userID string) error {
	queueKey := fmt.Sprintf("queue:%s", productID)
	err := c.client.ZRem(ctx, queueKey, userID).Err()
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
	channel := fmt.Sprintf("updates:%s:%s", productID, userID)

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
	err := c.client.HIncrBy(ctx, key, "available_units", int64(quantity)).Err()
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

// GetAndRemoveExpired atomically retrieves and removes items from the expiry timer that have timed out.
func (c *CacheRepo) GetAndRemoveExpired(ctx context.Context, now time.Time) ([]string, error) {
	score := strconv.FormatInt(now.Unix(), 10)

	res, err := getAndRemoveExpiredScript.Run(ctx, c.client, []string{"expiring:rights"}, score).StringSlice()
	if err != nil && !errors.Is(err, redis.Nil) {
		return nil, fmt.Errorf("redis.CacheRepo.GetAndRemoveExpired execute script: %w", err)
	}

	return res, nil
}

// PopAndAllocate atomically reads the first user, removes them if applicable, and allocates stock.
func (c *CacheRepo) PopAndAllocate(ctx context.Context, productID string) (string, int, int, bool, models.MembershipStatus, float64, error) {
	queueKey := fmt.Sprintf("queue:%s", productID)
	stockKey := fmt.Sprintf("stock:%s", productID)

	res, err := popAndAllocateScript.Run(ctx, c.client, []string{queueKey, stockKey}, productID).Result()
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

	err := c.client.ZAdd(ctx, queueKey, redis.Z{Score: score, Member: userID}).Err()
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
