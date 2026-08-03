// Package mw provides HTTP middleware functions for the transport layer.
package mw

import (
	"log/slog"
	"net/http"

	"backend/pkg/logger"
)

// LoggingMiddleware injects a request-scoped logger into the request context.
// It automatically enriches the logger with the HTTP method and URL path.
func LoggingMiddleware(baseLogger *slog.Logger, next http.HandlerFunc) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqLogger := baseLogger.With("method", r.Method, "path", r.URL.Path)
		ctx := logger.WithContext(r.Context(), reqLogger)
		reqWithContext := r.WithContext(ctx)

		next.ServeHTTP(w, reqWithContext)
	})
}
