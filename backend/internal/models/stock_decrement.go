package models

import "time"

// StockDecrement is a durable outbox event for an AvitoBackend stock decrement.
type StockDecrement struct {
	ID            string
	RightToken    string
	OrderID       string
	ProductID     string
	Quantity      int
	Attempts      int
	NextAttemptAt time.Time
	LockedUntil   *time.Time
	DeliveredAt   *time.Time
	LastError     *string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}
