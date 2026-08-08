// Package logger provides a structured logging setup based on log/slog.
// It supports context propagation for request-scoped fields like trace IDs.
package logger

import (
	"context"
	"log/slog"
	"os"
)

const (
	// EnvLocal represents the local development environment (human-readable text output).
	EnvLocal = "local"
	// EnvDev represents the development environment (JSON output with debug level).
	EnvDev = "dev"
)

type contextKey string

const loggerKey contextKey = "logger"

// Setup configures the global default slog logger based on the provided environment.
func Setup(env string) {
	var handler slog.Handler

	switch env {
	case EnvLocal:
		handler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		})
	case EnvDev:
		handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		})
	default:
		handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		})
	}

	slog.SetDefault(slog.New(handler))
}

// With creates a new logger with the given attributes, derived from the default logger.
func With(args ...any) *slog.Logger {
	return slog.With(args...)
}

// WithContext returns a new context carrying the provided logger.
func WithContext(ctx context.Context, log *slog.Logger) context.Context {
	return context.WithValue(ctx, loggerKey, log)
}

// FromContext retrieves a logger from the context.
// If no logger is found, it returns the global default logger.
func FromContext(ctx context.Context) *slog.Logger {
	if log, ok := ctx.Value(loggerKey).(*slog.Logger); ok {
		return log
	}
	return slog.Default()
}
