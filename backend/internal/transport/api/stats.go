package api

import "net/http"

// queueStats handles GET /api/v1/queue/{product_id}/stats.
//
// Public on purpose, and it serves two readers at once. A seller sees whether
// demand justifies restocking. A buyer sees that others are waiting too — which
// is what turns an opaque wait into a reasonable one (UR-1).
//
// Nothing here identifies anyone: only head counts, no user ids, so there is
// nothing to protect behind an auth we do not have (FR-D1).
func (h *QueueHandler) queueStats(w http.ResponseWriter, r *http.Request) {
	stats, err := h.service.GetQueueStats(r.Context(), r.PathValue("product_id"))
	if err != nil {
		writeError(w, r, err)
		return
	}

	writeJSON(w, r, http.StatusOK, newStatsResponse(stats))
}
