package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"backend/internal/models"
	"backend/pkg/logger"
)

// GetUserQueue returns one membership together with the user's place in that
// queue — what the waiting card shows.
//
// Whether a position applies at all is decided here rather than in the handler:
// it follows from the state machine, not from how the answer is rendered.
func (s *QueueService) GetUserQueue(ctx context.Context, productID, userID string) (*models.UserQueue, error) {
	mem, err := s.cache.GetMembership(ctx, productID, userID)
	if err != nil {
		return nil, fmt.Errorf("service.GetUserQueue: %w", err)
	}

	return s.withPosition(ctx, mem), nil
}

// GetUserQueues returns every queue the user takes part in, each with their
// position and estimated wait — what the «Мои очереди» screen shows.
//
// Terminal memberships (PURCHASED, DECLINED, SOLD_OUT) are returned too: a card
// that silently disappears leaves the user wondering what happened, so the
// frontend gets the whole picture and decides what to fade out.
func (s *QueueService) GetUserQueues(ctx context.Context, userID string) ([]*models.UserQueue, error) {
	memberships, err := s.durable.ListMembershipsByUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("service.GetUserQueues list: %w", err)
	}

	queues := make([]*models.UserQueue, 0, len(memberships))
	for _, mem := range memberships {
		queues = append(queues, s.withPosition(ctx, mem))
	}

	return queues, nil
}

// withPosition attaches the queue position to a membership when it is meaningful.
//
// Only someone still waiting has one: a right holder has already left the queue,
// so asking Redis for their rank would answer a question nobody asked. A failure
// to compute it is logged rather than returned — a card without a position is
// still useful, and a stale Redis queue must not break the screen.
func (s *QueueService) withPosition(ctx context.Context, mem *models.QueueMembership) *models.UserQueue {
	item := &models.UserQueue{Membership: mem}

	if mem == nil || mem.Status != models.MembershipStatusQueued {
		return item
	}

	position, eta, err := s.CalculateETA(ctx, mem.ProductID, mem.UserID)
	if err != nil {
		logger.FromContext(ctx).WarnContext(ctx, "queue position unavailable",
			slog.String("product_id", mem.ProductID),
			slog.String("user_id", mem.UserID),
			slog.Bool("missing_from_queue", errors.Is(err, models.ErrMembershipNotFound)),
			slog.Any("error", err))

		return item
	}

	item.Position = position
	item.ETA = eta

	return item
}
