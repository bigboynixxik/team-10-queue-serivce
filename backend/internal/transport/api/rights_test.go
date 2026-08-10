package api

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"backend/internal/models"

	"github.com/stretchr/testify/require"
)

func TestInternalCheckoutValidationCallsService(t *testing.T) {
	service := &realtimeQueueServiceStub{}
	realtime := &realtimeSubscriberStub{events: make(chan struct{})}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	server := httptest.NewServer(NewRouter(NewQueueHandler(service, realtime, time.Hour), log, "internal-token"))
	defer server.Close()

	req, err := http.NewRequest(
		http.MethodPost,
		server.URL+APIPrefix+"/internal/rights/token-1/validate",
		bytes.NewBufferString(`{"product_id":"product-1"}`),
	)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Internal-Token", "internal-token")

	resp, err := server.Client().Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	require.Equal(t, http.StatusNoContent, resp.StatusCode)
	calls, token, productID := service.checkoutCall()
	require.Equal(t, 1, calls)
	require.Equal(t, "token-1", token)
	require.Equal(t, "product-1", productID)
}

func TestInternalCheckoutValidationRejectsMissingInternalToken(t *testing.T) {
	service := &realtimeQueueServiceStub{}
	realtime := &realtimeSubscriberStub{events: make(chan struct{})}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	server := httptest.NewServer(NewRouter(NewQueueHandler(service, realtime, time.Hour), log, "internal-token"))
	defer server.Close()

	resp, err := server.Client().Post(
		server.URL+APIPrefix+"/internal/rights/token-1/validate",
		"application/json",
		bytes.NewBufferString(`{"product_id":"product-1"}`),
	)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	calls, _, _ := service.checkoutCall()
	require.Zero(t, calls)
}

func TestInternalCheckoutValidationRejectsEmptyProduct(t *testing.T) {
	service := &realtimeQueueServiceStub{}
	realtime := &realtimeSubscriberStub{events: make(chan struct{})}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	server := httptest.NewServer(NewRouter(NewQueueHandler(service, realtime, time.Hour), log, "internal-token"))
	defer server.Close()

	req, err := http.NewRequest(
		http.MethodPost,
		server.URL+APIPrefix+"/internal/rights/token-1/validate",
		bytes.NewBufferString(`{"product_id":""}`),
	)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Internal-Token", "internal-token")

	resp, err := server.Client().Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
	calls, _, _ := service.checkoutCall()
	require.Zero(t, calls)
}

func TestInternalCheckoutValidationPropagatesDomainError(t *testing.T) {
	service := &realtimeQueueServiceStub{checkoutErr: models.ErrForbidden}
	realtime := &realtimeSubscriberStub{events: make(chan struct{})}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	server := httptest.NewServer(NewRouter(NewQueueHandler(service, realtime, time.Hour), log, "internal-token"))
	defer server.Close()

	req, err := http.NewRequest(
		http.MethodPost,
		server.URL+APIPrefix+"/internal/rights/token-1/validate",
		bytes.NewBufferString(`{"product_id":"product-2"}`),
	)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Internal-Token", "internal-token")

	resp, err := server.Client().Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	require.Equal(t, http.StatusForbidden, resp.StatusCode)
}
