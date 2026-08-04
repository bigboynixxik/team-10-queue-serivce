// Package config reads the service settings from the environment
// (the variables are the ones docker-compose.yml passes in).
package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config holds everything the service needs to start.
type Config struct {
	// Env selects the log format: local, dev or anything else (see pkg/logger).
	Env string
	// Port is the HTTP port the API listens on.
	Port string
	// RightTTL is the lifetime of an issued purchase right.
	RightTTL time.Duration
	// OfferTTL is how long a partial offer waits for the user's decision.
	OfferTTL time.Duration
	// ShutdownTimeout bounds the graceful shutdown.
	ShutdownTimeout time.Duration
}

// Load reads the configuration, falling back to values that make the service
// runnable with no environment at all.
func Load() (Config, error) {
	cfg := Config{
		Env:  env("ENV", "local"),
		Port: env("PORT", "8080"),
	}

	var err error
	if cfg.RightTTL, err = duration("RIGHT_TTL", 15*time.Minute); err != nil {
		return Config{}, err
	}
	if cfg.OfferTTL, err = duration("OFFER_TTL", 2*time.Minute); err != nil {
		return Config{}, err
	}
	if cfg.ShutdownTimeout, err = duration("SHUTDOWN_TIMEOUT", 10*time.Second); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func env(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

// duration parses a Go duration ("15m"); a plain number is read as seconds.
func duration(name string, fallback time.Duration) (time.Duration, error) {
	raw := os.Getenv(name)
	if raw == "" {
		return fallback, nil
	}

	if seconds, err := strconv.Atoi(raw); err == nil {
		return time.Duration(seconds) * time.Second, nil
	}

	value, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("parse %s: %w", name, err)
	}

	return value, nil
}
