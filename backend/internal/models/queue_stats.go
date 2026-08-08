package models

// QueueStats is the public picture of demand for a product: how many people are
// waiting, how many are mid-purchase, and how much is left.
//
// It serves two audiences at once. A seller sees whether it is worth restocking;
// a buyer sees that others are waiting too, which is what makes waiting feel
// reasonable rather than arbitrary.
type QueueStats struct {
	// Waiting is the number of users in the FIFO queue with nothing allocated yet.
	Waiting int
	// HoldingRight is how many users currently hold a purchase right.
	HoldingRight int
	// PendingOffer is how many users are deciding on a partial offer.
	PendingOffer int
	// Available is how many units can be handed out right now — units held by
	// rights and offers are excluded, so this is not the physical stock.
	Available int
	// ProductCount is the physical stock left, as mirrored from AvitoBackend.
	ProductCount int
}
