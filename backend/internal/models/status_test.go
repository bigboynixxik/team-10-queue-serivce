package models_test

import (
	"testing"

	"backend/internal/models"

	"github.com/stretchr/testify/assert"
)

func TestRightStatus_Valid(t *testing.T) {
	tests := []struct {
		name        string
		status      models.RightStatus
		expectedErr error
	}{
		{"Valid: ACTIVE", models.RightStatusActive, nil},
		{"Valid: EXPIRED", models.RightStatusExpired, nil},
		{"Valid: USED", models.RightStatusUsed, nil},
		{"Invalid: Empty string", "", models.ErrInvalidStatus},
		{"Invalid: Lowercase", "active", models.ErrInvalidStatus},
		{"Invalid: Unknown status", "PENDING", models.ErrInvalidStatus},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.status.Valid()
			if tt.expectedErr != nil {
				assert.ErrorIs(t, err, tt.expectedErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestMembershipStatus_Valid(t *testing.T) {
	tests := []struct {
		name        string
		status      models.MembershipStatus
		expectedErr error
	}{
		{"Valid: QUEUED", models.MembershipStatusQueued, nil},
		{"Valid: RIGHT_ACTIVE", models.MembershipStatusRightActive, nil},
		{"Valid: OFFER_PENDING", models.MembershipStatusOfferPending, nil},
		{"Valid: DECLINED", models.MembershipStatusDeclined, nil},
		{"Valid: PURCHASED", models.MembershipStatusPurchased, nil},
		{"Valid: SOLD_OUT", models.MembershipStatusSoldOut, nil},
		{"Invalid: Empty string", "", models.ErrInvalidStatus},
		{"Invalid: Lowercase", "queued", models.ErrInvalidStatus},
		{"Invalid: Random string", "UNKNOWN", models.ErrInvalidStatus},
		{"Invalid: Partial match", "RIGHT", models.ErrInvalidStatus},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.status.Valid()
			if tt.expectedErr != nil {
				assert.ErrorIs(t, err, tt.expectedErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
