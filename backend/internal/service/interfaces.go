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

	// UpdateStockAndRightTx atomically marks a right as USED and decrements the product_stock.
	// This represents the final confirmation of a successful payment.
	UpdateStockAndRightTx(ctx context.Context, token string, orderID string, quantity int) error

	// SaveInitialStock persists the physical stock fetched from AvitoBackend.
	SaveInitialStock(ctx context.Context, stock *models.ProductStock) error
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

	// PublishEvent broadcasts a status change to connected WebSocket clients.
	PublishEvent(ctx context.Context, productID string, userID string, payload interface{}) error

	// AddToExpiryTimer sets up background tracking for a time-bound right or offer.
	AddToExpiryTimer(ctx context.Context, productID string, userID string, expiresAt time.Time) error

	// RemoveFromExpiryTimer removes a user's timer if they complete an action before expiration.
	RemoveFromExpiryTimer(ctx context.Context, productID string, userID string) error

	// RestoreAvailableUnits returns unused or rolled-back stock to the available pool.
	RestoreAvailableUnits(ctx context.Context, productID string, quantity int) error

	// GetFirstInQueue retrieves the first user ID from the queue without removing it.
	GetFirstInQueue(ctx context.Context, productID string) (string, error)

	// GetAndRemoveExpired atomically retrieves and removes items from the expiry timer that have timed out.
	GetAndRemoveExpired(ctx context.Context, now time.Time) ([]string, error)
}

// AvitoClient defines the contract for interacting with the external AvitoBackend API.
// It is strictly used for physical stock synchronization and payment notifications.
type AvitoClient interface {
	// GetInitialStock fetches the physical total stock of a product during the first request.
	GetInitialStock(ctx context.Context, productID string) (int, error)

	// DecrementStock notifies AvitoBackend that an item has been permanently sold.
	DecrementStock(ctx context.Context, productID string, quantity int) error
}
