package models

import "time"

// ExpiryClaim identifies one concrete lease of a scheduled expiration.
// Deadline is the original timer deadline; LeaseUntil fences acknowledgements
// from workers whose lease has already expired and been reassigned.
type ExpiryClaim struct {
	Key        string
	Deadline   time.Time
	LeaseUntil time.Time
}
