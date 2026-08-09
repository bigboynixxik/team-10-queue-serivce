package api

import (
	"net/http"

	"backend/internal/transport/mw"
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
