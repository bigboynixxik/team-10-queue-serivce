// Package postgres_test provides integration tests for the PostgreSQL repository.
// It uses testcontainers to spin up a PostgreSQL instance and applies real goose migrations.
package postgres_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"backend/internal/migrations"
	"backend/internal/models"
	"backend/internal/repository/postgres"
	"backend/pkg/migrator"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

// RepoTestSuite manages the test lifecycle and database dependencies.
type RepoTestSuite struct {
	suite.Suite
	ctx       context.Context
	container *tcpostgres.PostgresContainer
	pool      *pgxpool.Pool
	repo      *postgres.DurableRepo
}

// SetupSuite starts the PostgreSQL container, applies migrations via goose, and initializes the repository.
func (s *RepoTestSuite) SetupSuite() {
	s.ctx = context.Background()

	container, err := tcpostgres.Run(s.ctx,
		"postgres:16-alpine",
		tcpostgres.WithDatabase("test_db"),
		tcpostgres.WithUsername("test_user"),
		tcpostgres.WithPassword("test_pass"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(10*time.Second),
		),
	)
	require.NoError(s.T(), err, "failed to start container")
	s.container = container

	dsn, err := container.ConnectionString(s.ctx, "sslmode=disable")
	require.NoError(s.T(), err)

	s.pool, err = pgxpool.New(s.ctx, dsn)
	require.NoError(s.T(), err)

	sqlDB := stdlib.OpenDBFromPool(s.pool)

	m, err := migrator.EmbedMigrations(sqlDB, migrations.FS, ".")
	require.NoError(s.T(), err, "failed to init migrator")

	err = m.Up()
	require.NoError(s.T(), err, "failed to apply migrations")

	s.repo = postgres.NewDurableRepo(s.pool)
}

// TearDownSuite terminates the container and closes the connection pool.
func (s *RepoTestSuite) TearDownSuite() {
	if s.pool != nil {
		s.pool.Close()
	}
	if s.container != nil {
		require.NoError(s.T(), s.container.Terminate(s.ctx))
	}
}

// SetupTest truncates all tables before each test to ensure isolation.
func (s *RepoTestSuite) SetupTest() {
	_, err := s.pool.Exec(s.ctx, `TRUNCATE stock_decrement_outbox, rights, queue_memberships, product_stock CASCADE;`)
	require.NoError(s.T(), err)
}

func ptr[T any](v T) *T {
	return &v
}

// TestSaveRight validates the successful insertion of a new right.
func (s *RepoTestSuite) TestSaveRight() {
	now := time.Now().UTC().Truncate(time.Microsecond)
	right := &models.Right{
		Token:     "test-token-1",
		UserID:    "user-1",
		ProductID: "prod-1",
		Quantity:  2,
		Status:    models.RightStatusActive,
		CreatedAt: now,
		ExpiresAt: now.Add(15 * time.Minute),
	}

	err := s.repo.SaveRight(s.ctx, right)
	require.NoError(s.T(), err)

	fetched, err := s.repo.GetRightByToken(s.ctx, right.Token)
	require.NoError(s.T(), err)
	require.Equal(s.T(), right.Token, fetched.Token)
	require.Equal(s.T(), right.Quantity, fetched.Quantity)
	require.Equal(s.T(), right.Status, fetched.Status)
	require.True(s.T(), right.CreatedAt.Equal(fetched.CreatedAt))
}

// TestSaveRight_Duplicate validates that inserting a duplicate token yields an error.
func (s *RepoTestSuite) TestSaveRight_Duplicate() {
	now := time.Now().UTC().Truncate(time.Microsecond)
	right := &models.Right{
		Token:     "dup-token",
		UserID:    "user-1",
		ProductID: "prod-1",
		Quantity:  1,
		Status:    models.RightStatusActive,
		CreatedAt: now,
		ExpiresAt: now.Add(15 * time.Minute),
	}

	err := s.repo.SaveRight(s.ctx, right)
	require.NoError(s.T(), err)

	err = s.repo.SaveRight(s.ctx, right)
	require.Error(s.T(), err)
}

