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
	Env string `env:"APP_ENV" envDefault:"local"`

	// Port is the HTTP port the REST API and WebSocket server listens on.
	Port string `env:"APP_PORT" envDefault:"8080"`

	// PGDsn is the PostgreSQL connection string.
	PGDsn string `env:"PG_DSN,required"`

	// RedisAddr is the host and port of the Redis server.
	RedisAddr string `env:"REDIS_ADDR" envDefault:"redis:6379"`

	// RedisPassword is the authentication password for Redis (leave empty if none).
	RedisPassword string `env:"REDIS_PASSWORD"`

	// RedisDB selects the specific Redis logical database index.
	RedisDB int `env:"REDIS_DB" envDefault:"0"`

	// RedisPoolSize limits the maximum number of socket connections to Redis.
	RedisPoolSize int `env:"REDIS_POOL_SIZE" envDefault:"100"`

	// RedisDialTimeout is the maximum time to wait for a connection to be established.
	RedisDialTimeout time.Duration `env:"REDIS_DIAL_TIMEOUT" envDefault:"5s"`

	// RightTTL is the lifetime of an issued purchase right before it expires.
	RightTTL time.Duration `env:"RIGHT_TTL" envDefault:"4m"`

	// RightHeartbeatInterval controls how often the server probes a WebSocket client.
	RightHeartbeatInterval time.Duration `env:"RIGHT_HEARTBEAT_INTERVAL" envDefault:"5s"`

	// RightHeartbeatTimeout is the grace period before an unconfirmed right is released.
	RightHeartbeatTimeout time.Duration `env:"RIGHT_HEARTBEAT_TIMEOUT" envDefault:"30s"`
	// OfferTTL is how long a partial offer waits for the user's decision.

	OfferTTL time.Duration `env:"OFFER_TTL" envDefault:"2m"`

	// ShutdownTimeout bounds the graceful shutdown period.
	ShutdownTimeout time.Duration `env:"SHUTDOWN_TIMEOUT" envDefault:"10s"`

	// AvitoBaseURL points at AvitoBackend, the owner of the physical stock.
	AvitoBaseURL string `env:"AVITO_BASE_URL" envDefault:"http://avitomock:9090"`

	// InternalToken is the shared secret for service-to-service calls in both
	// directions. Empty disables the check, which is only sane locally.
	InternalToken string `env:"INTERNAL_TOKEN"`

	// ExpirationInterval is how often the background worker looks for expired
	// rights and offers.
	ExpirationInterval time.Duration `env:"EXPIRATION_INTERVAL" envDefault:"1s"`

	// StockOutboxInterval is how often the background worker delivers stock
	// decrement events to AvitoBackend.
	StockOutboxInterval time.Duration `env:"STOCK_OUTBOX_INTERVAL" envDefault:"1s"`

	// StockOutboxBatchSize bounds one delivery pass.
	StockOutboxBatchSize int `env:"STOCK_OUTBOX_BATCH_SIZE" envDefault:"50"`

	// StockOutboxLease is how long one worker owns claimed stock decrement events.
	StockOutboxLease time.Duration `env:"STOCK_OUTBOX_LEASE" envDefault:"30s"`

	// StockOutboxMaxBackoff caps retry delay for failed AvitoBackend deliveries.
	StockOutboxMaxBackoff time.Duration `env:"STOCK_OUTBOX_MAX_BACKOFF" envDefault:"1m"`

	// AvgPaymentTime is the estimated duration a single user takes to complete a purchase.
	AvgPaymentTime time.Duration `env:"AVG_PAYMENT_TIME" envDefault:"75s"`
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
	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("config.Load validate: %w", err)
	}

	return &cfg, nil
}

func (c Config) validate() error {
	if c.RightHeartbeatInterval <= 0 {
		return fmt.Errorf("RIGHT_HEARTBEAT_INTERVAL must be positive")
	}
	if c.RightHeartbeatTimeout <= 0 {
		return fmt.Errorf("RIGHT_HEARTBEAT_TIMEOUT must be positive")
	}
	if c.RightHeartbeatTimeout <= c.RightHeartbeatInterval {
		return fmt.Errorf("RIGHT_HEARTBEAT_TIMEOUT must be greater than RIGHT_HEARTBEAT_INTERVAL")
	}
	if c.StockOutboxInterval <= 0 {
		return fmt.Errorf("STOCK_OUTBOX_INTERVAL must be positive")
	}
	if c.StockOutboxBatchSize <= 0 {
		return fmt.Errorf("STOCK_OUTBOX_BATCH_SIZE must be positive")
	}
	if c.StockOutboxLease <= 0 {
		return fmt.Errorf("STOCK_OUTBOX_LEASE must be positive")
	}
	if c.StockOutboxMaxBackoff <= 0 {
		return fmt.Errorf("STOCK_OUTBOX_MAX_BACKOFF must be positive")
	}

	return nil
}
