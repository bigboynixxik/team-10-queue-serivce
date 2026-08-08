package models

import "time"

// QueueMembership represents a user's position and current state within a product's queue.
// It reflects what the user sees on their screen (e.g., waiting, holding an offer, etc.).
type QueueMembership struct {
	ID                int64
	ProductID         string
	UserID            string
	Status            MembershipStatus
	Quantity          int
	AvailableQuantity *int       // Used only for OFFER_PENDING state
	CurrentToken      *string    // Used when the user holds an ACTIVE right
	ExpiresAt         *time.Time // Nil if the user is just QUEUED without an active offer/right
	CreatedAt         time.Time
	UpdatedAt         time.Time
}
