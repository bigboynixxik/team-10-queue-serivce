package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"backend/pkg/logger"
)

const (
	// sseContentType is what EventSource sends in Accept on its own, which is why
	// the frontend needs no special flag to ask for the streaming mode.
	sseContentType = "text/event-stream"

	// ssePollInterval matches the WebSocket loop: the stream re-reads state and
	// pushes only when it changed. The service already publishes every change to
	// a Redis channel, and switching this poll to that subscription is the one
	// optimisation this file is waiting for.
	ssePollInterval = time.Second

	// sseKeepAlive keeps proxies from closing an idle connection. A comment line
	// is invisible to EventSource but counts as traffic for everything in between.
	sseKeepAlive = 25 * time.Second
)

// wantsSSE reports whether the client asked for the streaming mode. This mirrors
// how the WebSocket mode is chosen by the Upgrade header — one resource, several
// representations, rather than a separate URL per transport.
func wantsSSE(r *http.Request) bool {
	return strings.Contains(r.Header.Get("Accept"), sseContentType)
}

// streamJSON pushes the result of next() whenever it changes, in Server-Sent
// Events format.
//
// next returns the payload and its comparable form; the stream writes only when
// that form differs from what the client already has. Comparing rendered JSON
// rather than the value itself keeps this helper indifferent to what it streams.
func streamJSON(w http.ResponseWriter, r *http.Request, next func() (any, error)) {
	log := logger.FromContext(r.Context())

	flusher, ok := w.(http.Flusher)
	if !ok {
		// Without flushing, every event would sit in the buffer until the handler
		// returns — which for a stream means never.
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", sseContentType)
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	// Tells nginx not to buffer the response; without it events arrive in bursts.
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	ticker := time.NewTicker(ssePollInterval)
	defer ticker.Stop()

	keepAlive := time.NewTicker(sseKeepAlive)
	defer keepAlive.Stop()

	ctx := r.Context()

	var sent string

	for {
		payload, err := next()
		if err != nil {
			writeSSEEvent(w, flusher, "error", `{"error":"unavailable"}`)
			log.Debug("sse source failed", "error", err)

			return
		}

		encoded, err := json.Marshal(payload)
		if err != nil {
			log.Error("sse encode", "error", err)
			return
		}

		if string(encoded) != sent {
			writeSSEEvent(w, flusher, "update", string(encoded))
			sent = string(encoded)
		}

		select {
		case <-ctx.Done():
			return
		case <-keepAlive.C:
			if _, err := fmt.Fprint(w, ": keep-alive\n\n"); err != nil {
				return
			}
			flusher.Flush()
		case <-ticker.C:
		}
	}
}

func writeSSEEvent(w http.ResponseWriter, flusher http.Flusher, event, data string) {
	if _, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, data); err != nil {
		return
	}

	flusher.Flush()
}
