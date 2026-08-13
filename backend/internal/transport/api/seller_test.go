package api

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"backend/internal/models"

	"github.com/stretchr/testify/require"
)

// TestProductMetrics_Success verifies that the handler correctly maps the domain
// metrics model to the transport JSON structure, including converting durations to seconds.
func TestProductMetrics_Success(t *testing.T) {
	avgPayment := 20 * time.Second
	avgDropOff := 50 * time.Second

	service := &realtimeQueueServiceStub{
		metricsRes: &models.ProductMetrics{
			TotalStock:         100,
			TotalContenders:    8,
			UsedRightsCount:    2,
			ExpiredRightsCount: 1,
			SoldOutCount:       3,
			DropOffCount:       2,
			AvgPaymentTime:     &avgPayment,
			AvgDropOffTime:     &avgDropOff,
		},
	}
	realtime := &realtimeSubscriberStub{events: make(chan struct{})}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	server := httptest.NewServer(NewRouter(NewQueueHandler(service, realtime, time.Hour), log, "internal-token"))
	defer server.Close()

	resp, err := server.Client().Get(server.URL + APIPrefix + "/seller/products/prod-1/metrics")
	require.NoError(t, err)
	defer func() {
		require.NoError(t, resp.Body.Close())
	}()

	require.Equal(t, http.StatusOK, resp.StatusCode)

	var body productMetricsResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))

	require.Equal(t, 100, body.TotalStock)
	require.Equal(t, 8, body.TotalContenders)
	require.Equal(t, 2, body.UsedRightsCount)
	require.Equal(t, 1, body.ExpiredRightsCount)
	require.Equal(t, 3, body.SoldOutCount)
	require.Equal(t, 2, body.DropOffCount)
	require.NotNil(t, body.AvgPaymentTime)
	require.Equal(t, 20, *body.AvgPaymentTime)
	require.NotNil(t, body.AvgDropOffTime)
	require.Equal(t, 50, *body.AvgDropOffTime)
}

// TestProductMetrics_NotFound verifies that a request for a non-existent product
// correctly maps the domain ErrProductNotFound to an HTTP 404 status code.
func TestProductMetrics_NotFound(t *testing.T) {
	service := &realtimeQueueServiceStub{
		metricsErr: models.ErrProductNotFound,
	}
	realtime := &realtimeSubscriberStub{events: make(chan struct{})}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	server := httptest.NewServer(NewRouter(NewQueueHandler(service, realtime, time.Hour), log, "internal-token"))
	defer server.Close()

	resp, err := server.Client().Get(server.URL + APIPrefix + "/seller/products/prod-unknown/metrics")
	require.NoError(t, err)
	defer func() {
		require.NoError(t, resp.Body.Close())
	}()

	require.Equal(t, http.StatusNotFound, resp.StatusCode)
}

// TestProductMetrics_InvalidRequest verifies that an invalid request (e.g. empty product ID)
// correctly maps the domain ErrInvalidRequest to an HTTP 400 status code.
func TestProductMetrics_InvalidRequest(t *testing.T) {
	service := &realtimeQueueServiceStub{
		metricsErr: models.ErrInvalidRequest,
	}
	realtime := &realtimeSubscriberStub{events: make(chan struct{})}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	server := httptest.NewServer(NewRouter(NewQueueHandler(service, realtime, time.Hour), log, "internal-token"))
	defer server.Close()

	resp, err := server.Client().Get(server.URL + APIPrefix + "/seller/products/prod-invalid/metrics")
	require.NoError(t, err)
	defer func() {
		require.NoError(t, resp.Body.Close())
	}()

	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
}
