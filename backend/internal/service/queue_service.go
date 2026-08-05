// Package service provides the core business logic for the queue and order allocation system.
// It acts as a strict State Machine ensuring data consistency between cache and durable storage.
package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"backend/internal/models"
	"backend/pkg/logger"

	"github.com/google/uuid"
)

// QueueService orchestrates the queue state machine, durable storage, and fast cache.
type QueueService struct {
	durable    DurableRepo
	cache      CacheRepo
	avito      AvitoClient
	offerTTL   time.Duration
	paymentTTL time.Duration
}

// NewQueueService constructs a new QueueService.
func NewQueueService(
	durable DurableRepo,
	cache CacheRepo,
	avito AvitoClient,
	offerTTL time.Duration,
	paymentTTL time.Duration,
) *QueueService {
	return &QueueService{
		durable:    durable,
		cache:      cache,
		avito:      avito,
		offerTTL:   offerTTL,
		paymentTTL: paymentTTL,
	}
}

// JoinQueue acts as the entry point for users requesting to buy a product.
func (s *QueueService) JoinQueue(ctx context.Context, productID, userID string, quantity int) (*models.QueueMembership, *models.Right, error) {
	log := logger.FromContext(ctx)

	if quantity <= 0 {
		return nil, nil, models.ErrQuantityInvalid
	}

	existingMem, err := s.cache.GetMembership(ctx, productID, userID)
	if err == nil && existingMem != nil {
		if existingMem.Status == models.MembershipStatusQueued ||
			existingMem.Status == models.MembershipStatusOfferPending ||
			existingMem.Status == models.MembershipStatusRightActive {
			return existingMem, nil, nil
		}
	} else if err != nil && !errors.Is(err, models.ErrTokenNotFound) {
		return nil, nil, fmt.Errorf("service.JoinQueue check membership: %w", err)
	}

	totalStock, err := s.avito.GetInitialStock(ctx, productID)
	if err != nil {
		return nil, nil, fmt.Errorf("service.JoinQueue fetch initial stock: %w", err)
	}

	if errInit := s.cache.InitStock(ctx, productID, totalStock); errInit != nil {
		log.WarnContext(ctx, "failed to initialize stock in cache", slog.Any("error", errInit))
	}

	alloc, avail, soldOut, errAlloc := s.cache.TryAllocate(ctx, productID, quantity)
	if errAlloc != nil {
		return nil, nil, fmt.Errorf("service.JoinQueue try allocate: %w", errAlloc)
	}

	mem := &models.QueueMembership{
		ProductID: productID,
		UserID:    userID,
		Quantity:  quantity,
		CreatedAt: time.Now().UTC(),
	}

	right, errProcess := s.processAllocation(ctx, mem, alloc, avail, soldOut)
	if errProcess != nil {
		return nil, nil, errProcess
	}

	if mem.Status == models.MembershipStatusQueued {
		if errEnq := s.cache.Enqueue(ctx, productID, userID); errEnq != nil {
			log.ErrorContext(ctx, "failed to enqueue user", slog.Any("error", errEnq))
		}
	}

	return mem, right, nil
}

// processAllocation encapsulates the state machine transition logic.
func (s *QueueService) processAllocation(
	ctx context.Context,
	mem *models.QueueMembership,
	alloc, avail int,
	soldOut bool,
) (*models.Right, error) {
	log := logger.FromContext(ctx)
	now := time.Now().UTC()
	mem.UpdatedAt = now

	if alloc == mem.Quantity {
		right := &models.Right{
			Token:     uuid.NewString(),
			UserID:    mem.UserID,
			ProductID: mem.ProductID,
			Quantity:  alloc,
			Status:    models.RightStatusActive,
			CreatedAt: now,
			ExpiresAt: now.Add(s.paymentTTL),
		}

		if err := s.durable.SaveRight(ctx, right); err != nil {
			if errRb := s.cache.RestoreAvailableUnits(context.Background(), mem.ProductID, alloc); errRb != nil {
				log.ErrorContext(ctx, "CRITICAL: failed to rollback redis stock after failed PG save", slog.Any("error", errRb))
			}
			return nil, fmt.Errorf("service.processAllocation save right: %w", err)
		}

		mem.Status = models.MembershipStatusRightActive
		mem.CurrentToken = &right.Token
		mem.ExpiresAt = &right.ExpiresAt

		if err := s.durable.UpsertMembership(ctx, mem); err != nil {
			log.ErrorContext(ctx, "failed to upsert membership after right active", slog.Any("error", err))
		}
		if err := s.cache.SetRight(ctx, right); err != nil {
			log.ErrorContext(ctx, "failed to cache right", slog.Any("error", err))
		}
		if err := s.cache.SetMembership(ctx, mem); err != nil {
			log.ErrorContext(ctx, "failed to cache membership", slog.Any("error", err))
		}
		if err := s.cache.AddToExpiryTimer(ctx, mem.ProductID, mem.UserID, right.ExpiresAt); err != nil {
			log.ErrorContext(ctx, "failed to add to expiry timer", slog.Any("error", err))
		}
		if err := s.cache.PublishEvent(ctx, mem.ProductID, mem.UserID, map[string]string{"status": string(mem.Status)}); err != nil {
			log.ErrorContext(ctx, "failed to publish event", slog.Any("error", err))
		}

		return right, nil
	}

	if avail > 0 {
		mem.Status = models.MembershipStatusOfferPending
		mem.AvailableQuantity = &avail
		exp := new(time.Time)
		*exp = now.Add(s.offerTTL)
		mem.ExpiresAt = exp

		if err := s.durable.UpsertMembership(ctx, mem); err != nil {
			if errRb := s.cache.RestoreAvailableUnits(context.Background(), mem.ProductID, avail); errRb != nil {
				log.ErrorContext(ctx, "CRITICAL: failed to rollback partial redis stock", slog.Any("error", errRb))
			}
			return nil, fmt.Errorf("service.processAllocation upsert partial: %w", err)
		}

		if err := s.cache.SetMembership(ctx, mem); err != nil {
			log.ErrorContext(ctx, "failed to cache partial membership", slog.Any("error", err))
		}
		if err := s.cache.AddToExpiryTimer(ctx, mem.ProductID, mem.UserID, *mem.ExpiresAt); err != nil {
			log.ErrorContext(ctx, "failed to add partial to expiry timer", slog.Any("error", err))
		}
		if err := s.cache.PublishEvent(ctx, mem.ProductID, mem.UserID, map[string]string{"status": string(mem.Status)}); err != nil {
			log.ErrorContext(ctx, "failed to publish partial event", slog.Any("error", err))
		}

		return nil, nil
	}

	if soldOut {
		mem.Status = models.MembershipStatusSoldOut
	} else {
		mem.Status = models.MembershipStatusQueued
	}

	if err := s.durable.UpsertMembership(ctx, mem); err != nil {
		return nil, fmt.Errorf("service.processAllocation upsert final state: %w", err)
	}

	if err := s.cache.SetMembership(ctx, mem); err != nil {
		log.ErrorContext(ctx, "failed to cache final membership state", slog.Any("error", err))
	}
	if err := s.cache.PublishEvent(ctx, mem.ProductID, mem.UserID, map[string]string{"status": string(mem.Status)}); err != nil {
		log.ErrorContext(ctx, "failed to publish final event", slog.Any("error", err))
	}

	return nil, nil
}
