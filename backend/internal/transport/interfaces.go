// Package transport defines the HTTP/WebSocket delivery mechanisms and external contracts.
package transport

import (
	"context"
	"time"

	"backend/internal/models"
)

// QueueService defines the strict business logic contract required by the transport layer.
type QueueService interface {
	// JoinQueue processes a user's request to buy a product, placing them in the queue
	// or immediately issuing an offer/right depending on stock availability.
	JoinQueue(ctx context.Context, productID, userID string, quantity int) (*models.QueueMembership, *models.Right, error)

	// GetMembership returns the user's current state in the queue.
	GetMembership(ctx context.Context, productID, userID string) (*models.QueueMembership, error)

	// GetQueueStats reports public demand for a product: how many people wait,
	// how many are mid-purchase, and how much stock is left.
	GetQueueStats(ctx context.Context, productID string) (*models.QueueStats, error)

	// GetUserQueue returns one membership together with the user's place in it.
	GetUserQueue(ctx context.Context, productID, userID string) (*models.UserQueue, error)

	// GetUserQueues returns every queue the user takes part in, each with their
	// position and estimated wait.
	GetUserQueues(ctx context.Context, userID string) ([]*models.UserQueue, error)

	// AcceptOffer confirms a partial offer. The user can accept less than initially offered.
	// Any unused quantity is automatically returned to the pool for the next in line.
	AcceptOffer(ctx context.Context, productID, userID string, acceptedQuantity int) (*models.Right, error)

	// DeclineOffer rejects a pending offer. The reserved stock is returned to the pool,
	// and the queue is advanced.
	DeclineOffer(ctx context.Context, productID, userID string) error

	// LeaveQueue ends the user's participation, whether they are waiting,
	// considering a partial offer, or hold an active purchase right.
	LeaveQueue(ctx context.Context, productID, userID string) error

	// ValidateRight checks if a given token is valid, active, and belongs to the requesting user.
	ValidateRight(ctx context.Context, token, userID string) (*models.Right, error)

	// ValidateRightForCheckout checks if AvitoBackend may create an order for this token and product.
	ValidateRightForCheckout(ctx context.Context, token, productID string) (*models.Right, error)

	// ProcessPayment confirms a successful purchase, durably updating stock and invalidating the token.
	ProcessPayment(ctx context.Context, token, orderID string) error

	// AdvanceQueue acts as an internal engine to push the queue forward when stock frees up.
	// It is typically called internally after declines, expirations, or partial accepts.
	AdvanceQueue(ctx context.Context, productID string) error

	// CalculateETA computes the user's human-readable position in the queue (1-indexed)
	// and the estimated wait time in seconds before they receive an offer or right.
	CalculateETA(ctx context.Context, productID string, userID string) (position int, etaSeconds time.Duration, err error)

	// RefreshRightHeartbeat confirms that the holder of an active purchase right
	// still has a live WebSocket connection.
	RefreshRightHeartbeat(ctx context.Context, productID string, userID string) error
}

// RealtimeSubscriber provides transport-level invalidation signals without
// exposing Redis-specific Pub/Sub types to the WebSocket handler.
type RealtimeSubscriber interface {
	SubscribeUpdates(
		ctx context.Context,
		productID string,
		userID string,
	) (events <-chan struct{}, closeSubscription func() error, err error)
}
