package api

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"

	"backend/internal/models"
	"backend/internal/transport/mw"
	"backend/pkg/logger"
)

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

	events, closeSubscription, err := h.realtime.SubscribeUpdates(ctx, productID, userID)
	if err != nil {
		log.Error("websocket subscribe", "error", err)
		_ = conn.Close(websocket.StatusInternalError, "realtime subscription failed")
		return
	}
	defer func() {
		if errClose := closeSubscription(); errClose != nil {
			log.Debug("websocket unsubscribe", "error", errClose)
		}
	}()

	var sent membershipResponse

	sendCurrent := func() bool {
		queue, err := h.service.GetUserQueue(ctx, productID, userID)
		if err != nil {
			if errors.Is(err, models.ErrMembershipNotFound) || errors.Is(err, models.ErrTokenNotFound) {
				_ = conn.Close(websocket.StatusPolicyViolation, "membership not found")
			} else {
				log.Error("websocket status", "error", err)
				_ = conn.Close(websocket.StatusInternalError, "membership read failed")
			}
			return false
		}

		if queue == nil || queue.Membership == nil {
			_ = conn.Close(websocket.StatusPolicyViolation, "membership not found")
			return false
		}

		resp := newUserQueueResponse(queue).membershipResponse
		if !sameMembership(sent, resp) {
			if err := wsjson.Write(ctx, conn, resp); err != nil {
				log.Debug("websocket write", "error", err)
				return false
			}
			sent = resp
		}

		if isTerminal(queue.Membership.Status) {
			_ = conn.Close(websocket.StatusNormalClosure, "terminal status")
			return false
		}
		return true
	}

	refreshRightHeartbeat := func() {
		if sent.Status != models.MembershipStatusRightActive {
			return
		}

		if err := h.service.RefreshRightHeartbeat(ctx, productID, userID); err != nil {
			if isExpectedHeartbeatError(err) {
				log.Debug("websocket heartbeat not refreshed", "error", err)
				return
			}
			log.Error("websocket heartbeat refresh", "error", err)
		}
	}

	probeConnection := func() bool {
		pingCtx, cancel := context.WithTimeout(ctx, h.heartbeatInterval)
		defer cancel()

		if err := conn.Ping(pingCtx); err != nil {
			if ctx.Err() == nil {
				log.Debug("websocket heartbeat timeout", "error", err)
				_ = conn.CloseNow()
			}
			return false
		}

		refreshRightHeartbeat()
		return true
	}

	if !sendCurrent() {
		return
	}

	// A successful WebSocket handshake already proves initial presence. Native
	// Ping/Pong probes keep extending the lease after this first refresh.
	refreshRightHeartbeat()
	heartbeatTicker := time.NewTicker(h.heartbeatInterval)
	defer heartbeatTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case _, ok := <-events:
			if !ok {
				_ = conn.Close(websocket.StatusInternalError, "realtime subscription closed")
				return
			}
			if !sendCurrent() {
				return
			}
		case <-heartbeatTicker.C:
			if !probeConnection() {
				return
			}
		}
	}
}

// sameMembership compares by value — ExpiresAt is a pointer, so == would compare
// addresses and every event would look like a change.
func sameMembership(a, b membershipResponse) bool {
	if a.Status != b.Status || a.Token != b.Token ||
		a.Quantity != b.Quantity || a.AvailableQuantity != b.AvailableQuantity ||
		a.Position != b.Position || a.ETASeconds != b.ETASeconds {
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

func isExpectedHeartbeatError(err error) bool {
	return errors.Is(err, models.ErrInvalidStatus) ||
		errors.Is(err, models.ErrTokenExpired) ||
		errors.Is(err, models.ErrMembershipNotFound) ||
		errors.Is(err, models.ErrTokenNotFound)
}

func isTerminal(status models.MembershipStatus) bool {
	return status == models.MembershipStatusPurchased ||
		status == models.MembershipStatusDeclined ||
		status == models.MembershipStatusSoldOut
}
