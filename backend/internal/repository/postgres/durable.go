// Package postgres provides reliable, persistent storage operations for the queue service.
// It acts as the source of truth and durable log, ensuring data integrity.
package postgres

import (
	"context"
	"errors"
	"fmt"

	"backend/internal/models"
	"backend/pkg/logger"

	sq "github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DurableRepo implements the service.DurableRepo interface using PostgreSQL.
type DurableRepo struct {
	pool *pgxpool.Pool
	sq   sq.StatementBuilderType
}

// NewDurableRepo initializes a new DurableRepo with the given connection pool
// and configures the Squirrel SQL builder to use PostgreSQL dollar placeholders.
func NewDurableRepo(pool *pgxpool.Pool) *DurableRepo {
	return &DurableRepo{
		pool: pool,
		sq:   sq.StatementBuilder.PlaceholderFormat(sq.Dollar),
	}
}

// SaveRight persists a newly issued purchase right into the database.
func (dr *DurableRepo) SaveRight(ctx context.Context, right *models.Right) error {
	query, args, err := dr.sq.Insert("rights").
		Columns("token", "user_id", "product_id", "quantity", "status", "order_id", "created_at", "expires_at", "used_at").
		Values(right.Token, right.UserID, right.ProductID, right.Quantity, right.Status, right.OrderID, right.CreatedAt, right.ExpiresAt, right.UsedAt).
		ToSql()
	if err != nil {
		return fmt.Errorf("postgres.DurableRepo.SaveRight query build: %w", err)
	}

	_, err = dr.pool.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("postgres.DurableRepo.SaveRight execute: %w", err)
	}

	return nil
}

// GetRightByToken retrieves a right by its unique token.
// Returns models.ErrTokenNotFound if the token does not exist[cite: 37].
func (dr *DurableRepo) GetRightByToken(ctx context.Context, token string) (*models.Right, error) {
	query, args, err := dr.sq.Select(
		"token", "user_id", "product_id", "quantity", "status",
		"order_id", "created_at", "expires_at", "used_at",
	).
		From("rights").
		Where(sq.Eq{"token": token}).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("postgres.DurableRepo.GetRightByToken query build: %w", err)
	}

	var right models.Right
	row := dr.pool.QueryRow(ctx, query, args...)
	err = row.Scan(
		&right.Token,
		&right.UserID,
		&right.ProductID,
		&right.Quantity,
		&right.Status,
		&right.OrderID,
		&right.CreatedAt,
		&right.ExpiresAt,
		&right.UsedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("postgres.DurableRepo.GetRightByToken not found: %w", models.ErrTokenNotFound)
		}
		return nil, fmt.Errorf("postgres.DurableRepo.GetRightByToken scan: %w", err)
	}

	return &right, nil
}

// UpsertMembership creates or updates a user's current status in the queue.
// It relies on the UNIQUE(product_id, user_id) constraint to resolve conflicts[cite: 36].
func (dr *DurableRepo) UpsertMembership(ctx context.Context, membership *models.QueueMembership) error {
	query, args, err := dr.sq.Insert("queue_memberships").
		Columns("product_id", "user_id", "status", "quantity", "available_quantity", "current_token", "expires_at", "created_at", "updated_at").
		Values(membership.ProductID, membership.UserID, membership.Status, membership.Quantity, membership.AvailableQuantity, membership.CurrentToken, membership.ExpiresAt, membership.CreatedAt, membership.UpdatedAt).
		Suffix("ON CONFLICT (product_id, user_id) DO UPDATE SET " +
			"status = EXCLUDED.status, " +
			"quantity = EXCLUDED.quantity, " +
			"available_quantity = EXCLUDED.available_quantity, " +
			"current_token = EXCLUDED.current_token, " +
			"expires_at = EXCLUDED.expires_at, " +
			"updated_at = now()").
		ToSql()
	if err != nil {
		return fmt.Errorf("postgres.DurableRepo.UpsertMembership query build: %w", err)
	}

	_, err = dr.pool.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("postgres.DurableRepo.UpsertMembership execute: %w", err)
	}

	return nil
}

