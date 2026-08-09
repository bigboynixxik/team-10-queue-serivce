package api

import (
	"net/http"
	"strings"

	"backend/internal/models"
	"backend/internal/transport/mw"
)

// join handles POST /queue/{product_id}/members. Repeated calls by the same user
// are idempotent — the frontend may safely retry.
func (h *QueueHandler) join(w http.ResponseWriter, r *http.Request) {
	var req joinRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	membership, right, err := h.service.JoinQueue(
		r.Context(), r.PathValue("product_id"), mw.UserFromContext(r.Context()), req.Quantity,
	)
	if err != nil {
		writeError(w, r, err)
		return
	}

	resp := newMembershipResponse(membership)

	// The service reports a sold-out product as a status rather than an error,
	// but the contract answers it with 409 — nothing was created.
	if resp.Status == models.MembershipStatusSoldOut {
		writeJSON(w, r, http.StatusConflict, membershipResponse{Status: models.MembershipStatusSoldOut})
		return
	}

	// A right issued straight away may not be reflected in the membership the
	// service returned, so the token comes from the right itself.
	if right != nil && resp.Token == "" {
		resp.Token = right.Token
	}

	writeJSON(w, r, http.StatusCreated, resp)
}

// status handles GET /queue/{product_id}/members/me. One resource, three modes:
// a plain read, a WebSocket (Upgrade header) and an SSE feed (Accept header).
// The two realtime modes coexist so that adding SSE breaks no existing client.
func (h *QueueHandler) status(w http.ResponseWriter, r *http.Request) {
	if isWebSocketUpgrade(r) {
		h.stream(w, r)
		return
	}

	productID := r.PathValue("product_id")
	userID := mw.UserFromContext(r.Context())

	view := func() (any, error) {
		queue, err := h.service.GetUserQueue(r.Context(), productID, userID)
		if err != nil {
			return nil, err
		}

		return newUserQueueResponse(queue).membershipResponse, nil
	}

	if wantsSSE(r) {
		streamJSON(w, r, view)
		return
	}

	resp, err := view()
	if err != nil {
		writeError(w, r, err)
		return
	}

	writeJSON(w, r, http.StatusOK, resp)
}

// acceptOffer handles PATCH /queue/{product_id}/members/me.
func (h *QueueHandler) acceptOffer(w http.ResponseWriter, r *http.Request) {
	var req acceptRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	right, err := h.service.AcceptOffer(
		r.Context(), r.PathValue("product_id"), mw.UserFromContext(r.Context()), req.Quantity,
	)
	if err != nil {
		writeError(w, r, err)
		return
	}

	writeJSON(w, r, http.StatusOK, newRightResponse(right))
}

// leave handles DELETE /queue/{product_id}/members/me.
func (h *QueueHandler) leave(w http.ResponseWriter, r *http.Request) {
	err := h.service.LeaveQueue(r.Context(), r.PathValue("product_id"), mw.UserFromContext(r.Context()))
	if err != nil {
		writeError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func isWebSocketUpgrade(r *http.Request) bool {
	return strings.EqualFold(r.Header.Get("Upgrade"), "websocket")
}
