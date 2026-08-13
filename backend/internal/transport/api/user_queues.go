package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"

	"backend/internal/transport/mw"
	"backend/pkg/logger"
)

// userQueues handles GET /api/v1/me/queues — every queue the caller takes part
// in, with their position in each.
//
// Two modes on one resource, the same way /members/me serves both a read and a
// WebSocket: a plain request answers with JSON, and Accept: text/event-stream
// turns it into a live feed. EventSource sets that header itself, so the
// frontend just points it at this URL.
//
// The list carries no product names on purpose — the catalogue belongs to
// AvitoBackend, and Queue Service has no business mirroring it.
func (h *QueueHandler) userQueues(w http.ResponseWriter, r *http.Request) {
	userID := mw.UserFromContext(r.Context())

	if isWebSocketUpgrade(r) {
		h.streamUserQueues(w, r, userID)
		return
	}

	if wantsSSE(r) {
		streamJSON(w, r, func() (any, error) {
			queues, err := h.service.GetUserQueues(r.Context(), userID)
			if err != nil {
				return nil, err
			}

			return newUserQueuesResponse(queues), nil
		})

		return
	}

	queues, err := h.service.GetUserQueues(r.Context(), userID)
	if err != nil {
		writeError(w, r, err)
		return
	}

	writeJSON(w, r, http.StatusOK, newUserQueuesResponse(queues))
}

// streamUserQueues keeps one connection for the user's whole application
// session. Successful protocol Pong frames extend only QUEUED memberships.
func (h *QueueHandler) streamUserQueues(w http.ResponseWriter, r *http.Request, userID string) {
	log := logger.FromContext(r.Context())
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
	if err != nil {
		log.Error("user queues websocket accept", "error", err)
		return
	}
	defer func() { _ = conn.CloseNow() }()

	ctx := conn.CloseRead(r.Context())
	var sent []byte

	sendCurrent := func() bool {
		queues, errQueues := h.service.GetUserQueues(ctx, userID)
		if errQueues != nil {
			log.Error("user queues websocket read", "error", errQueues)
			_ = conn.Close(websocket.StatusInternalError, "queues read failed")
			return false
		}

		payload := newUserQueuesResponse(queues)
		encoded, errEncode := json.Marshal(payload)
		if errEncode != nil {
			log.Error("user queues websocket encode", "error", errEncode)
			return false
		}
		if bytes.Equal(sent, encoded) {
			return true
		}
		if errWrite := wsjson.Write(ctx, conn, payload); errWrite != nil {
			log.Debug("user queues websocket write", "error", errWrite)
			return false
		}
		sent = encoded
		return true
	}

	refreshPresence := func() bool {
		if errRefresh := h.service.RefreshUserPresence(ctx, userID); errRefresh != nil {
			log.Error("user presence refresh", "error", errRefresh)
			return false
		}
		return true
	}

	probeConnection := func() bool {
		pingCtx, cancel := context.WithTimeout(ctx, h.presencePingInterval)
		defer cancel()
		if errPing := conn.Ping(pingCtx); errPing != nil {
			if ctx.Err() == nil {
				log.Debug("user presence ping timeout", "error", errPing)
			}
			return false
		}
		return refreshPresence()
	}

	if !refreshPresence() || !sendCurrent() {
		return
	}

	heartbeats := time.NewTicker(h.presencePingInterval)
	defer heartbeats.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-heartbeats.C:
			if !probeConnection() {
				return
			}
			if !sendCurrent() {
				return
			}
		}
	}
}
