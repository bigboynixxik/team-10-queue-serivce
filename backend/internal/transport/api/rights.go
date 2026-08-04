package api

import (
	"net/http"
)

// rightEvents handles POST /rights/{token}/events, called by the Order Service.
// It reports an event rather than setting a status: the Order Service knows
// nothing about our internal state model.
func (h *QueueHandler) rightEvents(w http.ResponseWriter, r *http.Request) {
	var req rightEventRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	if req.Event != eventPaymentSucceeded {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if err := h.service.ReportPayment(r.Context(), r.PathValue("token")); err != nil {
		writeError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusAccepted)
}
