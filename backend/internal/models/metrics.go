package models

import "time"

// ProductMetrics aggregates historical and real-time demand data for a product.
// It is used by the seller dashboard to analyze conversion rates, drop-offs,
// and overall scarcity.
type ProductMetrics struct {
	// TotalStock is the initial physical quantity listed for sale.
	TotalStock int

	// TotalContenders is the number of unique users who attempted to join the queue.
	TotalContenders int

	// UsedRightsCount is the number of rights successfully paid for (status USED).
	UsedRightsCount int

	// ExpiredRightsCount is the number of issued rights that timed out (status EXPIRED).
	ExpiredRightsCount int

	// SoldOutCount is the number of users who reached the front but the stock was 0.
	SoldOutCount int

	// DropOffCount is the number of users whose latest participation ended without a purchase.
	DropOffCount int

	// AvgPaymentTime is the average duration users spend completing their payment.
	AvgPaymentTime *time.Duration

	// AvgDropOffTime is the average duration of latest participations that ended without a purchase.
	AvgDropOffTime *time.Duration
}
