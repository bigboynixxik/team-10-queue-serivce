// Package api holds the HTTP handlers of the Queue Service — the controller layer.
// Handlers only parse requests, delegate to the service layer and shape responses;
// all queue rules live in internal/service.
package api

import (
	"log/slog"
	"net/http"

	"backend/internal/transport"
	"backend/internal/transport/mw"
)

// QueueHandler serves the queue and rights endpoints.
type QueueHandler struct {
	service transport.QueueService
}

// NewQueueHandler creates the handler over the given service.
func NewQueueHandler(service transport.QueueService) *QueueHandler {
	return &QueueHandler{service: service}
}

// NewRouter wires the routes and the middleware chain.
//
// Three groups with different callers, hence three different guards: /queue and
// GET /rights/{token} act on behalf of a user and go through UserMiddleware;
// POST /rights/{token}/events comes from AvitoBackend and is guarded by the
// shared secret; /healthz is infrastructure and is open.
func NewRouter(h *QueueHandler, log *slog.Logger, internalToken string) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", health)

	user := http.NewServeMux()
	user.HandleFunc("POST /queue/{product_id}/members", h.join)
	user.HandleFunc("GET /queue/{product_id}/members/me", h.status)
	user.HandleFunc("PATCH /queue/{product_id}/members/me", h.acceptOffer)
	user.HandleFunc("DELETE /queue/{product_id}/members/me", h.leave)

	mux.Handle("/queue/", mw.UserMiddleware(user))

	mux.Handle("GET /rights/{token}", mw.UserMiddleware(http.HandlerFunc(h.validateRight)))
	mux.Handle("POST /rights/{token}/events", mw.InternalAuth(internalToken, http.HandlerFunc(h.rightEvents)))

	return mw.LoggingMiddleware(log, mux.ServeHTTP)
}
