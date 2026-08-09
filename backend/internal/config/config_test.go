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
