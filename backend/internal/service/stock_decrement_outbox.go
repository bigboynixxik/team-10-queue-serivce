package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"backend/internal/models"
	"backend/pkg/logger"
)

const (
	defaultStockOutboxLease      = 30 * time.Second
	defaultStockOutboxBatchSize  = 50
	defaultStockOutboxMaxBackoff = time.Minute
)

// ProcessStockDecrementOutbox delivers persisted Avito stock decrement events.
// A failed HTTP call is retried with backoff; a process crash only keeps rows
// leased until locked_until, after which another pass can pick them up.
func (s *QueueService) ProcessStockDecrementOutbox(ctx context.Context) error {
	log := logger.FromContext(ctx)
	now := time.Now().UTC()

	events, err := s.durable.ClaimStockDecrements(
		ctx,
		now,
		now.Add(s.stockOutboxLease),
		s.stockOutboxBatchSize,
	)
	if err != nil {
		return fmt.Errorf("service.ProcessStockDecrementOutbox claim: %w", err)
	}

	var batchErrs []error
	for _, event := range events {
		if errDeliver := s.deliverStockDecrement(ctx, event); errDeliver != nil {
			log.ErrorContext(ctx, "failed to deliver stock decrement",
				slog.String("event_id", event.ID),
				slog.String("right_token", event.RightToken),
				slog.String("product_id", event.ProductID),
				slog.Int("attempts", event.Attempts),
				slog.Any("error", errDeliver),
			)
			batchErrs = append(batchErrs, errDeliver)
		}
	}

	return errors.Join(batchErrs...)
}

func (s *QueueService) deliverStockDecrement(ctx context.Context, event models.StockDecrement) error {
	now := time.Now().UTC()

	if err := s.avito.DecrementStock(ctx, event.ID, event.ProductID, event.Quantity); err != nil {
		nextAttemptAt := now.Add(stockOutboxBackoff(event.Attempts, s.stockOutboxMaxBackoff))
		lastError := truncateError(err.Error())

		if errReschedule := s.durable.RescheduleStockDecrement(
			ctx, event.ID, nextAttemptAt, lastError, now,
		); errReschedule != nil {
			return fmt.Errorf("reschedule after delivery failure: %w", errors.Join(err, errReschedule))
		}

		return fmt.Errorf("deliver: %w", err)
	}

	if err := s.durable.MarkStockDecrementDelivered(ctx, event.ID, now); err != nil {
		return fmt.Errorf("mark delivered: %w", err)
	}

	return nil
}

func stockOutboxBackoff(attempts int, maxBackoff time.Duration) time.Duration {
	if attempts <= 1 {
		return time.Second
	}

	delay := time.Second
	for i := 1; i < attempts && delay < maxBackoff; i++ {
		delay *= 2
	}
	if delay > maxBackoff {
		return maxBackoff
	}

	return delay
}

func truncateError(message string) string {
	const maxLastErrorLength = 1024
	if len(message) <= maxLastErrorLength {
		return message
	}

	return message[:maxLastErrorLength]
}
