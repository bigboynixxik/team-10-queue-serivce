// Package api holds the HTTP handlers of the Queue Service — the controller layer.
// Handlers only parse requests, delegate to the service layer and shape responses;
// all queue rules live in internal/service.
package api

import (
	"log/slog"
	"net/http"
	"time"

	"backend/internal/transport"
	"backend/internal/transport/mw"
)

// QueueHandler serves the queue and rights endpoints.
type QueueHandler struct {
	service              transport.QueueService
	realtime             transport.RealtimeSubscriber
	presencePingInterval time.Duration
}

// NewQueueHandler creates the handler over the service and realtime event source.
func NewQueueHandler(
	service transport.QueueService,
	realtime transport.RealtimeSubscriber,
	presencePingInterval time.Duration,
) *QueueHandler {
	return &QueueHandler{service: service, realtime: realtime, presencePingInterval: presencePingInterval}
}

// APIPrefix versions the public API. Everything a client calls lives behind it,
// so a breaking change can ship as /api/v2 while v1 keeps serving old clients.
//
// /healthz stays outside: it is infrastructure, not API, and the container
// healthcheck must not break when the API version changes.
const APIPrefix = "/api/v1"

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
	user.HandleFunc("POST "+APIPrefix+"/queue/{product_id}/members", h.join)
	user.HandleFunc("GET "+APIPrefix+"/queue/{product_id}/members/me", h.status)
	user.HandleFunc("PATCH "+APIPrefix+"/queue/{product_id}/members/me", h.acceptOffer)
	user.HandleFunc("DELETE "+APIPrefix+"/queue/{product_id}/members/me", h.leave)

	mux.Handle(APIPrefix+"/queue/", mw.UserMiddleware(user))

	// Public: head counts only, nothing tied to a person. Registered on the outer
	// mux so it stays outside UserMiddleware — the more specific pattern wins over
	// the /queue/ prefix above.
	mux.HandleFunc("GET "+APIPrefix+"/queue/{product_id}/stats", h.queueStats)

	// Public seller analytics endpoint
	mux.HandleFunc("GET "+APIPrefix+"/seller/products/{product_id}/metrics", h.productMetrics)

	// Acts on behalf of a user, so it goes through UserMiddleware like /queue.
	mux.Handle("GET "+APIPrefix+"/me/queues", mw.UserMiddleware(http.HandlerFunc(h.userQueues)))

	mux.Handle("GET "+APIPrefix+"/rights/{token}",
		mw.UserMiddleware(http.HandlerFunc(h.validateRight)))
	mux.Handle("POST "+APIPrefix+"/internal/rights/{token}/validate",
		mw.InternalAuth(internalToken, http.HandlerFunc(h.validateRightForCheckout)))
	mux.Handle("POST "+APIPrefix+"/rights/{token}/events",
		mw.InternalAuth(internalToken, http.HandlerFunc(h.rightEvents)))

	return mw.LoggingMiddleware(log, mux.ServeHTTP)
}
