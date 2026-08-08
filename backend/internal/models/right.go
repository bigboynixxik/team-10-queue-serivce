package models

import "time"

// Right represents an exclusive, time-bound permission for a specific user
// to purchase a specific quantity of a product.
type Right struct {
	Token     string
	UserID    string
	ProductID string
	Quantity  int
	Status    RightStatus
	OrderID   *string // Nil until the payment succeeds and order is created
	CreatedAt time.Time
	ExpiresAt time.Time
	UsedAt    *time.Time // Nil until the right is successfully used
}
