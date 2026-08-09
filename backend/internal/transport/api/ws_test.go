package api

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"backend/internal/models"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/stretchr/testify/require"
)

type realtimeQueueServiceStub struct {
	mu             sync.Mutex
	membership     models.QueueMembership
	position       int
	eta            time.Duration
	userQueueCalls int
}

func (s *realtimeQueueServiceStub) JoinQueue(
	context.Context,
	string,
	string,
	int,
) (*models.QueueMembership, *models.Right, error) {
	return nil, nil, nil
}

func (s *realtimeQueueServiceStub) GetMembership(
	context.Context,
	string,
	string,
) (*models.QueueMembership, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	membership := s.membership
	return &membership, nil
}

func (s *realtimeQueueServiceStub) GetQueueStats(
	context.Context,
	string,
) (*models.QueueStats, error) {
	return nil, nil
}

func (s *realtimeQueueServiceStub) GetUserQueue(
	context.Context,
	string,
	string,
) (*models.UserQueue, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.userQueueCalls++
	membership := s.membership
	queue := &models.UserQueue{Membership: &membership}
	if membership.Status == models.MembershipStatusQueued {
		queue.Position = s.position
		queue.ETA = s.eta
	}

	return queue, nil
}

func (s *realtimeQueueServiceStub) GetUserQueues(
	context.Context,
	string,
) ([]*models.UserQueue, error) {
	return nil, nil
}
func (s *realtimeQueueServiceStub) AcceptOffer(
	context.Context,
	string,
	string,
	int,
) (*models.Right, error) {
	return nil, nil
}

func (s *realtimeQueueServiceStub) DeclineOffer(context.Context, string, string) error {
	return nil
}

func (s *realtimeQueueServiceStub) LeaveQueue(context.Context, string, string) error {
	return nil
}

func (s *realtimeQueueServiceStub) ValidateRight(
	context.Context,
	string,
	string,
) (*models.Right, error) {
	return nil, nil
}

func (s *realtimeQueueServiceStub) ProcessPayment(context.Context, string, string) error {
	return nil
}

func (s *realtimeQueueServiceStub) AdvanceQueue(context.Context, string) error {
	return nil
}

func (s *realtimeQueueServiceStub) CalculateETA(
	context.Context,
	string,
	string,
) (int, time.Duration, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.position, s.eta, nil
}

func (s *realtimeQueueServiceStub) setQueueMetrics(position int, eta time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.position = position
	s.eta = eta
}

func (s *realtimeQueueServiceStub) setStatus(status models.MembershipStatus) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.membership.Status = status
}

func (s *realtimeQueueServiceStub) userQueueCallCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.userQueueCalls
}

type realtimeSubscriberStub struct {
	events chan struct{}
	once   sync.Once
}

func (s *realtimeSubscriberStub) SubscribeUpdates(
	context.Context,
	string,
	string,
) (<-chan struct{}, func() error, error) {
	return s.events, func() error {
		s.once.Do(func() {
			close(s.events)
		})
		return nil
	}, nil
}

func TestWebSocketStreamsInitialSnapshotAndRedisUpdatesWithoutPolling(t *testing.T) {
	service := &realtimeQueueServiceStub{
		membership: models.QueueMembership{
			ProductID: "product-1",
			UserID:    "user-1",
			Status:    models.MembershipStatusQueued,
			Quantity:  2,
		},
		position: 4,
		eta:      30 * time.Second,
	}
	realtime := &realtimeSubscriberStub{events: make(chan struct{}, 4)}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	server := httptest.NewServer(NewRouter(NewQueueHandler(service, realtime), log, "internal-token"))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") +
		APIPrefix + "/queue/product-1/members/me?user_id=user-1"

	dialCtx, cancelDial := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancelDial()

	conn, _, err := websocket.Dial(dialCtx, wsURL, nil)
	require.NoError(t, err)
	defer func() {
		_ = conn.CloseNow()
	}()

	readCtx, cancelRead := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancelRead()

	var initial membershipResponse
	require.NoError(t, wsjson.Read(readCtx, conn, &initial))
	require.Equal(t, models.MembershipStatusQueued, initial.Status)
	require.Equal(t, 4, initial.Position)
	require.Equal(t, 30, initial.ETASeconds)

	time.Sleep(1100 * time.Millisecond)
	require.Equal(t, 1, service.userQueueCallCount(), "queue state must not be polled")

	service.setQueueMetrics(3, 20*time.Second)
	realtime.events <- struct{}{}

	var moved membershipResponse
	require.NoError(t, wsjson.Read(readCtx, conn, &moved))
	require.Equal(t, 3, moved.Position)
	require.Equal(t, 20, moved.ETASeconds)

	service.setStatus(models.MembershipStatusPurchased)
	realtime.events <- struct{}{}

	var purchased membershipResponse
	require.NoError(t, wsjson.Read(readCtx, conn, &purchased))
	require.Equal(t, models.MembershipStatusPurchased, purchased.Status)
	require.Zero(t, purchased.Position)
	require.Zero(t, purchased.ETASeconds)

}
func TestStatusReturnsQueuePositionAndETA(t *testing.T) {
	service := &realtimeQueueServiceStub{
		membership: models.QueueMembership{
			ProductID: "product-1",
			UserID:    "user-1",
			Status:    models.MembershipStatusQueued,
			Quantity:  2,
		},
		position: 7,
		eta:      45 * time.Second,
	}
	realtime := &realtimeSubscriberStub{events: make(chan struct{})}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	server := httptest.NewServer(NewRouter(NewQueueHandler(service, realtime), log, "internal-token"))
	defer server.Close()

	response, err := server.Client().Get(
		server.URL + APIPrefix + "/queue/product-1/members/me?user_id=user-1",
	)
	require.NoError(t, err)
	defer func() {
		require.NoError(t, response.Body.Close())
	}()

	require.Equal(t, http.StatusOK, response.StatusCode)

	var body membershipResponse
	require.NoError(t, json.NewDecoder(response.Body).Decode(&body))
	require.Equal(t, models.MembershipStatusQueued, body.Status)
	require.Equal(t, 7, body.Position)
	require.Equal(t, 45, body.ETASeconds)
}

func TestSameMembershipIncludesQueueMetrics(t *testing.T) {
	first := membershipResponse{Status: models.MembershipStatusQueued, Position: 1, ETASeconds: 10}
	same := membershipResponse{Status: models.MembershipStatusQueued, Position: 1, ETASeconds: 10}
	moved := membershipResponse{Status: models.MembershipStatusQueued, Position: 2, ETASeconds: 10}

	require.True(t, sameMembership(first, same))
	require.False(t, sameMembership(first, moved))
}
