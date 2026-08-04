// Command api starts the Queue Service HTTP API.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"backend/internal/config"
	"backend/internal/service/queue"
	"backend/internal/transport/api"
	"backend/pkg/closer"
	"backend/pkg/logger"
)

// readHeaderTimeout guards against slow-header clients.
const readHeaderTimeout = 10 * time.Second

func main() {
	if err := run(); err != nil {
		slog.Error("service stopped with error", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	logger.Setup(cfg.Env)
	log := logger.With("service", "queue-service")

	service := queue.New(cfg.RightTTL, cfg.OfferTTL)

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           api.NewRouter(api.NewQueueHandler(service), log),
		ReadHeaderTimeout: readHeaderTimeout,
	}

	shutdown := closer.New()
	shutdown.Add(srv.Shutdown)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		log.Info("http server started", "addr", srv.Addr, "env", cfg.Env)

		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		log.Info("shutdown signal received")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()

	return shutdown.Close(shutdownCtx)
}
