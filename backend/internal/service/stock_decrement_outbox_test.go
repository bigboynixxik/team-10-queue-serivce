package service_test

import (
	"errors"
	"time"

	"backend/internal/models"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func stockDecrementEvent(id string, attempts int) models.StockDecrement {
	now := time.Now().UTC()
	return models.StockDecrement{
		ID:            id,
		RightToken:    "token-" + id,
		OrderID:       "order-" + id,
		ProductID:     "prod-1",
		Quantity:      1,
		Attempts:      attempts,
		NextAttemptAt: now,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
}

func (s *QueueServiceTestSuite) TestProcessStockDecrementOutbox_Success() {
	event := stockDecrementEvent("event-1", 1)

	s.mockDurable.EXPECT().
		ClaimStockDecrements(s.ctx, gomock.Any(), gomock.Any(), 50).
		Return([]models.StockDecrement{event}, nil)
	s.mockAvito.EXPECT().
		DecrementStock(s.ctx, "event-1", "prod-1", 1).
		Return(nil)
	s.mockDurable.EXPECT().
		MarkStockDecrementDelivered(s.ctx, "event-1", gomock.Any()).
		Return(nil)

	err := s.srv.ProcessStockDecrementOutbox(s.ctx)

	require.NoError(s.T(), err)
}

func (s *QueueServiceTestSuite) TestProcessStockDecrementOutbox_FailureIsRescheduled() {
	event := stockDecrementEvent("event-2", 3)
	avitoErr := errors.New("avito timeout")

	s.mockDurable.EXPECT().
		ClaimStockDecrements(s.ctx, gomock.Any(), gomock.Any(), 50).
		Return([]models.StockDecrement{event}, nil)
	s.mockAvito.EXPECT().
		DecrementStock(s.ctx, "event-2", "prod-1", 1).
		Return(avitoErr)
	s.mockDurable.EXPECT().
		RescheduleStockDecrement(s.ctx, "event-2", gomock.Any(), "avito timeout", gomock.Any()).
		Return(nil)

	err := s.srv.ProcessStockDecrementOutbox(s.ctx)

	require.ErrorIs(s.T(), err, avitoErr)
}

func (s *QueueServiceTestSuite) TestProcessStockDecrementOutbox_FailureDoesNotBlockBatch() {
	failed := stockDecrementEvent("event-failed", 1)
	delivered := stockDecrementEvent("event-delivered", 1)
	avitoErr := errors.New("temporary avito error")

	s.mockDurable.EXPECT().
		ClaimStockDecrements(s.ctx, gomock.Any(), gomock.Any(), 50).
		Return([]models.StockDecrement{failed, delivered}, nil)
	s.mockAvito.EXPECT().
		DecrementStock(s.ctx, "event-failed", "prod-1", 1).
		Return(avitoErr)
	s.mockDurable.EXPECT().
		RescheduleStockDecrement(s.ctx, "event-failed", gomock.Any(), "temporary avito error", gomock.Any()).
		Return(nil)
	s.mockAvito.EXPECT().
		DecrementStock(s.ctx, "event-delivered", "prod-1", 1).
		Return(nil)
	s.mockDurable.EXPECT().
		MarkStockDecrementDelivered(s.ctx, "event-delivered", gomock.Any()).
		Return(nil)

	err := s.srv.ProcessStockDecrementOutbox(s.ctx)

	require.ErrorIs(s.T(), err, avitoErr)
}
