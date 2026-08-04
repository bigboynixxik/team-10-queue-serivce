package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"backend/internal/models"
	"backend/pkg/logger"
)

func writeJSON(w http.ResponseWriter, r *http.Request, code int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)

	if err := json.NewEncoder(w).Encode(body); err != nil {
		logger.FromContext(r.Context()).Error("write response", "error", err)
	}
}

// writeError maps a domain error onto an HTTP status code. Bodies are empty
// except for SOLD_OUT, which the contract defines as a status-carrying 409.
func writeError(w http.ResponseWriter, r *http.Request, err error) {
	log := logger.FromContext(r.Context())

	switch {
	case errors.Is(err, models.ErrInvalidQuantity):
		w.WriteHeader(http.StatusBadRequest)
	case errors.Is(err, models.ErrNoPendingOffer):
		w.WriteHeader(http.StatusConflict)
	case errors.Is(err, models.ErrSoldOut):
		writeJSON(w, r, http.StatusConflict, membershipResponse{Status: models.StatusSoldOut})
	case errors.Is(err, models.ErrMembershipNotFound), errors.Is(err, models.ErrRightNotFound):
		w.WriteHeader(http.StatusNotFound)
	default:
		log.Error("unexpected error", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
	}
}

// decodeJSON answers 400 itself and reports whether the caller may continue.
func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		logger.FromContext(r.Context()).Debug("decode request body", "error", err)
		w.WriteHeader(http.StatusBadRequest)

		return false
	}

	return true
}