// TestSaveRight_InvalidQuantity validates that the database CHECK constraint prevents zero quantity.
func (s *RepoTestSuite) TestSaveRight_InvalidQuantity() {
	now := time.Now().UTC().Truncate(time.Microsecond)
	right := &models.Right{
		Token:     "invalid-qty-token",
		UserID:    "user-1",
		ProductID: "prod-1",
		Quantity:  0,
		Status:    models.RightStatusActive,
		CreatedAt: now,
		ExpiresAt: now.Add(15 * time.Minute),
	}

	err := s.repo.SaveRight(s.ctx, right)
	require.Error(s.T(), err)
}

func (s *RepoTestSuite) TestIssueRightAndUpsertMembershipTx_Success() {
	now := time.Now().UTC().Truncate(time.Microsecond)
	right := &models.Right{
		Token: "atomic-token", UserID: "user-1", ProductID: "prod-1",
		Quantity: 2, Status: models.RightStatusActive,
		CreatedAt: now, ExpiresAt: now.Add(15 * time.Minute),
	}
	membership := &models.QueueMembership{
		ProductID: "prod-1", UserID: "user-1",
		Status: models.MembershipStatusRightActive, Quantity: 2,
		CurrentToken: &right.Token, ExpiresAt: &right.ExpiresAt,
		CreatedAt: now, UpdatedAt: now,
	}

	err := s.repo.IssueRightAndUpsertMembershipTx(s.ctx, right, membership)
	require.NoError(s.T(), err)

	var token string
	err = s.pool.QueryRow(s.ctx, `
		SELECT current_token
		FROM queue_memberships
		WHERE product_id = $1 AND user_id = $2
	`, "prod-1", "user-1").Scan(&token)
	require.NoError(s.T(), err)
	require.Equal(s.T(), right.Token, token)

	storedRight, err := s.repo.GetRightByToken(s.ctx, right.Token)
	require.NoError(s.T(), err)
	require.Equal(s.T(), models.RightStatusActive, storedRight.Status)
}

func (s *RepoTestSuite) TestIssueRightAndUpsertMembershipTx_RollsBackRightWhenMembershipFails() {
	now := time.Now().UTC().Truncate(time.Microsecond)
	right := &models.Right{
		Token: "rolled-back-token", UserID: "user-1", ProductID: "prod-1",
		Quantity: 1, Status: models.RightStatusActive,
		CreatedAt: now, ExpiresAt: now.Add(15 * time.Minute),
	}
	invalidMembership := &models.QueueMembership{
		ProductID: "prod-1", UserID: "user-1",
		Status: models.MembershipStatusRightActive, Quantity: 0,
		CurrentToken: &right.Token, ExpiresAt: &right.ExpiresAt,
		CreatedAt: now, UpdatedAt: now,
	}

	err := s.repo.IssueRightAndUpsertMembershipTx(s.ctx, right, invalidMembership)
	require.Error(s.T(), err)

	_, err = s.repo.GetRightByToken(s.ctx, right.Token)
	require.ErrorIs(s.T(), err, models.ErrTokenNotFound)
}

// TestGetRightByToken_NotFound validates correct error mapping for missing tokens.
func (s *RepoTestSuite) TestGetRightByToken_NotFound() {
	_, err := s.repo.GetRightByToken(s.ctx, "unknown-token")
	require.ErrorIs(s.T(), err, models.ErrTokenNotFound)
}

// TestUpsertMembership validates inserting a new membership and updating it subsequently.
func (s *RepoTestSuite) TestUpsertMembership() {
	now := time.Now().UTC().Truncate(time.Microsecond)
	membership := &models.QueueMembership{
		ProductID: "prod-1",
		UserID:    "user-1",
		Status:    models.MembershipStatusQueued,
		Quantity:  1,
		CreatedAt: now,
		UpdatedAt: now,
	}

	err := s.repo.UpsertMembership(s.ctx, membership)
	require.NoError(s.T(), err)

	membership.Status = models.MembershipStatusOfferPending
	membership.AvailableQuantity = ptr(1)
	err = s.repo.UpsertMembership(s.ctx, membership)
	require.NoError(s.T(), err)

	var status string
	var availQty *int
	err = s.pool.QueryRow(s.ctx, `SELECT status, available_quantity FROM queue_memberships WHERE product_id=$1 AND user_id=$2`, membership.ProductID, membership.UserID).Scan(&status, &availQty)
	require.NoError(s.T(), err)
	require.Equal(s.T(), string(models.MembershipStatusOfferPending), status)
	require.Equal(s.T(), 1, *availQty)
}

