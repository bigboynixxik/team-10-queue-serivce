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
// except for SOLD_OUT, which the contract defines as a status-carrying 409, and
// for the queue limit, which shares that 409 and would otherwise be
// indistinguishable from it.
//
// The three token errors collapse into one 404 on purpose: "expired", "used" and
// "never existed" are the same answer to the caller — this right cannot be used —
// and telling them apart would leak whether a token exists at all.
func writeError(w http.ResponseWriter, r *http.Request, err error) {
	log := logger.FromContext(r.Context())

	var status int

	switch {
	case errors.Is(err, models.ErrQuantityInvalid),
		errors.Is(err, models.ErrQuantityExceeded),
		errors.Is(err, models.ErrInvalidRequest):
		status = http.StatusBadRequest
	case errors.Is(err, models.ErrForbidden):
		status = http.StatusForbidden
	case errors.Is(err, models.ErrNoPendingOffer), errors.Is(err, models.ErrInvalidStatus),
		errors.Is(err, models.ErrConcurrentJoin), errors.Is(err, models.ErrQueueLimitReached):
		status = http.StatusConflict
	case errors.Is(err, models.ErrStockDepleted):
		status = http.StatusConflict
	case errors.Is(err, models.ErrMembershipNotFound),
		errors.Is(err, models.ErrTokenNotFound),
		errors.Is(err, models.ErrTokenExpired),
		errors.Is(err, models.ErrTokenUsed),
		errors.Is(err, models.ErrProductNotFound):
		status = http.StatusNotFound
	default:
		status = http.StatusInternalServerError
	}

	if status == http.StatusInternalServerError {
		log.Error("unexpected error", "error", err, "status", status)
	} else {
		log.Info("domain error", "error", err, "status", status)
	}

	if errors.Is(err, models.ErrStockDepleted) {
		writeJSON(w, r, status, membershipResponse{Status: models.MembershipStatusSoldOut})
		return
	}

	var limitErr *models.QueueLimitError
	if errors.As(err, &limitErr) {
		writeJSON(w, r, status, queueLimitResponse{
			Error: queueLimitReachedCode,
			Limit: limitErr.Limit,
		})

		return
	}

	w.WriteHeader(status)
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
