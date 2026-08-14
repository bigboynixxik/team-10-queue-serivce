package models

import "time"

// UserQueue is one row of the «Мои очереди» screen: the user's membership plus
// where they stand in that particular queue.
//
// Position and ETA are kept next to the membership rather than inside it because
// they are derived, not stored: the membership lives in Postgres, the position is
// computed from the Redis queue at read time.
type UserQueue struct {
	Membership *QueueMembership
	// Position is 1-indexed and meaningful only while the user is QUEUED —
	// zero everywhere else, because someone holding a right is no longer waiting.
	Position int
	// ETA is the estimated wait before this user gets an offer or a right.
	ETA time.Duration
}
