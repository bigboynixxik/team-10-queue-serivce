// Package postgres_test provides integration tests for the PostgreSQL repository.
// It uses testcontainers to spin up a PostgreSQL instance and applies real goose migrations.
package postgres_test

import (
	"context"
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
	_, err := s.pool.Exec(s.ctx, `TRUNCATE rights, queue_memberships, product_stock CASCADE;`)
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

// TestUpdateStockAndRightTx validates the atomic decrement of stock and status update of a right.
func (s *RepoTestSuite) TestUpdateStockAndRightTx() {
	now := time.Now().UTC()

	err := s.repo.SaveInitialStock(s.ctx, &models.ProductStock{
		ProductID: "prod-1", ProductCount: 5, TotalStock: 5, UpdatedAt: now,
	})
	require.NoError(s.T(), err)

	err = s.repo.SaveRight(s.ctx, &models.Right{
		Token: "token-pay", UserID: "u1", ProductID: "prod-1", Quantity: 2, Status: models.RightStatusActive, CreatedAt: now, ExpiresAt: now.Add(time.Minute),
	})
	require.NoError(s.T(), err)

	err = s.repo.UpdateStockAndRightTx(s.ctx, "token-pay", "order-777", 2)
	require.NoError(s.T(), err)

	right, err := s.repo.GetRightByToken(s.ctx, "token-pay")
	require.NoError(s.T(), err)
	require.Equal(s.T(), models.RightStatusUsed, right.Status)
	require.Equal(s.T(), "order-777", *right.OrderID)
	require.NotNil(s.T(), right.UsedAt)

	var count int
	err = s.pool.QueryRow(s.ctx, `SELECT product_count FROM product_stock WHERE product_id=$1`, "prod-1").Scan(&count)
	require.NoError(s.T(), err)
	require.Equal(s.T(), 3, count)
}

// TestUpdateStockAndRightTx_StockDepleted validates that a check constraint violation rolls back the transaction.
func (s *RepoTestSuite) TestUpdateStockAndRightTx_StockDepleted() {
	now := time.Now().UTC()

	err := s.repo.SaveInitialStock(s.ctx, &models.ProductStock{
		ProductID: "prod-1", ProductCount: 1, TotalStock: 1, UpdatedAt: now,
	})
	require.NoError(s.T(), err)

	err = s.repo.SaveRight(s.ctx, &models.Right{
		Token: "token-fail", UserID: "u1", ProductID: "prod-1", Quantity: 2, Status: models.RightStatusActive, CreatedAt: now, ExpiresAt: now.Add(time.Minute),
	})
	require.NoError(s.T(), err)

	err = s.repo.UpdateStockAndRightTx(s.ctx, "token-fail", "order-888", 2)
	require.ErrorIs(s.T(), err, models.ErrStockDepleted)

	right, err := s.repo.GetRightByToken(s.ctx, "token-fail")
	require.NoError(s.T(), err)
	require.Equal(s.T(), models.RightStatusActive, right.Status)
}

// TestUpdateStockAndRightTx_TokenNotFound validates behavior when an unknown token is processed.
func (s *RepoTestSuite) TestUpdateStockAndRightTx_TokenNotFound() {
	err := s.repo.UpdateStockAndRightTx(s.ctx, "ghost-token", "order-999", 1)
	require.ErrorIs(s.T(), err, models.ErrTokenNotFound)
}

// TestRepoTestSuite acts as the entry point for 'go test'.
func TestRepoTestSuite(t *testing.T) {
	suite.Run(t, new(RepoTestSuite))
}