// TestSaveInitialStock validates idempotency when saving the initial stock multiple times.
func (s *RepoTestSuite) TestSaveInitialStock() {
	now := time.Now().UTC()
	stock := &models.ProductStock{
		ProductID:    "prod-1",
		ProductCount: 10,
		TotalStock:   10,
		UpdatedAt:    now,
	}

	err := s.repo.SaveInitialStock(s.ctx, stock)
	require.NoError(s.T(), err)

	stock.ProductCount = 5
	err = s.repo.SaveInitialStock(s.ctx, stock)
	require.NoError(s.T(), err)

	var count int
	err = s.pool.QueryRow(s.ctx, `SELECT product_count FROM product_stock WHERE product_id=$1`, stock.ProductID).Scan(&count)
	require.NoError(s.T(), err)
	require.Equal(s.T(), 10, count)
}

// TestUseRightTx validates the atomic decrement and complete USED state.
func (s *RepoTestSuite) TestUseRightTx() {
	now := time.Now().UTC().Truncate(time.Microsecond)

	err := s.repo.SaveInitialStock(s.ctx, &models.ProductStock{
		ProductID: "prod-1", ProductCount: 5, TotalStock: 5, UpdatedAt: now,
	})
	require.NoError(s.T(), err)

	err = s.repo.SaveRight(s.ctx, &models.Right{
		Token: "token-pay", UserID: "u1", ProductID: "prod-1", Quantity: 2, Status: models.RightStatusActive, CreatedAt: now, ExpiresAt: now.Add(time.Minute),
	})
	require.NoError(s.T(), err)

	token := "token-pay"
	err = s.repo.UpsertMembership(s.ctx, &models.QueueMembership{
		ProductID:         "prod-1",
		UserID:            "u1",
		Status:            models.MembershipStatusRightActive,
		Quantity:          2,
		AvailableQuantity: ptr(2),
		CurrentToken:      &token,
		ExpiresAt:         ptr(now.Add(time.Minute)),
		CreatedAt:         now,
		UpdatedAt:         now,
	})
	require.NoError(s.T(), err)

	right, transitioned, err := s.repo.UseRightTx(s.ctx, "token-pay", "order-777", now)
	require.NoError(s.T(), err)
	require.True(s.T(), transitioned)
	require.Equal(s.T(), models.RightStatusUsed, right.Status)
	require.Equal(s.T(), "order-777", *right.OrderID)
	require.NotNil(s.T(), right.UsedAt)
	require.True(s.T(), now.Equal(*right.UsedAt))

	var count int
	err = s.pool.QueryRow(s.ctx, `SELECT product_count FROM product_stock WHERE product_id=$1`, "prod-1").Scan(&count)
	require.NoError(s.T(), err)
	require.Equal(s.T(), 3, count)

	var status models.MembershipStatus
	var currentTokenIsNull, expiresAtIsNull, availableQuantityIsNull bool
	err = s.pool.QueryRow(s.ctx, `
		SELECT status, current_token IS NULL, expires_at IS NULL, available_quantity IS NULL
		FROM queue_memberships WHERE product_id=$1 AND user_id=$2
	`, "prod-1", "u1").Scan(&status, &currentTokenIsNull, &expiresAtIsNull, &availableQuantityIsNull)
	require.NoError(s.T(), err)
	require.Equal(s.T(), models.MembershipStatusPurchased, status)
	require.True(s.T(), currentTokenIsNull)
	require.True(s.T(), expiresAtIsNull)
	require.True(s.T(), availableQuantityIsNull)

	var outboxCount int
	err = s.pool.QueryRow(s.ctx, `
		SELECT count(*)
		FROM stock_decrement_outbox
		WHERE right_token=$1 AND order_id=$2 AND product_id=$3 AND quantity=$4
	`, "token-pay", "order-777", "prod-1", 2).Scan(&outboxCount)
	require.NoError(s.T(), err)
	require.Equal(s.T(), 1, outboxCount)
}

