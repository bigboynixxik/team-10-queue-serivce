// Package transport defines the HTTP/WebSocket delivery mechanisms and external contracts.
package transport

import (
	"context"

	"backend/internal/models"
)

// QueueService defines the strict business logic contract required by the transport layer.
type QueueService interface {
	// JoinQueue processes a user's request to buy a product, placing them in the queue
	// or immediately issuing an offer/right depending on stock availability.
	JoinQueue(ctx context.Context, productID, userID string, quantity int) (*models.QueueMembership, *models.Right, error)

	// GetMembership returns the user's current state in the queue. It serves both
	// the polling read and every push of the realtime channel.
	GetMembership(ctx context.Context, productID, userID string) (*models.QueueMembership, error)

	// AcceptOffer confirms a partial offer. The user can accept less than initially offered.
	// Any unused quantity is automatically returned to the pool for the next in line.
	AcceptOffer(ctx context.Context, productID, userID string, acceptedQuantity int) (*models.Right, error)

	// DeclineOffer rejects a pending offer. The reserved stock is returned to the pool,
	// and the queue is advanced.
	DeclineOffer(ctx context.Context, productID, userID string) error

	// ValidateRight checks if a given token is valid, active, and belongs to the requesting user.
	ValidateRight(ctx context.Context, token, userID string) (*models.Right, error)

	// ProcessPayment confirms a successful purchase, durably updating stock and invalidating the token.
	ProcessPayment(ctx context.Context, token, orderID string) error

	// AdvanceQueue acts as an internal engine to push the queue forward when stock frees up.
	// It is typically called internally after declines, expirations, or partial accepts.
	AdvanceQueue(ctx context.Context, productID string) error

	// CalculateETA computes the user's human-readable position in the queue (1-indexed)
	// and the estimated wait time in seconds before they receive an offer or right.
	CalculateETA(ctx context.Context, productID string, userID string) (position int, etaSeconds int, err error)
}
