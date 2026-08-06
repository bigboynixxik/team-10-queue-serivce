// Package redis_settings provides connection pooling and initialization for Redis.
package redis_settings

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	defaultPoolSize    = 100
	defaultDialTimeout = 5 * time.Second
	defaultReadTimeout = 3 * time.Second
)

// Options configures the Redis connection pool behavior.
type Options struct {
	Addr        string
	Password    string
	DB          int
	PoolSize    int
	DialTimeout time.Duration
}

// NewClient establishes a connection pool to Redis using the provided Options.
// It verifies the connection with a Ping before returning the client.
func NewClient(ctx context.Context, opts Options) (*redis.Client, error) {
	applyDefaults(&opts)

	client := redis.NewClient(&redis.Options{
		Addr:         opts.Addr,
		Password:     opts.Password,
		DB:           opts.DB,
		PoolSize:     opts.PoolSize,
		DialTimeout:  opts.DialTimeout,
		ReadTimeout:  defaultReadTimeout,
		WriteTimeout: defaultReadTimeout,
	})

	pingCtx, cancel := context.WithTimeout(ctx, opts.DialTimeout)
	defer cancel()

	if err := client.Ping(pingCtx).Err(); err != nil {
		return nil, fmt.Errorf("redis_settings.NewClient, failed to ping redis: %w", err)
	}

	return client, nil
}

// MustNewClient establishes a connection pool and panics if it fails.
// Useful for application initialization where a cache connection is mandatory.
func MustNewClient(ctx context.Context, opts Options) *redis.Client {
	client, err := NewClient(ctx, opts)
	if err != nil {
		panic(err)
	}
	return client
}

// applyDefaults sets recommended fallback values for any zero-value fields.
func applyDefaults(opts *Options) {
	if opts.PoolSize <= 0 {
		opts.PoolSize = defaultPoolSize
	}
	if opts.DialTimeout <= 0 {
		opts.DialTimeout = defaultDialTimeout
	}
}
