package models

import "errors"

var (
	// ErrMembershipNotFound means the user never entered the queue for this product.
	ErrMembershipNotFound = errors.New("membership not found")
	// ErrInvalidQuantity means the requested quantity is outside the allowed range.
	ErrInvalidQuantity = errors.New("invalid quantity")
	// ErrSoldOut means the product stock reached zero — a terminal state in the MVP.
	ErrSoldOut = errors.New("product sold out")
	// ErrNoPendingOffer means the operation requires the OFFER_PENDING state.
	ErrNoPendingOffer = errors.New("no pending offer")
	// ErrRightNotFound means no active purchase right matches the given token.
	ErrRightNotFound = errors.New("right not found")
)