func (s *RepoTestSuite) TestUseRightTx_UsedRightRejectsDifferentOrder() {
	now := time.Now().UTC().Truncate(time.Microsecond)

	err := s.repo.SaveInitialStock(s.ctx, &models.ProductStock{
		ProductID: "prod-1", ProductCount: 5, TotalStock: 5, UpdatedAt: now,
	})
	require.NoError(s.T(), err)
	err = s.repo.SaveRight(s.ctx, &models.Right{
		Token: "token-order", UserID: "u1", ProductID: "prod-1", Quantity: 1, Status: models.RightStatusActive, CreatedAt: now, ExpiresAt: now.Add(time.Minute),
	})
	require.NoError(s.T(), err)

	_, transitioned, err := s.repo.UseRightTx(s.ctx, "token-order", "order-1", now)
	require.NoError(s.T(), err)
	require.True(s.T(), transitioned)

	_, transitioned, err = s.repo.UseRightTx(s.ctx, "token-order", "order-2", now)
	require.ErrorIs(s.T(), err, models.ErrTokenUsed)
	require.False(s.T(), transitioned)

	var outboxCount int
	err = s.pool.QueryRow(s.ctx, `SELECT count(*) FROM stock_decrement_outbox WHERE right_token=$1`, "token-order").Scan(&outboxCount)
	require.NoError(s.T(), err)
	require.Equal(s.T(), 1, outboxCount)
}

func (s *RepoTestSuite) TestUseRightTx_DoesNotOverwriteNewMembershipToken() {
	now := time.Now().UTC().Truncate(time.Microsecond)

	err := s.repo.SaveInitialStock(s.ctx, &models.ProductStock{
		ProductID: "prod-1", ProductCount: 5, TotalStock: 5, UpdatedAt: now,
	})
	require.NoError(s.T(), err)
	err = s.repo.SaveRight(s.ctx, &models.Right{
		Token: "old-token", UserID: "u1", ProductID: "prod-1", Quantity: 1, Status: models.RightStatusActive, CreatedAt: now, ExpiresAt: now.Add(time.Minute),
	})
	require.NoError(s.T(), err)

	newToken := "new-token"
	err = s.repo.SaveRight(s.ctx, &models.Right{
		Token: newToken, UserID: "u1", ProductID: "prod-1", Quantity: 1,
		Status: models.RightStatusActive, CreatedAt: now, ExpiresAt: now.Add(2 * time.Minute),
	})
	require.NoError(s.T(), err)
	err = s.repo.UpsertMembership(s.ctx, &models.QueueMembership{
		ProductID:    "prod-1",
		UserID:       "u1",
		Status:       models.MembershipStatusRightActive,
		Quantity:     1,
		CurrentToken: &newToken,
		ExpiresAt:    ptr(now.Add(2 * time.Minute)),
		CreatedAt:    now,
		UpdatedAt:    now,
	})
	require.NoError(s.T(), err)

	_, transitioned, err := s.repo.UseRightTx(s.ctx, "old-token", "order-old", now)
	require.NoError(s.T(), err)
	require.True(s.T(), transitioned)

	var status models.MembershipStatus
	var currentToken string
	var hasExpiresAt bool
	err = s.pool.QueryRow(s.ctx, `
		SELECT status, current_token, expires_at IS NOT NULL
		FROM queue_memberships WHERE product_id=$1 AND user_id=$2
	`, "prod-1", "u1").Scan(&status, &currentToken, &hasExpiresAt)
	require.NoError(s.T(), err)
	require.Equal(s.T(), models.MembershipStatusRightActive, status)
	require.Equal(s.T(), newToken, currentToken)
	require.True(s.T(), hasExpiresAt)
}

// TestUseRightTx_StockDepleted verifies that both Right and stock roll back together.
// TestExpireRights verifies recovery can settle orphaned rights without
// touching the ones that already reached a terminal state.
func (s *RepoTestSuite) TestExpireRights() {
	now := time.Now().UTC()

	err := s.repo.SaveInitialStock(s.ctx, &models.ProductStock{
		ProductID: "prod-1", ProductCount: 5, TotalStock: 5, UpdatedAt: now,
	})
	require.NoError(s.T(), err)

	for _, token := range []string{"orphan-1", "orphan-2", "keep-active"} {
		err = s.repo.SaveRight(s.ctx, &models.Right{
			Token: token, UserID: "u1", ProductID: "prod-1", Quantity: 1,
			Status: models.RightStatusActive, CreatedAt: now, ExpiresAt: now.Add(time.Minute),
		})
		require.NoError(s.T(), err)
	}

	require.NoError(s.T(), s.repo.ExpireRights(s.ctx, []string{"orphan-1", "orphan-2"}))

	for token, expected := range map[string]models.RightStatus{
		"orphan-1":    models.RightStatusExpired,
		"orphan-2":    models.RightStatusExpired,
		"keep-active": models.RightStatusActive,
	} {
		right, errGet := s.repo.GetRightByToken(s.ctx, token)
		require.NoError(s.T(), errGet)
		require.Equal(s.T(), expected, right.Status, "token %s", token)
	}

	// An empty batch is a no-op rather than a statement with no arguments.
	require.NoError(s.T(), s.repo.ExpireRights(s.ctx, nil))
}

