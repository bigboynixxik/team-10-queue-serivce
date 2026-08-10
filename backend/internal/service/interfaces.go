// Package service defines the core business logic and orchestrates operations
// between durable storage, cache, and external clients.
package service

import (
	"context"
	"time"

	"backend/internal/models"
)

// DurableRepo defines the contract for reliable, persistent storage (PostgreSQL).
// It acts as the source of truth and owns transactional state transitions.
type DurableRepo interface {
	// SaveRight persists a newly issued purchase right.
	SaveRight(ctx context.Context, right *models.Right) error

	// IssueRightAndUpsertMembershipTx atomically persists a newly issued right
	// and the membership that owns it.
	IssueRightAndUpsertMembershipTx(ctx context.Context, right *models.Right, membership *models.QueueMembership) error

	// GetRightByToken retrieves a right by its unique token.
	GetRightByToken(ctx context.Context, token string) (*models.Right, error)

	// LoadRecoverySnapshot reads the durable state needed to rebuild Redis.
	LoadRecoverySnapshot(ctx context.Context) (*models.RecoverySnapshot, error)

	// UpsertMembership creates or updates a user's current status in the queue.
	// It handles (product_id, user_id) conflicts gracefully.
	UpsertMembership(ctx context.Context, membership *models.QueueMembership) error

	// UseRightTx atomically locks an ACTIVE right, marks it as USED, decrements
	// product_stock, writes a stock decrement outbox event, and finalizes the
	// matching membership when it still owns this token. transitioned is false
	// for an already processed webhook, so side effects are not repeated.
	UseRightTx(ctx context.Context, token string, orderID string, now time.Time) (right *models.Right, transitioned bool, err error)

	// ClaimStockDecrements leases due outbox events for external delivery.
	ClaimStockDecrements(ctx context.Context, now time.Time, leaseUntil time.Time, limit int) ([]models.StockDecrement, error)

	// MarkStockDecrementDelivered acknowledges a successfully delivered event.
	MarkStockDecrementDelivered(ctx context.Context, eventID string, now time.Time) error

	// RescheduleStockDecrement releases a failed event for a later retry.
	RescheduleStockDecrement(ctx context.Context, eventID string, nextAttemptAt time.Time, lastError string, now time.Time) error

	// ExpireRightAndUpsertMembershipTx atomically marks an ACTIVE right as EXPIRED
	// and persists the corresponding terminal membership state.
	ExpireRightAndUpsertMembershipTx(ctx context.Context, token string, membership *models.QueueMembership) (right *models.Right, transitioned bool, err error)

	// ExpireRights marks the given ACTIVE rights as EXPIRED. Recovery uses it to
	// settle rights no live membership refers to any more.
	ExpireRights(ctx context.Context, tokens []string) error

	// SaveInitialStock persists the physical stock fetched from AvitoBackend.
	SaveInitialStock(ctx context.Context, stock *models.ProductStock) error

	// CountMembershipsByStatus reports how many users sit in each status for a
	// product. Reporting read, not part of the allocation path.
	CountMembershipsByStatus(ctx context.Context, productID string) (map[models.MembershipStatus]int, error)

	// ListMembershipsByUser returns every queue the user takes part in.
	ListMembershipsByUser(ctx context.Context, userID string) ([]*models.QueueMembership, error)
}

