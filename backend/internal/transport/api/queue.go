package api

import (
	"net/http"
	"strings"

	"backend/internal/transport/mw"
)

// join handles POST /queue/{product_id}/members. Repeated calls by the same user
// are idempotent — the frontend may safely retry.
func (h *QueueHandler) join(w http.ResponseWriter, r *http.Request) {
	var req joinRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	m, err := h.service.Join(r.Context(), r.PathValue("product_id"), mw.UserFromContext(r.Context()), req.Quantity)
	if err != nil {
		writeError(w, r, err)
		return
	}

	writeJSON(w, r, http.StatusCreated, newMembershipResponse(m))
}

// status handles GET /queue/{product_id}/members/me. One resource serves both the
// polling fallback and the realtime channel, switched by the Upgrade header.
func (h *QueueHandler) status(w http.ResponseWriter, r *http.Request) {
	if isWebSocketUpgrade(r) {
		h.stream(w, r)
		return
	}

	m, err := h.service.Status(r.Context(), r.PathValue("product_id"), mw.UserFromContext(r.Context()))
	if err != nil {
		writeError(w, r, err)
		return
	}

	writeJSON(w, r, http.StatusOK, newMembershipResponse(m))
}

// acceptOffer handles PATCH /queue/{product_id}/members/me.
func (h *QueueHandler) acceptOffer(w http.ResponseWriter, r *http.Request) {
	var req acceptRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	m, err := h.service.AcceptOffer(r.Context(), r.PathValue("product_id"), mw.UserFromContext(r.Context()), req.Quantity)
	if err != nil {
		writeError(w, r, err)
		return
	}

	writeJSON(w, r, http.StatusOK, newMembershipResponse(m))
}

// leave handles DELETE /queue/{product_id}/members/me.
func (h *QueueHandler) leave(w http.ResponseWriter, r *http.Request) {
	err := h.service.Leave(r.Context(), r.PathValue("product_id"), mw.UserFromContext(r.Context()))
	if err != nil {
		writeError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func isWebSocketUpgrade(r *http.Request) bool {
	return strings.EqualFold(r.Header.Get("Upgrade"), "websocket")
}