func (s *RepoTestSuite) TestUseRightTx_StockDepleted() {
	now := time.Now().UTC()

	err := s.repo.SaveInitialStock(s.ctx, &models.ProductStock{
		ProductID: "prod-1", ProductCount: 1, TotalStock: 1, UpdatedAt: now,
	})
	require.NoError(s.T(), err)

	err = s.repo.SaveRight(s.ctx, &models.Right{
		Token: "token-fail", UserID: "u1", ProductID: "prod-1", Quantity: 2, Status: models.RightStatusActive, CreatedAt: now, ExpiresAt: now.Add(time.Minute),
	})
	require.NoError(s.T(), err)

	_, transitioned, err := s.repo.UseRightTx(s.ctx, "token-fail", "order-888", now)
	require.ErrorIs(s.T(), err, models.ErrStockDepleted)
	require.False(s.T(), transitioned)

	right, err := s.repo.GetRightByToken(s.ctx, "token-fail")
	require.NoError(s.T(), err)
	require.Equal(s.T(), models.RightStatusActive, right.Status)
}

func (s *RepoTestSuite) TestUseRightTx_TokenNotFound() {
	_, transitioned, err := s.repo.UseRightTx(s.ctx, "ghost-token", "order-999", time.Now().UTC())
	require.ErrorIs(s.T(), err, models.ErrTokenNotFound)
	require.False(s.T(), transitioned)
}

func (s *RepoTestSuite) TestUseRightTx_ExpiredRight() {
	now := time.Now().UTC()

	err := s.repo.SaveInitialStock(s.ctx, &models.ProductStock{
		ProductID: "prod-1", ProductCount: 5, TotalStock: 5, UpdatedAt: now,
	})
	require.NoError(s.T(), err)
	err = s.repo.SaveRight(s.ctx, &models.Right{
		Token: "token-expired", UserID: "u1", ProductID: "prod-1", Quantity: 1, Status: models.RightStatusExpired, CreatedAt: now, ExpiresAt: now.Add(time.Minute),
	})
	require.NoError(s.T(), err)

	_, transitioned, err := s.repo.UseRightTx(s.ctx, "token-expired", "order-1", now)
	require.ErrorIs(s.T(), err, models.ErrTokenExpired)
	require.False(s.T(), transitioned)
}

func (s *RepoTestSuite) TestUseRightTx_PastDeadline() {
	now := time.Now().UTC()

	err := s.repo.SaveInitialStock(s.ctx, &models.ProductStock{
		ProductID: "prod-1", ProductCount: 5, TotalStock: 5, UpdatedAt: now,
	})
	require.NoError(s.T(), err)
	err = s.repo.SaveRight(s.ctx, &models.Right{
		Token: "token-late", UserID: "u1", ProductID: "prod-1", Quantity: 1, Status: models.RightStatusActive, CreatedAt: now.Add(-time.Hour), ExpiresAt: now.Add(-time.Second),
	})
	require.NoError(s.T(), err)

	_, transitioned, err := s.repo.UseRightTx(s.ctx, "token-late", "order-1", now)
	require.ErrorIs(s.T(), err, models.ErrTokenExpired)
	require.False(s.T(), transitioned)
}

