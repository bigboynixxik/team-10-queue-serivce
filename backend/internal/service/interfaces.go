// Package service defines the core business logic and orchestrates operations
// between durable storage, cache, and external clients.
package service

import (
	"context"
	"time"

	"backend/internal/models"
)

// DurableRepo defines the contract for reliable, persistent storage (PostgreSQL).
// It acts as the source of truth and durable log, but does not handle race conditions.
type DurableRepo interface {
	// SaveRight persists a newly issued purchase right.
	SaveRight(ctx context.Context, right *models.Right) error

	// GetRightByToken retrieves a right by its unique token.
	GetRightByToken(ctx context.Context, token string) (*models.Right, error)

	// UpsertMembership creates or updates a user's current status in the queue.
	// It handles (product_id, user_id) conflicts gracefully.
	UpsertMembership(ctx context.Context, membership *models.QueueMembership) error

	// UseRightTx atomically locks an ACTIVE right, marks it as USED, and decrements
	// product_stock using the quantity stored with the right. transitioned is false
	// for an already processed webhook, so external side effects are not repeated.
	UseRightTx(ctx context.Context, token string, orderID string, now time.Time) (right *models.Right, transitioned bool, err error)

	// ExpireRightAndUpsertMembershipTx atomically marks an ACTIVE right as EXPIRED
	// and persists the corresponding terminal membership state.
	ExpireRightAndUpsertMembershipTx(ctx context.Context, token string, membership *models.QueueMembership) (right *models.Right, transitioned bool, err error)

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

	// SetRight caches an issued right for fast validation before checkout.
	SetRight(ctx context.Context, right *models.Right) error

	// GetRight retrieves a cached right by its token.
	GetRight(ctx context.Context, token string) (*models.Right, error)

	// ClaimJoin marks the start of an entry attempt and reports whether the caller
	// won it. Losing means a concurrent request for the same user is in flight.
	ClaimJoin(ctx context.Context, productID, userID string, ttl time.Duration) (bool, error)

	// ReleaseJoinClaim frees the claim once the entry is decided.
	ReleaseJoinClaim(ctx context.Context, productID, userID string) error

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
	ClaimExpired(ctx context.Context, now time.Time, lease time.Duration, limit int) ([]string, error)

	// AckExpired confirms that claimed timers were handled.
	AckExpired(ctx context.Context, keys []string) error

	// NackExpired returns claimed timers to the schedule after a failed attempt.
	NackExpired(ctx context.Context, keys []string, retryAt time.Time) error

	// ReclaimStaleExpired returns timers whose lease expired and reports how many.
	ReclaimStaleExpired(ctx context.Context, now time.Time) (int, error)

	// PopAndAllocate atomically reads the first user in the queue, checks their status,
	// removes them if applicable, and allocates available stock.
	PopAndAllocate(ctx context.Context, productID string) (userID string, allocated int, available int, soldOut bool, status models.MembershipStatus, score float64, err error)

	// Requeue atomically puts a user back into the queue at their original position (used for rollbacks).
	Requeue(ctx context.Context, productID string, userID string, score float64) error

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
	DecrementStock(ctx context.Context, productID string, quantity int) error
}
