// Package config reads the service settings from the environment.
// It provides fail-fast validation for required infrastructure settings.
package config

import (
	"fmt"
	"time"

	"github.com/caarlos0/env/v11"
	"github.com/joho/godotenv"
)

// Config holds everything the service needs to start and connect to infrastructure.
type Config struct {
	// Env selects the application environment (e.g., local, dev, prod).
	Env string `env:"ENV" envDefault:"local"`

	// Port is the HTTP port the REST API and WebSocket server listens on.
	Port string `env:"PORT" envDefault:"8080"`

	// PGDsn is the PostgreSQL connection string.
	PGDsn string `env:"PG_DSN,required"`

	RedisAddr        string        `env:"REDIS_ADDR" envDefault:"redis:6379"`
	RedisPassword    string        `env:"REDIS_PASSWORD"`
	RedisDB          int           `env:"REDIS_DB" envDefault:"0"`
	RedisPoolSize    int           `env:"REDIS_POOL_SIZE" envDefault:"100"`
	RedisDialTimeout time.Duration `env:"REDIS_DIAL_TIMEOUT" envDefault:"5s"`

	// RightTTL is the lifetime of an issued purchase right before it expires.
	RightTTL time.Duration `env:"RIGHT_TTL" envDefault:"15m"`

	// OfferTTL is how long a partial offer waits for the user's decision.
	OfferTTL time.Duration `env:"OFFER_TTL" envDefault:"2m"`

	// ShutdownTimeout bounds the graceful shutdown period.
	ShutdownTimeout time.Duration `env:"SHUTDOWN_TIMEOUT" envDefault:"10s"`
}

// Load reads the configuration from the .env file and environment variables.
// Variables from the OS environment override those in the .env file.
func Load(path string) (*Config, error) {
	// The error from godotenv.Load is explicitly ignored because the .env file
	// is only required for local development. In Docker environments,
	// variables are injected directly via docker-compose.
	_ = godotenv.Load(path)

	var cfg Config
	if err := env.Parse(&cfg); err != nil {
		return nil, fmt.Errorf("config.Load parse error: %w", err)
	}

	return &cfg, nil
}