func (s *RepoTestSuite) TestUseRightTx_ConcurrentWebhooksTransitionOnce() {
	now := time.Now().UTC()

	err := s.repo.SaveInitialStock(s.ctx, &models.ProductStock{
		ProductID: "prod-1", ProductCount: 5, TotalStock: 5, UpdatedAt: now,
	})
	require.NoError(s.T(), err)
	err = s.repo.SaveRight(s.ctx, &models.Right{
		Token: "token-race", UserID: "u1", ProductID: "prod-1", Quantity: 1, Status: models.RightStatusActive, CreatedAt: now, ExpiresAt: now.Add(time.Minute),
	})
	require.NoError(s.T(), err)

	const workers = 20
	var wg sync.WaitGroup
	var transitionCount atomic.Int32
	errCh := make(chan error, workers)
	start := make(chan struct{})

	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func(index int) {
			defer wg.Done()
			<-start

			right, transitioned, useErr := s.repo.UseRightTx(s.ctx, "token-race", "order-race", now)
			if useErr != nil {
				errCh <- useErr
				return
			}
			if right.Status != models.RightStatusUsed {
				errCh <- fmt.Errorf("unexpected right status: %s", right.Status)
				return
			}
			if transitioned {
				transitionCount.Add(1)
			}
		}(i)
	}

	close(start)
	wg.Wait()
	close(errCh)

	for workerErr := range errCh {
		require.NoError(s.T(), workerErr)
	}
	require.Equal(s.T(), int32(1), transitionCount.Load())

	var count int
	err = s.pool.QueryRow(s.ctx, `SELECT product_count FROM product_stock WHERE product_id=$1`, "prod-1").Scan(&count)
	require.NoError(s.T(), err)
	require.Equal(s.T(), 4, count)

	var outboxCount int
	err = s.pool.QueryRow(s.ctx, `SELECT count(*) FROM stock_decrement_outbox WHERE right_token=$1`, "token-race").Scan(&outboxCount)
	require.NoError(s.T(), err)
	require.Equal(s.T(), 1, outboxCount)
}

func (s *RepoTestSuite) TestStockDecrementOutboxClaimAndAck() {
	now := time.Now().UTC().Truncate(time.Microsecond)
	require.NoError(s.T(), s.repo.SaveRight(s.ctx, &models.Right{
		Token: "token-outbox", UserID: "u1", ProductID: "prod-1", Quantity: 1, Status: models.RightStatusUsed, CreatedAt: now, ExpiresAt: now.Add(time.Minute),
	}))

	_, err := s.pool.Exec(s.ctx, `
		INSERT INTO stock_decrement_outbox
			(id, right_token, order_id, product_id, quantity, next_attempt_at, created_at, updated_at)
		VALUES
			('00000000-0000-0000-0000-000000000001', 'token-outbox', 'order-outbox', 'prod-1', 1, $1, $1, $1)
	`, now.Add(-time.Second))
	require.NoError(s.T(), err)

	leaseUntil := now.Add(30 * time.Second)
	events, err := s.repo.ClaimStockDecrements(s.ctx, now, leaseUntil, 10)
	require.NoError(s.T(), err)
	require.Len(s.T(), events, 1)
	require.Equal(s.T(), "00000000-0000-0000-0000-000000000001", events[0].ID)
	require.Equal(s.T(), 1, events[0].Attempts)
	require.NotNil(s.T(), events[0].LockedUntil)

	events, err = s.repo.ClaimStockDecrements(s.ctx, now, leaseUntil.Add(time.Minute), 10)
	require.NoError(s.T(), err)
	require.Empty(s.T(), events)

	require.NoError(s.T(), s.repo.MarkStockDecrementDelivered(s.ctx, "00000000-0000-0000-0000-000000000001", now))

	events, err = s.repo.ClaimStockDecrements(s.ctx, leaseUntil.Add(time.Minute), leaseUntil.Add(2*time.Minute), 10)
	require.NoError(s.T(), err)
	require.Empty(s.T(), events)
}

func (s *RepoTestSuite) TestStockDecrementOutboxExpiredLeaseIsReclaimed() {
	now := time.Now().UTC().Truncate(time.Microsecond)
	require.NoError(s.T(), s.repo.SaveRight(s.ctx, &models.Right{
		Token: "token-reclaim", UserID: "u1", ProductID: "prod-1", Quantity: 1, Status: models.RightStatusUsed, CreatedAt: now, ExpiresAt: now.Add(time.Minute),
	}))

	_, err := s.pool.Exec(s.ctx, `
		INSERT INTO stock_decrement_outbox
			(id, right_token, order_id, product_id, quantity, attempts, next_attempt_at, locked_until, created_at, updated_at)
		VALUES
			('00000000-0000-0000-0000-000000000002', 'token-reclaim', 'order-reclaim', 'prod-1', 1, 2, $1, $2, $1, $1)
	`, now.Add(-time.Minute), now.Add(-time.Second))
	require.NoError(s.T(), err)

	events, err := s.repo.ClaimStockDecrements(s.ctx, now, now.Add(30*time.Second), 10)
	require.NoError(s.T(), err)
	require.Len(s.T(), events, 1)
	require.Equal(s.T(), 3, events[0].Attempts)
}

