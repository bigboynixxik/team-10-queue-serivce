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

// checkoutValidationRequest is AvitoBackend's server-side guard before it creates an order.
type checkoutValidationRequest struct {
	ProductID string `json:"product_id"`
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
	Position          int                     `json:"position,omitempty"`
	ETASeconds        int                     `json:"eta_seconds,omitempty"`
}

// userQueueResponse is one row of the «Мои очереди» list: the same membership
// shape the single-queue endpoint returns, plus the product it belongs to.
type userQueueResponse struct {
	ProductID string `json:"product_id"`
	membershipResponse
}

func newUserQueueResponse(q *models.UserQueue) userQueueResponse {
	if q == nil || q.Membership == nil {
		return userQueueResponse{}
	}

	resp := newMembershipResponse(q.Membership)
	resp.Position = q.Position
	resp.ETASeconds = int(q.ETA.Seconds())

	return userQueueResponse{ProductID: q.Membership.ProductID, membershipResponse: resp}
}

func newUserQueuesResponse(queues []*models.UserQueue) []userQueueResponse {
	out := make([]userQueueResponse, 0, len(queues))
	for _, q := range queues {
		out = append(out, newUserQueueResponse(q))
	}

	return out
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

// queueLimitReachedCode identifies the refusal in the body, since the queue
// limit and SOLD_OUT share the same 409 status.
const queueLimitReachedCode = "queue_limit_reached"

// queueLimitResponse is the body of a 409 caused by the per-user queue limit.
// The limit itself is included so the client can state it without knowing the
// server configuration.
type queueLimitResponse struct {
	Error string `json:"error"`
	Limit int    `json:"limit"`
}

// statsResponse is the body of GET /api/v1/queue/{product_id}/stats. All fields
// are always present — a zero is meaningful here, unlike in membershipResponse
// where an absent field means "not applicable to this status".
type statsResponse struct {
	Waiting      int `json:"waiting"`
	HoldingRight int `json:"holding_right"`
	PendingOffer int `json:"pending_offer"`
	Available    int `json:"available"`
	ProductCount int `json:"product_count"`
}

func newStatsResponse(s *models.QueueStats) statsResponse {
	if s == nil {
		return statsResponse{}
	}

	return statsResponse{
		Waiting:      s.Waiting,
		HoldingRight: s.HoldingRight,
		PendingOffer: s.PendingOffer,
		Available:    s.Available,
		ProductCount: s.ProductCount,
	}
}

// productMetricsResponse defines the JSON structure for seller analytics.
type productMetricsResponse struct {
	TotalStock         int  `json:"total_stock"`
	TotalContenders    int  `json:"total_contenders"`
	UsedRightsCount    int  `json:"used_rights_count"`
	ExpiredRightsCount int  `json:"expired_rights_count"`
	SoldOutCount       int  `json:"soldout_count"`
	DropOffCount       int  `json:"dropoff_count"`
	AvgPaymentTime     *int `json:"avg_payment_time"`
	AvgDropOffTime     *int `json:"avg_dropoff_time"`
}

// newProductMetricsResponse converts the domain model into the transport DTO.
func newProductMetricsResponse(m *models.ProductMetrics) productMetricsResponse {
	resp := productMetricsResponse{
		TotalStock:         m.TotalStock,
		TotalContenders:    m.TotalContenders,
		UsedRightsCount:    m.UsedRightsCount,
		ExpiredRightsCount: m.ExpiredRightsCount,
		SoldOutCount:       m.SoldOutCount,
		DropOffCount:       m.DropOffCount,
	}

	if m.AvgPaymentTime != nil {
		resp.AvgPaymentTime = new(int)
		*resp.AvgPaymentTime = int(m.AvgPaymentTime.Seconds())
	}

	if m.AvgDropOffTime != nil {
		resp.AvgDropOffTime = new(int)
		*resp.AvgDropOffTime = int(m.AvgDropOffTime.Seconds())
	}

	return resp
}
