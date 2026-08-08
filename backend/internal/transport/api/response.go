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
//
// The three token errors collapse into one 404 on purpose: "expired", "used" and
// "never existed" are the same answer to the caller — this right cannot be used —
// and telling them apart would leak whether a token exists at all.
func writeError(w http.ResponseWriter, r *http.Request, err error) {
	log := logger.FromContext(r.Context())

	switch {
	case errors.Is(err, models.ErrQuantityInvalid), errors.Is(err, models.ErrQuantityExceeded):
		w.WriteHeader(http.StatusBadRequest)
	case errors.Is(err, models.ErrForbidden):
		w.WriteHeader(http.StatusForbidden)
	case errors.Is(err, models.ErrNoPendingOffer), errors.Is(err, models.ErrInvalidStatus):
		w.WriteHeader(http.StatusConflict)
	case errors.Is(err, models.ErrStockDepleted):
		writeJSON(w, r, http.StatusConflict, membershipResponse{Status: models.MembershipStatusSoldOut})
	case errors.Is(err, models.ErrMembershipNotFound),
		errors.Is(err, models.ErrTokenNotFound),
		errors.Is(err, models.ErrTokenExpired),
		errors.Is(err, models.ErrTokenUsed):
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
