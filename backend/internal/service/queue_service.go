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
		if existingMem.Status == models.MembershipStatusRightActive && existingMem.CurrentToken != nil {
			right, _ := s.cache.GetRight(ctx, *existingMem.CurrentToken)
			return existingMem, right, nil
		}
		if existingMem.Status == models.MembershipStatusQueued || existingMem.Status == models.MembershipStatusOfferPending {
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
	stockModel := &models.ProductStock{
		ProductID:    productID,
		TotalStock:   totalStock,
		ProductCount: totalStock,
		UpdatedAt:    time.Now().UTC(),
	}
	if err := s.durable.SaveInitialStock(ctx, stockModel); err != nil {
		log.ErrorContext(ctx, "CRITICAL: failed to save initial stock to db", slog.Any("error", err))
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

		s.syncCacheState(ctx, mem, right)
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

		s.syncCacheState(ctx, mem, nil)
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

	s.syncCacheState(ctx, mem, nil)
	return nil, nil
}

// AcceptOffer confirms a partial offer. If the accepted quantity is less than the
// available quantity, the unused remainder is restored to the pool and the queue advances.
func (s *QueueService) AcceptOffer(ctx context.Context, productID, userID string, acceptedQuantity int) (*models.Right, error) {
	log := logger.FromContext(ctx)

	if acceptedQuantity <= 0 {
		return nil, models.ErrQuantityInvalid
	}

	mem, err := s.cache.GetMembership(ctx, productID, userID)
	if err != nil {
		return nil, fmt.Errorf("service.AcceptOffer get membership: %w", err)
	}

	if mem.ExpiresAt != nil && time.Now().UTC().After(*mem.ExpiresAt) {
		return nil, models.ErrTokenExpired
	}

	if mem.Status != models.MembershipStatusOfferPending || mem.AvailableQuantity == nil {
		return nil, models.ErrInvalidStatus
	}

	if acceptedQuantity > *mem.AvailableQuantity {
		return nil, models.ErrQuantityExceeded
	}

	returnedQty := *mem.AvailableQuantity - acceptedQuantity
	now := time.Now().UTC()

	right := &models.Right{
		Token:     uuid.NewString(),
		UserID:    mem.UserID,
		ProductID: mem.ProductID,
		Quantity:  acceptedQuantity,
		Status:    models.RightStatusActive,
		CreatedAt: now,
		ExpiresAt: now.Add(s.paymentTTL),
	}

	if err := s.durable.SaveRight(ctx, right); err != nil {
		return nil, fmt.Errorf("service.AcceptOffer save right: %w", err)
	}

	mem.Status = models.MembershipStatusRightActive
	mem.Quantity = acceptedQuantity
	mem.AvailableQuantity = nil
	mem.CurrentToken = &right.Token
	mem.ExpiresAt = &right.ExpiresAt
	mem.UpdatedAt = now

	if err := s.durable.UpsertMembership(ctx, mem); err != nil {
		log.ErrorContext(ctx, "failed to upsert membership after accept", slog.Any("error", err))
	}

	s.syncCacheState(ctx, mem, right)

	if returnedQty > 0 {
		if err := s.cache.RestoreAvailableUnits(ctx, productID, returnedQty); err != nil {
			log.ErrorContext(ctx, "failed to restore unused units", slog.Any("error", err))
		}
		if err := s.AdvanceQueue(ctx, productID); err != nil {
			log.ErrorContext(ctx, "failed to advance queue after partial accept", slog.Any("error", err))
		}
	}

	return right, nil
}

// DeclineOffer rejects a pending offer. The reserved stock is entirely returned
// to the available pool, and the queue is advanced.
func (s *QueueService) DeclineOffer(ctx context.Context, productID, userID string) error {
	log := logger.FromContext(ctx)

	mem, err := s.cache.GetMembership(ctx, productID, userID)
	if err != nil {
		return fmt.Errorf("service.DeclineOffer get membership: %w", err)
	}

	if mem.ExpiresAt != nil && time.Now().UTC().After(*mem.ExpiresAt) {
		return models.ErrTokenExpired
	}

	if mem.Status != models.MembershipStatusOfferPending || mem.AvailableQuantity == nil {
		return models.ErrInvalidStatus
	}

	returnedQty := *mem.AvailableQuantity
	now := time.Now().UTC()

	mem.Status = models.MembershipStatusDeclined
	mem.AvailableQuantity = nil
	mem.ExpiresAt = nil
	mem.UpdatedAt = now

	if err := s.durable.UpsertMembership(ctx, mem); err != nil {
		return fmt.Errorf("service.DeclineOffer upsert final state: %w", err)
	}

	s.syncCacheState(ctx, mem, nil)

	if err := s.cache.RemoveFromExpiryTimer(ctx, productID, userID); err != nil {
		log.ErrorContext(ctx, "failed to remove from expiry timer", slog.Any("error", err))
	}

	if returnedQty > 0 {
		if err := s.cache.RestoreAvailableUnits(ctx, productID, returnedQty); err != nil {
			log.ErrorContext(ctx, "failed to restore unused units", slog.Any("error", err))
		}
		if err := s.AdvanceQueue(ctx, productID); err != nil {
			log.ErrorContext(ctx, "failed to advance queue after decline", slog.Any("error", err))
		}
	}

	return nil
}

// syncCacheState is a DRY helper to update Redis and broadcast the state.
func (s *QueueService) syncCacheState(ctx context.Context, mem *models.QueueMembership, right *models.Right) {
	log := logger.FromContext(ctx)

	if right != nil {
		if err := s.cache.SetRight(ctx, right); err != nil {
			log.ErrorContext(ctx, "failed to cache right", slog.Any("error", err))
		}
	}
	if err := s.cache.SetMembership(ctx, mem); err != nil {
		log.ErrorContext(ctx, "failed to cache membership", slog.Any("error", err))
	}
	if mem.ExpiresAt != nil {
		if err := s.cache.AddToExpiryTimer(ctx, mem.ProductID, mem.UserID, *mem.ExpiresAt); err != nil {
			log.ErrorContext(ctx, "failed to add to expiry timer", slog.Any("error", err))
		}
	}
	if err := s.cache.PublishEvent(ctx, mem.ProductID, mem.UserID, map[string]string{"status": string(mem.Status)}); err != nil {
		log.ErrorContext(ctx, "failed to publish event", slog.Any("error", err))
	}
}

// AdvanceQueue acts as an internal engine to push the queue forward when stock frees up.
// It intentionally ignores the boolean soldOut flag returned by PopAndAllocate,
// relying exclusively on the strict models.MembershipStatus for state transitions.
func (s *QueueService) AdvanceQueue(ctx context.Context, productID string) error {
	log := logger.FromContext(ctx)

	for {
		uid, alloc, avail, _, status, score, err := s.cache.PopAndAllocate(ctx, productID)
		if err != nil {
			return fmt.Errorf("service.AdvanceQueue pop and allocate: %w", err)
		}

		if uid == "" || status == models.MembershipStatusQueued {
			break
		}

		if status != models.MembershipStatusRightActive &&
			status != models.MembershipStatusOfferPending &&
			status != models.MembershipStatusSoldOut {
			continue
		}

		mem, err := s.cache.GetMembership(ctx, productID, uid)
		if err != nil {
			s.rollbackAdvance(ctx, productID, uid, alloc+avail, score)
			log.ErrorContext(ctx, "failed to get membership for advanced user", slog.Any("error", err), slog.String("user_id", uid))
			continue
		}

		now := time.Now().UTC()
		mem.UpdatedAt = now

		if status == models.MembershipStatusRightActive {
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
				s.rollbackAdvance(ctx, productID, uid, alloc, score)
				log.ErrorContext(ctx, "failed to save right in advance queue", slog.Any("error", err))
				continue
			}

			mem.Status = models.MembershipStatusRightActive
			mem.CurrentToken = &right.Token
			mem.ExpiresAt = &right.ExpiresAt

			if err := s.durable.UpsertMembership(ctx, mem); err != nil {
				log.ErrorContext(ctx, "failed to upsert membership", slog.Any("error", err))
			}
			s.syncCacheState(ctx, mem, right)

		} else if status == models.MembershipStatusOfferPending {
			mem.Status = models.MembershipStatusOfferPending
			mem.AvailableQuantity = &avail

			exp := new(time.Time)
			*exp = now.Add(s.offerTTL)
			mem.ExpiresAt = exp

			if err := s.durable.UpsertMembership(ctx, mem); err != nil {
				s.rollbackAdvance(ctx, productID, uid, avail, score)
				log.ErrorContext(ctx, "failed to upsert partial membership", slog.Any("error", err))
				continue
			}
			s.syncCacheState(ctx, mem, nil)

		} else {
			mem.Status = models.MembershipStatusSoldOut
			if err := s.durable.UpsertMembership(ctx, mem); err != nil {
				s.rollbackAdvance(ctx, productID, uid, 0, score)
				log.ErrorContext(ctx, "failed to upsert sold_out membership", slog.Any("error", err))
				continue
			}
			s.syncCacheState(ctx, mem, nil)
		}
	}

	return nil
}

// rollbackAdvance handles disaster recovery if the durable database fails during queue advancement.
func (s *QueueService) rollbackAdvance(ctx context.Context, productID, userID string, qty int, score float64) {
	log := logger.FromContext(ctx)

	if qty > 0 {
		if err := s.cache.RestoreAvailableUnits(context.Background(), productID, qty); err != nil {
			log.ErrorContext(ctx, "CRITICAL: failed to restore units on advance rollback", slog.Any("error", err))
		}
	}

	if err := s.cache.Requeue(context.Background(), productID, userID, score); err != nil {
		log.ErrorContext(ctx, "CRITICAL: failed to requeue user on advance rollback", slog.Any("error", err))
	}
}
