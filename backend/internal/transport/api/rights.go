package api

import (
	"net/http"

	"backend/internal/transport/mw"
)

// validateRight handles GET /rights/{token} — the check the browser makes before
// going to AvitoBackend to create an order. This is what makes the queue
// unskippable: without an active right of your own there is no way past it
// (docs/design_context.md, п. 5.8; FR-C1/FR-C2).
//
// The check is deliberately ours rather than AvitoBackend's: invalid tokens are
// rejected here and never reach the checkout endpoint, which is the hot one.
func (h *QueueHandler) validateRight(w http.ResponseWriter, r *http.Request) {
	_, err := h.service.ValidateRight(r.Context(), r.PathValue("token"), mw.UserFromContext(r.Context()))
	if err != nil {
		writeError(w, r, err)
		return
	}

	writeJSON(w, r, http.StatusOK, validationResponse{Valid: true})
}

// validateRightForCheckout handles AvitoBackend's private pre-payment check.
// A browser can still open /checkout, but AvitoBackend refuses to create an
// order unless Queue Service says this token is active and bound to the product.
func (h *QueueHandler) validateRightForCheckout(w http.ResponseWriter, r *http.Request) {
	var req checkoutValidationRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.ProductID == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if _, err := h.service.ValidateRightForCheckout(r.Context(), r.PathValue("token"), req.ProductID); err != nil {
		writeError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// rightEvents handles POST /rights/{token}/events, called by AvitoBackend.
// It reports an event rather than setting a status: AvitoBackend knows nothing
// about our internal state model.
func (h *QueueHandler) rightEvents(w http.ResponseWriter, r *http.Request) {
	var req rightEventRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	if req.Event != eventPaymentSucceeded {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if err := h.service.ProcessPayment(r.Context(), r.PathValue("token"), req.OrderID); err != nil {
		writeError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusAccepted)
}