func (s *RepoTestSuite) TestExpireRightAndUpsertMembershipTx() {
	now := time.Now().UTC()
	token := "token-expire"

	err := s.repo.SaveRight(s.ctx, &models.Right{
		Token: token, UserID: "u1", ProductID: "prod-1", Quantity: 2, Status: models.RightStatusActive, CreatedAt: now, ExpiresAt: now.Add(time.Minute),
	})
	require.NoError(s.T(), err)
	err = s.repo.UpsertMembership(s.ctx, &models.QueueMembership{
		ProductID: "prod-1", UserID: "u1", Status: models.MembershipStatusRightActive, Quantity: 2, CurrentToken: &token, ExpiresAt: ptr(now.Add(time.Minute)), CreatedAt: now, UpdatedAt: now,
	})
	require.NoError(s.T(), err)

	finalMembership := &models.QueueMembership{
		ProductID: "prod-1", UserID: "u1", Status: models.MembershipStatusDeclined, Quantity: 2, CreatedAt: now, UpdatedAt: now,
	}
	right, transitioned, err := s.repo.ExpireRightAndUpsertMembershipTx(s.ctx, token, finalMembership)
	require.NoError(s.T(), err)
	require.True(s.T(), transitioned)
	require.Equal(s.T(), models.RightStatusExpired, right.Status)

	var status models.MembershipStatus
	var currentToken *string
	err = s.pool.QueryRow(s.ctx, `SELECT status, current_token FROM queue_memberships WHERE product_id=$1 AND user_id=$2`, "prod-1", "u1").Scan(&status, &currentToken)
	require.NoError(s.T(), err)
	require.Equal(s.T(), models.MembershipStatusDeclined, status)
	require.Nil(s.T(), currentToken)

	right, transitioned, err = s.repo.ExpireRightAndUpsertMembershipTx(s.ctx, token, finalMembership)
	require.NoError(s.T(), err)
	require.False(s.T(), transitioned)
	require.Equal(s.T(), models.RightStatusExpired, right.Status)
}

func (s *RepoTestSuite) TestRightTerminalTransitionsRace() {
	now := time.Now().UTC()
	token := "token-terminal-race"

	err := s.repo.SaveInitialStock(s.ctx, &models.ProductStock{
		ProductID: "prod-1", ProductCount: 5, TotalStock: 5, UpdatedAt: now,
	})
	require.NoError(s.T(), err)
	err = s.repo.SaveRight(s.ctx, &models.Right{
		Token: token, UserID: "u1", ProductID: "prod-1", Quantity: 1, Status: models.RightStatusActive, CreatedAt: now, ExpiresAt: now.Add(time.Minute),
	})
	require.NoError(s.T(), err)
	err = s.repo.UpsertMembership(s.ctx, &models.QueueMembership{
		ProductID: "prod-1", UserID: "u1", Status: models.MembershipStatusRightActive, Quantity: 1, CurrentToken: &token, ExpiresAt: ptr(now.Add(time.Minute)), CreatedAt: now, UpdatedAt: now,
	})
	require.NoError(s.T(), err)

	finalMembership := &models.QueueMembership{
		ProductID: "prod-1", UserID: "u1", Status: models.MembershipStatusDeclined, Quantity: 1, CreatedAt: now, UpdatedAt: now,
	}

	type transitionResult struct {
		name         string
		transitioned bool
		err          error
	}
	results := make(chan transitionResult, 2)
	start := make(chan struct{})

	go func() {
		<-start
		_, transitioned, useErr := s.repo.UseRightTx(s.ctx, token, "order-race", now)
		results <- transitionResult{name: "payment", transitioned: transitioned, err: useErr}
	}()
	go func() {
		<-start
		_, transitioned, expireErr := s.repo.ExpireRightAndUpsertMembershipTx(s.ctx, token, finalMembership)
		results <- transitionResult{name: "expiration", transitioned: transitioned, err: expireErr}
	}()

	close(start)
	first := <-results
	second := <-results

	transitionCount := 0
	for _, result := range []transitionResult{first, second} {
		if result.transitioned {
			transitionCount++
		}
		if result.name == "payment" && errors.Is(result.err, models.ErrTokenExpired) {
			continue
		}
		require.NoError(s.T(), result.err)
	}
	require.Equal(s.T(), 1, transitionCount)

	right, err := s.repo.GetRightByToken(s.ctx, token)
	require.NoError(s.T(), err)

	var count int
	err = s.pool.QueryRow(s.ctx, `SELECT product_count FROM product_stock WHERE product_id=$1`, "prod-1").Scan(&count)
	require.NoError(s.T(), err)

	switch right.Status {
	case models.RightStatusUsed:
		require.Equal(s.T(), 4, count)
	case models.RightStatusExpired:
		require.Equal(s.T(), 5, count)
	default:
		s.T().Fatalf("unexpected final right status: %s", right.Status)
	}
}

