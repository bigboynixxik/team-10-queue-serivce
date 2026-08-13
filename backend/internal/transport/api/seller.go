package api

import (
	"net/http"
)

// productMetrics handles requests for historical and real-time product analytics.
// It relies on the service layer to process data and writeError to handle domain errors.
func (h *QueueHandler) productMetrics(w http.ResponseWriter, r *http.Request) {
	productID := r.PathValue("product_id")

	metrics, err := h.service.GetProductMetrics(r.Context(), productID)
	if err != nil {
		writeError(w, r, err)
		return
	}

	writeJSON(w, r, http.StatusOK, newProductMetricsResponse(metrics))
}
