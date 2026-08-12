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

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"

	"backend/internal/client/avito"
	"backend/internal/config"
	"backend/internal/migrations"
	"backend/internal/repository/postgres"
	"backend/internal/repository/redis"
	"backend/internal/service"
	"backend/internal/transport/api"
	"backend/pkg/closer"
	"backend/pkg/logger"
	"backend/pkg/migrator"
	"backend/pkg/postgres_settings"
	"backend/pkg/redis_settings"
)

const (
	// readHeaderTimeout guards against slow-header clients.
	readHeaderTimeout = 10 * time.Second
	// envFile is read when running outside Docker; see config.Load.
	envFile = ".env"
)

func main() {
	if err := run(); err != nil {
		slog.Error("service stopped with error", "error", err)
		os.Exit(1)
	}
}

func run() error {
	// The path only matters outside Docker: in a container the variables come
	// from docker-compose and the missing file is ignored.
	cfg, err := config.Load(envFile)
	if err != nil {
		return err
	}

	logger.Setup(cfg.Env)
	log := logger.With("service", "queue-service")

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	shutdown := closer.New()

	pool, err := postgres_settings.NewPool(ctx, cfg.PGDsn)
	if err != nil {
		return err
	}
	shutdown.Add(func(context.Context) error {
		pool.Close()
		return nil
	})

	if err := applyMigrations(pool); err != nil {
		return err
	}
	log.Info("migrations applied")

	redisClient, err := redis_settings.NewClient(ctx, redis_settings.Options{
		Addr:        cfg.RedisAddr,
		Password:    cfg.RedisPassword,
		DB:          cfg.RedisDB,
		PoolSize:    cfg.RedisPoolSize,
		DialTimeout: cfg.RedisDialTimeout,
	})
	if err != nil {
		return err
	}
	shutdown.Add(func(context.Context) error { return redisClient.Close() })

	cacheRepo := redis.NewCacheRepo(redisClient)
	queueService := service.NewQueueService(
		postgres.NewDurableRepo(pool),
		cacheRepo,
		avito.New(cfg.AvitoBaseURL, cfg.InternalToken, 0),
		cfg.OfferTTL,
		cfg.RightTTL,
		cfg.AvgPaymentTime,
		cfg.UserPresenceTimeout,
		service.WithStockOutbox(cfg.StockOutboxLease, cfg.StockOutboxBatchSize, cfg.StockOutboxMaxBackoff),
		service.WithMaxActiveQueues(cfg.MaxActiveQueues),
	)

	if err := queueService.RecoverCache(ctx); err != nil {
		return err
	}

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           api.NewRouter(api.NewQueueHandler(queueService, cacheRepo, cfg.UserPresencePingInterval), log, cfg.InternalToken),
		ReadHeaderTimeout: readHeaderTimeout,
	}
	shutdown.Add(srv.Shutdown)

	go runExpirationWorker(ctx, queueService, cfg.ExpirationInterval, log)
	go runStockDecrementWorker(ctx, queueService, cfg.StockOutboxInterval, log)

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

// applyMigrations runs goose over the embedded SQL. goose needs a database/sql
// handle, so the pgx pool is wrapped rather than opened a second time.
func applyMigrations(pool *pgxpool.Pool) error {
	db := stdlib.OpenDBFromPool(pool)
	defer func() { _ = db.Close() }()

	m, err := migrator.EmbedMigrations(db, migrations.FS, ".")
	if err != nil {
		return err
	}

	return m.Up()
}

// runExpirationWorker is what makes an unused right come back to the queue: the
// state machine only moves on a request or on a timer, and this is the timer
// (docs/design_context.md, п. 5.5).
func runExpirationWorker(ctx context.Context, svc *service.QueueService, interval time.Duration, log *slog.Logger) {
	process := func() {
		if err := svc.ProcessExpirations(ctx); err != nil && !errors.Is(err, context.Canceled) {
			log.Error("process expirations", "error", err)
		}
	}

	process()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			process()
		}
	}
}

func runStockDecrementWorker(ctx context.Context, svc *service.QueueService, interval time.Duration, log *slog.Logger) {
	process := func() {
		if err := svc.ProcessStockDecrementOutbox(ctx); err != nil && !errors.Is(err, context.Canceled) {
			log.Error("process stock decrement outbox", "error", err)
		}
	}

	process()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			process()
		}
	}
}
