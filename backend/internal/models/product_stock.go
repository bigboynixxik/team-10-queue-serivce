package models

import "time"

// ProductStock serves as a durable mirror of the physical stock available at AvitoBackend.
type ProductStock struct {
	ProductID    string
	ProductCount int
	TotalStock   int
	UpdatedAt    time.Time
}
