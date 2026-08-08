package models

// RightStatus represents the lifecycle state of a purchased or issued right.
type RightStatus string

// Allowed values for RightStatus.
const (
	RightStatusActive  RightStatus = "ACTIVE"
	RightStatusExpired RightStatus = "EXPIRED"
	RightStatusUsed    RightStatus = "USED"
)

// Valid checks if the RightStatus contains a recognized value.
func (s RightStatus) Valid() error {
	switch s {
	case RightStatusActive, RightStatusExpired, RightStatusUsed:
		return nil
	default:
		return ErrInvalidStatus
	}
}

// MembershipStatus represents a user's current state within the queue mechanism.
type MembershipStatus string

// Allowed values for MembershipStatus.
const (
	MembershipStatusQueued       MembershipStatus = "QUEUED"
	MembershipStatusRightActive  MembershipStatus = "RIGHT_ACTIVE"
	MembershipStatusOfferPending MembershipStatus = "OFFER_PENDING"
	MembershipStatusDeclined     MembershipStatus = "DECLINED"
	MembershipStatusPurchased    MembershipStatus = "PURCHASED"
	MembershipStatusSoldOut      MembershipStatus = "SOLD_OUT"
)

// Valid checks if the MembershipStatus contains a recognized value.
func (s MembershipStatus) Valid() error {
	switch s {
	case MembershipStatusQueued, MembershipStatusRightActive,
		MembershipStatusOfferPending, MembershipStatusDeclined,
		MembershipStatusPurchased, MembershipStatusSoldOut:
		return nil
	default:
		return ErrInvalidStatus
	}
}