func (s *RepoTestSuite) TestLoadRecoverySnapshot() {
	base := time.Now().UTC().Truncate(time.Microsecond)
	activeToken := "recovery-active"
	usedToken := "recovery-used"
	offerAvailable := 2
	orderID := "order-recovery"
	usedAt := base.Add(3 * time.Minute)

	_, err := s.pool.Exec(s.ctx, `
		INSERT INTO product_stock (product_id, product_count, total_stock, updated_at)
		VALUES
			('prod-a', 5, 8, $1),
			('prod-b', 3, 3, $1)
	`, base)
	require.NoError(s.T(), err)

	_, err = s.pool.Exec(s.ctx, `
		INSERT INTO rights
			(token, user_id, product_id, quantity, status, order_id, created_at, expires_at, used_at)
		VALUES
			($1, 'user-active', 'prod-a', 1, 'ACTIVE', NULL, $2, $3, NULL),
			($4, 'user-used', 'prod-a', 1, 'USED', $5, $2, $3, $6)
	`, activeToken, base, base.Add(time.Hour), usedToken, orderID, usedAt)
	require.NoError(s.T(), err)

	_, err = s.pool.Exec(s.ctx, `
		INSERT INTO queue_memberships
			(product_id, user_id, status, quantity, available_quantity, current_token, expires_at, created_at, updated_at)
		VALUES
			('prod-a', 'queued-1', 'QUEUED', 1, NULL, NULL, NULL, $1, $2),
			('prod-a', 'active-user', 'RIGHT_ACTIVE', 1, NULL, $3, $4, $1, $5),
			('prod-a', 'offer-user', 'OFFER_PENDING', 4, $6, NULL, $4, $1, $7),
			('prod-b', 'queued-b', 'QUEUED', 1, NULL, NULL, NULL, $1, $2)
	`, base, base.Add(time.Second), activeToken, base.Add(time.Hour), base.Add(2*time.Second), offerAvailable, base.Add(3*time.Second))
	require.NoError(s.T(), err)

	snapshot, err := s.repo.LoadRecoverySnapshot(s.ctx)
	require.NoError(s.T(), err)

	require.Len(s.T(), snapshot.Stocks, 2)
	require.Equal(s.T(), "prod-a", snapshot.Stocks[0].ProductID)
	require.Equal(s.T(), 5, snapshot.Stocks[0].ProductCount)
	require.Equal(s.T(), "prod-b", snapshot.Stocks[1].ProductID)

	require.Len(s.T(), snapshot.Memberships, 4)
	require.Equal(s.T(), []string{"queued-1", "active-user", "offer-user", "queued-b"}, []string{
		snapshot.Memberships[0].UserID,
		snapshot.Memberships[1].UserID,
		snapshot.Memberships[2].UserID,
		snapshot.Memberships[3].UserID,
	})
	require.Equal(s.T(), activeToken, *snapshot.Memberships[1].CurrentToken)
	require.Equal(s.T(), offerAvailable, *snapshot.Memberships[2].AvailableQuantity)

	require.Len(s.T(), snapshot.Rights, 2)
	require.Equal(s.T(), activeToken, snapshot.Rights[0].Token)
	require.Equal(s.T(), models.RightStatusActive, snapshot.Rights[0].Status)
	require.Equal(s.T(), usedToken, snapshot.Rights[1].Token)
	require.Equal(s.T(), orderID, *snapshot.Rights[1].OrderID)
	require.True(s.T(), usedAt.Equal(*snapshot.Rights[1].UsedAt))
}

// TestRepoTestSuite acts as the entry point for 'go test'.
func TestRepoTestSuite(t *testing.T) {
	suite.Run(t, new(RepoTestSuite))
}
