package api

import (
	"time"

	"backend/internal/models"
)

// joinRequest is the body of POST /queue/{product_id}/members.
type joinRequest struct {
	Quantity int `json:"quantity"`
}

// acceptRequest is the body of PATCH /queue/{product_id}/members/me.
type acceptRequest struct {
	Quantity int `json:"quantity"`
}

// rightEventRequest is the body of POST /rights/{token}/events.
type rightEventRequest struct {
	Event   string `json:"event"`
	OrderID string `json:"order_id"`
}

// eventPaymentSucceeded is the only event AvitoBackend reports so far.
const eventPaymentSucceeded = "payment_succeeded"

// validationResponse is the body of GET /rights/{token}.
type validationResponse struct {
	Valid bool `json:"valid"`
}

// membershipResponse is the single response shape of the queue endpoints: the
// status decides which of the optional fields are present, exactly as the oneOf
// in docs/api.yml describes. There is no human-readable message on purpose — the
// frontend maps statuses to text itself.
type membershipResponse struct {
	Status            models.MembershipStatus `json:"status"`
	Token             string                  `json:"token,omitempty"`
	Quantity          int                     `json:"quantity,omitempty"`
	AvailableQuantity int                     `json:"available_quantity,omitempty"`
	ExpiresAt         *time.Time              `json:"expires_at,omitempty"`
}

func newMembershipResponse(m *models.QueueMembership) membershipResponse {
	if m == nil {
		return membershipResponse{}
	}

	resp := membershipResponse{
		Status:   m.Status,
		Quantity: m.Quantity,
	}

	if m.CurrentToken != nil {
		resp.Token = *m.CurrentToken
	}
	if m.AvailableQuantity != nil {
		resp.AvailableQuantity = *m.AvailableQuantity
	}
	if m.ExpiresAt != nil {
		expiresAt := m.ExpiresAt.UTC()
		resp.ExpiresAt = &expiresAt
	}

	return resp
}

// newRightResponse builds the answer to PATCH, where the service returns the
// issued right rather than the membership — the resulting state is fully
// described by the right itself.
func newRightResponse(r *models.Right) membershipResponse {
	if r == nil {
		return membershipResponse{}
	}

	expiresAt := r.ExpiresAt.UTC()

	return membershipResponse{
		Status:    models.MembershipStatusRightActive,
		Token:     r.Token,
		Quantity:  r.Quantity,
		ExpiresAt: &expiresAt,
	}
}
