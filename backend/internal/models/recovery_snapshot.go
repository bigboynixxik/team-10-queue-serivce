package models

// RecoverySnapshot is the durable state used to rebuild Redis after a restart.
// PostgreSQL remains the source of truth; Redis is reconstructed from these rows.
type RecoverySnapshot struct {
	Stocks      []*ProductStock
	Memberships []*QueueMembership
	Rights      []*Right
}
