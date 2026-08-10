package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestLoadHeartbeatDefaults(t *testing.T) {
	t.Setenv("PG_DSN", "postgres://test")
	t.Setenv("RIGHT_HEARTBEAT_INTERVAL", "")
	t.Setenv("RIGHT_HEARTBEAT_TIMEOUT", "")

	cfg, err := Load("missing.env")

	require.NoError(t, err)
	require.Equal(t, 5*time.Second, cfg.RightHeartbeatInterval)
	require.Equal(t, 30*time.Second, cfg.RightHeartbeatTimeout)
}

func TestLoadStockOutboxDefaults(t *testing.T) {
	t.Setenv("PG_DSN", "postgres://test")
	t.Setenv("STOCK_OUTBOX_INTERVAL", "")
	t.Setenv("STOCK_OUTBOX_BATCH_SIZE", "")
	t.Setenv("STOCK_OUTBOX_LEASE", "")
	t.Setenv("STOCK_OUTBOX_MAX_BACKOFF", "")

	cfg, err := Load("missing.env")

	require.NoError(t, err)
	require.Equal(t, time.Second, cfg.StockOutboxInterval)
	require.Equal(t, 50, cfg.StockOutboxBatchSize)
	require.Equal(t, 30*time.Second, cfg.StockOutboxLease)
	require.Equal(t, time.Minute, cfg.StockOutboxMaxBackoff)
}

func TestLoadRejectsInvalidHeartbeatConfiguration(t *testing.T) {
	tests := []struct {
		name     string
		interval string
		timeout  string
	}{
		{name: "zero interval", interval: "0s", timeout: "30s"},
		{name: "zero timeout", interval: "5s", timeout: "0s"},
		{name: "timeout equals interval", interval: "5s", timeout: "5s"},
		{name: "timeout below interval", interval: "10s", timeout: "5s"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("PG_DSN", "postgres://test")
			t.Setenv("RIGHT_HEARTBEAT_INTERVAL", tt.interval)
			t.Setenv("RIGHT_HEARTBEAT_TIMEOUT", tt.timeout)

			_, err := Load("missing.env")

			require.ErrorContains(t, err, "config.Load validate")
		})
	}
}

func TestLoadRejectsInvalidStockOutboxConfiguration(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		value    string
		contains string
	}{
		{name: "zero interval", key: "STOCK_OUTBOX_INTERVAL", value: "0s", contains: "STOCK_OUTBOX_INTERVAL"},
		{name: "zero batch", key: "STOCK_OUTBOX_BATCH_SIZE", value: "0", contains: "STOCK_OUTBOX_BATCH_SIZE"},
		{name: "zero lease", key: "STOCK_OUTBOX_LEASE", value: "0s", contains: "STOCK_OUTBOX_LEASE"},
		{name: "zero max backoff", key: "STOCK_OUTBOX_MAX_BACKOFF", value: "0s", contains: "STOCK_OUTBOX_MAX_BACKOFF"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("PG_DSN", "postgres://test")
			t.Setenv(tt.key, tt.value)

			_, err := Load("missing.env")

			require.ErrorContains(t, err, tt.contains)
		})
	}
}
