// Package models provides core domain entities and business logic errors.
// It is completely isolated from infrastructure, transport, and external dependencies.
package models

import (
	"errors"
	"fmt"
)

// Core business logic errors. These errors are returned by repositories and services,
// and should be mapped to appropriate HTTP status codes in the transport layer.
var (
	ErrQuantityInvalid  = errors.New("quantity must be greater than zero")
	ErrTokenNotFound    = errors.New("token not found")
	ErrTokenExpired     = errors.New("token is expired")
	ErrTokenUsed        = errors.New("token has already been used")
	ErrForbidden        = errors.New("access denied: token belongs to another user")
	ErrStockDepleted    = errors.New("product is sold out")
	ErrInvalidStatus    = errors.New("invalid status value")
	ErrQuantityExceeded = errors.New("requested quantity exceeds available offer")

	// Queue service errors.
	ErrMembershipNotFound = errors.New("membership not found")
	ErrNoPendingOffer     = errors.New("no pending offer")

	// ErrConcurrentJoin means another request for the same user is still deciding
	// their entry into the queue. Retrying is safe: the claim behind it expires.
	ErrConcurrentJoin = errors.New("concurrent join in progress")

	// ErrQueueLimitReached means the user already waits in as many queues as the
	// service allows. Leaving one of them frees a slot; retrying alone will not.
	ErrQueueLimitReached = errors.New("active queue limit reached")

	ErrProductNotFound = errors.New("product not found")

	ErrInvalidRequest = errors.New("invalid request parameters")
)

// QueueLimitError reports the configured limit alongside the refusal, so the
// client can say how many queues are allowed without hardcoding the number.
type QueueLimitError struct {
	Limit int
}

func (e *QueueLimitError) Error() string {
	return fmt.Sprintf("%s: %d", ErrQueueLimitReached, e.Limit)
}

// Unwrap keeps errors.Is(err, ErrQueueLimitReached) working for callers that do
// not care about the number.
func (e *QueueLimitError) Unwrap() error {
	return ErrQueueLimitReached
}

var (

	// Backward-compatible aliases.
	ErrInvalidQuantity = ErrQuantityInvalid
	ErrSoldOut         = ErrStockDepleted
	ErrRightNotFound   = ErrTokenNotFound
)
