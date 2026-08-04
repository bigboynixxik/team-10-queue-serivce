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

// eventPaymentSucceeded is the only event the Order Service reports so far.
const eventPaymentSucceeded = "payment_succeeded"

// membershipResponse is the single response shape of the queue endpoints: the
// status decides which of the optional fields are present, exactly as the oneOf
// in docs/api.yml describes. There is no human-readable message on purpose — the
// frontend maps statuses to text itself.
type membershipResponse struct {
	Status            models.Status `json:"status"`
	Token             string        `json:"token,omitempty"`
	Quantity          int           `json:"quantity,omitempty"`
	AvailableQuantity int           `json:"available_quantity,omitempty"`
	ExpiresAt         *time.Time    `json:"expires_at,omitempty"`
}

func newMembershipResponse(m models.Membership) membershipResponse {
	resp := membershipResponse{
		Status:            m.Status,
		Token:             m.Token,
		Quantity:          m.Quantity,
		AvailableQuantity: m.AvailableQuantity,
	}
	if !m.ExpiresAt.IsZero() {
		expiresAt := m.ExpiresAt.UTC()
		resp.ExpiresAt = &expiresAt
	}

	return resp
}
