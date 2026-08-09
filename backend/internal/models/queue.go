package models

import "time"

// Status is the machine-readable state of a queue membership, as exposed by the API.
// Internal states (EXPIRED, USED) are not statuses: expiry shows up as QUEUED or
// DECLINED, a used right shows up as PURCHASED (see docs/design_context.md, п. 6).
type Status string

const (
	// StatusQueued means the user waits in the FIFO queue for units to free up.
	StatusQueued Status = "QUEUED"
	// StatusRightActive means the user holds a purchase right until ExpiresAt.
	StatusRightActive Status = "RIGHT_ACTIVE"
	// StatusOfferPending means fewer units are available than requested and the user must decide.
	StatusOfferPending Status = "OFFER_PENDING"
	// StatusDeclined means the user rejected an offer or did not react in time. Terminal.
	StatusDeclined Status = "DECLINED"
	// StatusPurchased means the right was paid for and used up. Terminal.
	StatusPurchased Status = "PURCHASED"
	// StatusSoldOut means the product is gone for good — restocking is out of the MVP scope.
	StatusSoldOut Status = "SOLD_OUT"
)

// Membership is one user's participation in the queue for one product.
// Which fields are meaningful depends on Status: Token/Quantity/ExpiresAt for
// RIGHT_ACTIVE, AvailableQuantity/ExpiresAt for OFFER_PENDING, nothing extra for
// the remaining statuses.
type Membership struct {
	ProductID string
	UserID    string
	Status    Status

	// Requested is how many units the user asked for on entry. Kept to decide what
	// to hand out when the FIFO queue reaches this user.
	Requested int

	Token             string
	Quantity          int
	AvailableQuantity int
	ExpiresAt         time.Time
}
