package api

import (
	"errors"
	"net/http"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"

	"backend/internal/models"
	"backend/internal/transport/mw"
	"backend/pkg/logger"
)

// pollInterval is how often the loop re-reads the membership. The service also
// publishes every change to a Redis channel; switching this loop to that
// subscription is a drop-in change the client will not notice.
const pollInterval = time.Second

// stream serves the realtime mode of GET /queue/{product_id}/members/me.
func (h *QueueHandler) stream(w http.ResponseWriter, r *http.Request) {
	log := logger.FromContext(r.Context())

	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		// The frontend is served from another origin in the MVP; tightening this
		// belongs with real deployment settings.
		InsecureSkipVerify: true,
	})
	if err != nil {
		log.Error("websocket accept", "error", err)
		return
	}
	defer func() { _ = conn.CloseNow() }()

	productID := r.PathValue("product_id")
	userID := mw.UserFromContext(r.Context())

	// CloseRead keeps reading (and discarding) client frames so that a close from
	// the browser cancels ctx — the client never sends anything meaningful here.
	ctx := conn.CloseRead(r.Context())

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	var sent membershipResponse

	for {
		membership, err := h.service.GetMembership(ctx, productID, userID)
		if err != nil {
			if errors.Is(err, models.ErrMembershipNotFound) || errors.Is(err, models.ErrTokenNotFound) {
				_ = conn.Close(websocket.StatusPolicyViolation, "membership not found")
			} else {
				log.Error("websocket status", "error", err)
			}

			return
		}

		resp := newMembershipResponse(membership)
		if !sameMembership(sent, resp) {
			if err := wsjson.Write(ctx, conn, resp); err != nil {
				log.Debug("websocket write", "error", err)
				return
			}
			sent = resp
		}

		if isTerminal(membership.Status) {
			_ = conn.Close(websocket.StatusNormalClosure, "terminal status")
			return
		}

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// sameMembership compares by value — ExpiresAt is a pointer, so == would compare
// addresses and every tick would look like a change.
func sameMembership(a, b membershipResponse) bool {
	if a.Status != b.Status || a.Token != b.Token ||
		a.Quantity != b.Quantity || a.AvailableQuantity != b.AvailableQuantity {
		return false
	}

	switch {
	case a.ExpiresAt == nil && b.ExpiresAt == nil:
		return true
	case a.ExpiresAt == nil || b.ExpiresAt == nil:
		return false
	default:
		return a.ExpiresAt.Equal(*b.ExpiresAt)
	}
}

func isTerminal(status models.MembershipStatus) bool {
	return status == models.MembershipStatusPurchased ||
		status == models.MembershipStatusDeclined ||
		status == models.MembershipStatusSoldOut
}
