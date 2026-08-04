// Package api holds the HTTP handlers of the Queue Service — the controller layer.
// Handlers only parse requests, delegate to the service layer and shape responses;
// all queue rules live in internal/service.
package api

import (
	"context"
	"log/slog"
	"net/http"

	"backend/internal/models"
	"backend/internal/transport/mw"
)

// QueueService is what the controller layer needs from the business layer. It is
// declared here, on the consumer side, so the mock in internal/service/queue can be
// replaced by the real implementation without touching the handlers.
type QueueService interface {
	Join(ctx context.Context, productID, userID string, quantity int) (models.Membership, error)
	Status(ctx context.Context, productID, userID string) (models.Membership, error)
	AcceptOffer(ctx context.Context, productID, userID string, quantity int) (models.Membership, error)
	Leave(ctx context.Context, productID, userID string) error
	ReportPayment(ctx context.Context, token string) error
}

// QueueHandler serves the queue
// and rights endpoints.
type QueueHandler struct {
	service QueueService
}

// NewQueueHandler creates the handler over the given service.
func NewQueueHandler(service QueueService) *QueueHandler {
	return &QueueHandler{service: service}
}

// NewRouter wires the routes and the middleware chain.
//
// The /queue routes act on behalf of a user and therefore go through
// UserMiddleware; /rights/{token}/events is service-to-service (called by the
// Order Service, not by a browser) and /healthz is infrastructure — both skip it.
func NewRouter(h *QueueHandler, log *slog.Logger) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", health)

	user := http.NewServeMux()
	user.HandleFunc("POST /queue/{product_id}/members", h.join)
	user.HandleFunc("GET /queue/{product_id}/members/me", h.status)
	user.HandleFunc("PATCH /queue/{product_id}/members/me", h.acceptOffer)
	user.HandleFunc("DELETE /queue/{product_id}/members/me", h.leave)

	mux.Handle("/queue/", mw.UserMiddleware(user))

	mux.HandleFunc("POST /rights/{token}/events", h.rightEvents)

	return mw.LoggingMiddleware(log, mux.ServeHTTP)
}