// CacheRepo defines the contract for high-speed, concurrency-safe storage (Redis).
// It acts as the hot-path and handles race conditions via atomic operations (Lua).
type CacheRepo interface {
	// InitStock initializes the product stock in the cache if it doesn't already exist.
	InitStock(ctx context.Context, productID string, totalStock int) error

	// TryAllocate attempts to reserve the requested quantity using a Lua script.
	// It returns the allocated quantity, any available partial quantity, and a soldOut flag.
	TryAllocate(ctx context.Context, productID string, quantity int) (allocated int, available int, soldOut bool, err error)

	// CommitPurchase decrements the physical product_count in the cache after a successful payment.
	CommitPurchase(ctx context.Context, productID string, quantity int) error

	// Enqueue places a user at the end of the FIFO queue using a monotonic counter.
	Enqueue(ctx context.Context, productID string, userID string) error

	// RemoveFromQueue completely removes a user from the product's queue.
	RemoveFromQueue(ctx context.Context, productID string, userID string) error

	// SetMembership quickly caches the user's current state.
	SetMembership(ctx context.Context, membership *models.QueueMembership) error

	// GetMembership retrieves the cached state of a user.
	GetMembership(ctx context.Context, productID string, userID string) (*models.QueueMembership, error)

	// MarkPurchasedIfCurrentToken finalizes the cached membership only when it
	// still points at the paid right token.
	MarkPurchasedIfCurrentToken(ctx context.Context, right *models.Right, updatedAt time.Time) (bool, error)

	// SetRight caches an issued right for fast validation before checkout.
	SetRight(ctx context.Context, right *models.Right) error

	// GetRight retrieves a cached right by its token.
	GetRight(ctx context.Context, token string) (*models.Right, error)

	// ClaimMembership marks the start of a membership transition and reports
	// whether the caller won it. Losing means a concurrent request for the same
	// user is in flight.
	ClaimMembership(ctx context.Context, productID, userID, ownerID string, ttl time.Duration) (bool, error)

	// ReleaseMembershipClaim frees the claim only if ownerID still owns it.
	ReleaseMembershipClaim(ctx context.Context, productID, userID, ownerID string) error

	// GetStock reads the cached stock counters of a product.
	GetStock(ctx context.Context, productID string) (productCount, available int, err error)

	// PublishEvent broadcasts a status change to connected WebSocket clients.
	PublishEvent(ctx context.Context, productID string, userID string, payload interface{}) error

	// AddToExpiryTimer sets up background tracking for a time-bound right or offer.
	AddToExpiryTimer(ctx context.Context, productID string, userID string, expiresAt time.Time) error

	// RefreshExpiryTimer atomically extends an existing timer without recreating
	// a timer that the expiration worker has already claimed.
	RefreshExpiryTimer(
		ctx context.Context, productID string, userID string, expiresAt time.Time,
	) (refreshed bool, err error)
	// RemoveFromExpiryTimer removes a user's timer if they complete an action before expiration.
	RemoveFromExpiryTimer(ctx context.Context, productID string, userID string) error

	// RestoreAvailableUnits returns unused or rolled-back stock to the available pool.
	RestoreAvailableUnits(ctx context.Context, productID string, quantity int) error

	// GetFirstInQueue retrieves the first user ID from the queue without removing it.
	GetFirstInQueue(ctx context.Context, productID string) (string, error)

	// ClaimExpired takes up to limit due timers under a lease. Unacknowledged
	// items return to the schedule once the lease runs out, so a crashed worker
	// delays the work instead of losing it.
	ClaimExpired(ctx context.Context, now time.Time, lease time.Duration, limit int) ([]models.ExpiryClaim, error)

	// AckExpired confirms timers only while the caller still owns their lease.
	AckExpired(ctx context.Context, claims []models.ExpiryClaim) error

	// NackExpired returns timers only while the caller still owns their lease.
	NackExpired(ctx context.Context, claims []models.ExpiryClaim, retryAt time.Time) error

	// ReclaimStaleExpired returns timers whose lease expired and reports how many.
	ReclaimStaleExpired(ctx context.Context, now time.Time) (int, error)

	// PopAndAllocate atomically reads the first user in the queue, checks their status,
	// removes them if applicable, and allocates available stock.
	PopAndAllocate(ctx context.Context, productID string) (userID string, allocated int, available int, soldOut bool, status models.MembershipStatus, score float64, err error)

	// Requeue atomically puts a user back into the queue at their original position (used for rollbacks).
	Requeue(ctx context.Context, productID string, userID string, score float64) error

	// RestoreProductState replaces one product's recovered stock and FIFO queue.
	RestoreProductState(ctx context.Context, productID string, productCount int, available int, queuedUserIDs []string) error

	// ResetExpiryTimers clears only expiration worker indexes before recovery recreates them.
	ResetExpiryTimers(ctx context.Context) error

	// GetQueueMetrics retrieves the user's 0-indexed rank in the queue and the currently available stock.
	// It uses a pipeline to minimize network round-trips for real-time ETA calculation.
	GetQueueMetrics(ctx context.Context, productID string, userID string) (rank int, availableUnits int, err error)
}

// AvitoClient defines the contract for interacting with the external AvitoBackend API.
// It is strictly used for physical stock synchronization and payment notifications.
type AvitoClient interface {
	// GetInitialStock fetches the physical total stock of a product during the first request.
	GetInitialStock(ctx context.Context, productID string) (int, error)

	// DecrementStock notifies AvitoBackend that an item has been permanently sold.
	DecrementStock(ctx context.Context, idempotencyKey string, productID string, quantity int) error
}