// UpdateStockAndRightTx atomically marks a right as USED and decrements the product_stock.
// Both operations are performed within a single database transaction[cite: 36].
func (dr *DurableRepo) UpdateStockAndRightTx(ctx context.Context, token string, orderID string, quantity int) error {
	log := logger.FromContext(ctx)

	tx, err := dr.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("postgres.DurableRepo.UpdateStockAndRightTx begin: %w", err)
	}
	defer func() {
		if rbErr := tx.Rollback(ctx); rbErr != nil && !errors.Is(rbErr, pgx.ErrTxClosed) {
			log.Error("failed to rollback transaction", "error", rbErr)
		}
	}()

	queryRight, argsRight, err := dr.sq.Update("rights").
		Set("status", models.RightStatusUsed).
		Set("order_id", orderID).
		Set("used_at", sq.Expr("now()")).
		Where(sq.Eq{"token": token}).
		Suffix("RETURNING product_id").
		ToSql()
	if err != nil {
		return fmt.Errorf("postgres.DurableRepo.UpdateStockAndRightTx right query build: %w", err)
	}

	var productID string
	err = tx.QueryRow(ctx, queryRight, argsRight...).Scan(&productID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("postgres.DurableRepo.UpdateStockAndRightTx token not found: %w", models.ErrTokenNotFound)
		}
		return fmt.Errorf("postgres.DurableRepo.UpdateStockAndRightTx right execute: %w", err)
	}

	queryStock, argsStock, err := dr.sq.Update("product_stock").
		Set("product_count", sq.Expr("product_count - ?", quantity)).
		Set("updated_at", sq.Expr("now()")).
		Where(sq.Eq{"product_id": productID}).
		ToSql()
	if err != nil {
		return fmt.Errorf("postgres.DurableRepo.UpdateStockAndRightTx stock query build: %w", err)
	}

	_, err = tx.Exec(ctx, queryStock, argsStock...)
	if err != nil {
		if pgErr, ok := errors.AsType[*pgconn.PgError](err); ok && pgErr.Code == "23514" {
			log.Warn("stock check constraint violated during payment confirmation", "product_id", productID, "quantity", quantity)
			return fmt.Errorf("postgres.DurableRepo.UpdateStockAndRightTx stock constraint: %w", models.ErrStockDepleted)
		}
		return fmt.Errorf("postgres.DurableRepo.UpdateStockAndRightTx stock execute: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		log.Error("failed to commit transaction", "error", err)
		return fmt.Errorf("postgres.DurableRepo.UpdateStockAndRightTx commit: %w", err)
	}

	return nil
}

// SaveInitialStock persists the physical stock fetched from AvitoBackend.
// If the product_id already exists, it gracefully ignores the insertion to avoid overwriting ongoing operations.
func (dr *DurableRepo) SaveInitialStock(ctx context.Context, stock *models.ProductStock) error {
	query, args, err := dr.sq.Insert("product_stock").
		Columns("product_id", "product_count", "total_stock", "updated_at").
		Values(stock.ProductID, stock.ProductCount, stock.TotalStock, stock.UpdatedAt).
		Suffix("ON CONFLICT (product_id) DO NOTHING").
		ToSql()
	if err != nil {
		return fmt.Errorf("postgres.DurableRepo.SaveInitialStock query build: %w", err)
	}

	_, err = dr.pool.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("postgres.DurableRepo.SaveInitialStock execute: %w", err)
	}

	return nil
}

// CountMembershipsByStatus returns how many users sit in each membership status
// for a product.
//
// The counts come from Postgres rather than Redis because this is a reporting
// read, not the hot path: scanning Redis for member:{pid}:* would mean a KEYS
// sweep, while here the (product_id, user_id) index does the work.
func (dr *DurableRepo) CountMembershipsByStatus(ctx context.Context, productID string) (map[models.MembershipStatus]int, error) {
	query, args, err := dr.sq.Select("status", "count(*)").
		From("queue_memberships").
		Where(sq.Eq{"product_id": productID}).
		GroupBy("status").
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("postgres.DurableRepo.CountMembershipsByStatus query build: %w", err)
	}

	rows, err := dr.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("postgres.DurableRepo.CountMembershipsByStatus execute: %w", err)
	}
	defer rows.Close()

	counts := make(map[models.MembershipStatus]int)

	for rows.Next() {
		var (
			status models.MembershipStatus
			count  int
		)

		if err := rows.Scan(&status, &count); err != nil {
			return nil, fmt.Errorf("postgres.DurableRepo.CountMembershipsByStatus scan: %w", err)
		}

		counts[status] = count
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres.DurableRepo.CountMembershipsByStatus rows: %w", err)
	}

	return counts, nil
}

// ListMembershipsByUser returns every queue the user takes part in, newest first.
//
// Postgres is the only place this can come from: Redis keys are shaped
// member:{product_id}:{user_id} and cannot be searched by their tail without a
// KEYS sweep. This is a screen read, not the allocation path, so the round trip
// is affordable — see the index added in 002_membership_user_index.sql.
func (dr *DurableRepo) ListMembershipsByUser(ctx context.Context, userID string) ([]*models.QueueMembership, error) {
	query, args, err := dr.sq.Select(
		"id", "product_id", "user_id", "status", "quantity",
		"available_quantity", "current_token", "expires_at", "created_at", "updated_at",
	).
		From("queue_memberships").
		Where(sq.Eq{"user_id": userID}).
		OrderBy("created_at DESC").
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("postgres.DurableRepo.ListMembershipsByUser query build: %w", err)
	}

	rows, err := dr.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("postgres.DurableRepo.ListMembershipsByUser execute: %w", err)
	}
	defer rows.Close()

	memberships := make([]*models.QueueMembership, 0)

	for rows.Next() {
		m := &models.QueueMembership{}

		if err := rows.Scan(
			&m.ID, &m.ProductID, &m.UserID, &m.Status, &m.Quantity,
			&m.AvailableQuantity, &m.CurrentToken, &m.ExpiresAt, &m.CreatedAt, &m.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("postgres.DurableRepo.ListMembershipsByUser scan: %w", err)
		}

		memberships = append(memberships, m)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres.DurableRepo.ListMembershipsByUser rows: %w", err)
	}

	return memberships, nil
}
