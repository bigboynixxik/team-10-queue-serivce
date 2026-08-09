package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPayValidatesRightBeforeReportingPayment(t *testing.T) {
	var (
		mu               sync.Mutex
		validationCalled bool
		eventCalled      bool
		gotProductID     string
		gotToken         string
		gotHeader        string
	)

	queue := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()

		gotHeader = r.Header.Get(internalTokenHeader)

		switch r.URL.Path {
		case "/api/v1/internal/rights/token-1/validate":
			validationCalled = true
			gotToken = "token-1"

			var body struct {
				ProductID string `json:"product_id"`
			}
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			gotProductID = body.ProductID
			w.WriteHeader(http.StatusNoContent)
		case "/api/v1/rights/token-1/events":
			require.True(t, validationCalled)
			eventCalled = true
			w.WriteHeader(http.StatusAccepted)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer queue.Close()

	srv := &server{
		queueBaseURL:  queue.URL,
		internalToken: "secret-token",
		http:          queue.Client(),
	}
	req := httptest.NewRequest(
		http.MethodPost,
		"/checkout/pay",
		bytes.NewBufferString(`{"token":"token-1","product_id":"product-1"}`),
	)
	rec := httptest.NewRecorder()

	srv.pay(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var body map[string]string
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
	require.NotEmpty(t, body["order_id"])

	mu.Lock()
	defer mu.Unlock()
	require.True(t, validationCalled)
	require.True(t, eventCalled)
	require.Equal(t, "token-1", gotToken)
	require.Equal(t, "product-1", gotProductID)
	require.Equal(t, "secret-token", gotHeader)
}

func TestPayDoesNotReportPaymentWhenValidationFails(t *testing.T) {
	var (
		mu          sync.Mutex
		eventCalled bool
	)

	queue := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()

		switch r.URL.Path {
		case "/api/v1/internal/rights/token-1/validate":
			w.WriteHeader(http.StatusNotFound)
		case "/api/v1/rights/token-1/events":
			eventCalled = true
			w.WriteHeader(http.StatusAccepted)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer queue.Close()

	srv := &server{
		queueBaseURL:  queue.URL,
		internalToken: "secret-token",
		http:          queue.Client(),
	}
	req := httptest.NewRequest(
		http.MethodPost,
		"/checkout/pay",
		bytes.NewBufferString(`{"token":"token-1","product_id":"product-1"}`),
	)
	rec := httptest.NewRecorder()

	srv.pay(rec, req)

	require.Equal(t, http.StatusConflict, rec.Code)

	mu.Lock()
	defer mu.Unlock()
	require.False(t, eventCalled)
}

func TestPatchStockRequiresIdempotencyKey(t *testing.T) {
	srv := &server{stock: newStock(3)}
	req := httptest.NewRequest(http.MethodPatch, "/products/prod-1/stock", bytes.NewBufferString(`{"decrement":1}`))
	rec := httptest.NewRecorder()

	srv.patchStock(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestPatchStockIdempotencyReplaysWithoutSecondDecrement(t *testing.T) {
	srv := &server{stock: newStock(3)}

	first := httptest.NewRequest(http.MethodPatch, "/products/prod-1/stock", bytes.NewBufferString(`{"decrement":1}`))
	first.SetPathValue("product_id", "prod-1")
	first.Header.Set(idempotencyKeyHeader, "event-1")
	firstRec := httptest.NewRecorder()
	srv.patchStock(firstRec, first)

	second := httptest.NewRequest(http.MethodPatch, "/products/prod-1/stock", bytes.NewBufferString(`{"decrement":1}`))
	second.SetPathValue("product_id", "prod-1")
	second.Header.Set(idempotencyKeyHeader, "event-1")
	secondRec := httptest.NewRecorder()
	srv.patchStock(secondRec, second)

	require.Equal(t, http.StatusOK, firstRec.Code)
	require.Equal(t, http.StatusOK, secondRec.Code)
	require.Equal(t, 2, srv.stock.get("prod-1"))
}

func TestPatchStockIdempotencyRejectsConflictingPayload(t *testing.T) {
	srv := &server{stock: newStock(3)}

	first := httptest.NewRequest(http.MethodPatch, "/products/prod-1/stock", bytes.NewBufferString(`{"decrement":1}`))
	first.SetPathValue("product_id", "prod-1")
	first.Header.Set(idempotencyKeyHeader, "event-1")
	firstRec := httptest.NewRecorder()
	srv.patchStock(firstRec, first)

	second := httptest.NewRequest(http.MethodPatch, "/products/prod-1/stock", bytes.NewBufferString(`{"decrement":2}`))
	second.SetPathValue("product_id", "prod-1")
	second.Header.Set(idempotencyKeyHeader, "event-1")
	secondRec := httptest.NewRecorder()
	srv.patchStock(secondRec, second)

	require.Equal(t, http.StatusOK, firstRec.Code)
	require.Equal(t, http.StatusConflict, secondRec.Code)
	require.Equal(t, 2, srv.stock.get("prod-1"))
}
