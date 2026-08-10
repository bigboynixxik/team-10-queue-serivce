// Package postgres provides reliable, persistent storage operations for the queue service.
// It acts as the source of truth and durable log, ensuring data integrity.
package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"backend/internal/models"
	"backend/pkg/logger"

	sq "github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
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

// LoadRecoverySnapshot reads every durable row needed to reconstruct Redis from
// PostgreSQL. A repeatable-read, read-only transaction keeps stock, membership,
// and right rows on the same database snapshot.
func (dr *DurableRepo) LoadRecoverySnapshot(ctx context.Context) (*models.RecoverySnapshot, error) {
	log := logger.FromContext(ctx)

	tx, err := dr.pool.BeginTx(ctx, pgx.TxOptions{
		IsoLevel:   pgx.RepeatableRead,
		AccessMode: pgx.ReadOnly,
	})
	if err != nil {
		return nil, fmt.Errorf("postgres.DurableRepo.LoadRecoverySnapshot begin: %w", err)
	}
	defer func() {
		if rbErr := tx.Rollback(ctx); rbErr != nil && !errors.Is(rbErr, pgx.ErrTxClosed) {
			log.Error("failed to rollback recovery snapshot transaction", "error", rbErr)
		}
	}()

	stocks, err := dr.loadRecoveryStocks(ctx, tx)
	if err != nil {
		return nil, err
	}

	memberships, err := dr.loadRecoveryMemberships(ctx, tx)
	if err != nil {
		return nil, err
	}

	rights, err := dr.loadRecoveryRights(ctx, tx)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("postgres.DurableRepo.LoadRecoverySnapshot commit: %w", err)
	}

	return &models.RecoverySnapshot{
		Stocks:      stocks,
		Memberships: memberships,
		Rights:      rights,
	}, nil
}

func (dr *DurableRepo) loadRecoveryStocks(ctx context.Context, tx pgx.Tx) ([]*models.ProductStock, error) {
	query, args, err := dr.sq.Select("product_id", "product_count", "total_stock", "updated_at").
		From("product_stock").
		OrderBy("product_id").
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("postgres.DurableRepo.loadRecoveryStocks query build: %w", err)
	}

	rows, err := tx.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("postgres.DurableRepo.loadRecoveryStocks execute: %w", err)
	}
	defer rows.Close()

	stocks := make([]*models.ProductStock, 0)
	for rows.Next() {
		stock := &models.ProductStock{}
		if err := rows.Scan(
			&stock.ProductID, &stock.ProductCount, &stock.TotalStock, &stock.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("postgres.DurableRepo.loadRecoveryStocks scan: %w", err)
		}

		stocks = append(stocks, stock)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres.DurableRepo.loadRecoveryStocks rows: %w", err)
	}

	return stocks, nil
}

func (dr *DurableRepo) loadRecoveryMemberships(ctx context.Context, tx pgx.Tx) ([]*models.QueueMembership, error) {
	query, args, err := dr.sq.Select(
		"id", "product_id", "user_id", "status", "quantity",
		"available_quantity", "current_token", "expires_at", "created_at", "updated_at",
	).
		From("queue_memberships").
		OrderBy("product_id", "updated_at", "id").
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("postgres.DurableRepo.loadRecoveryMemberships query build: %w", err)
	}

	rows, err := tx.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("postgres.DurableRepo.loadRecoveryMemberships execute: %w", err)
	}
	defer rows.Close()

	memberships := make([]*models.QueueMembership, 0)
	for rows.Next() {
		membership := &models.QueueMembership{}
		if err := rows.Scan(
			&membership.ID, &membership.ProductID, &membership.UserID,
			&membership.Status, &membership.Quantity, &membership.AvailableQuantity,
			&membership.CurrentToken, &membership.ExpiresAt,
			&membership.CreatedAt, &membership.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("postgres.DurableRepo.loadRecoveryMemberships scan: %w", err)
		}

		memberships = append(memberships, membership)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres.DurableRepo.loadRecoveryMemberships rows: %w", err)
	}

	return memberships, nil
}

func (dr *DurableRepo) loadRecoveryRights(ctx context.Context, tx pgx.Tx) ([]*models.Right, error) {
	query, args, err := dr.sq.Select(
		"token", "user_id", "product_id", "quantity", "status",
		"order_id", "created_at", "expires_at", "used_at",
	).
		From("rights").
		OrderBy("created_at", "token").
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("postgres.DurableRepo.loadRecoveryRights query build: %w", err)
	}

	rows, err := tx.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("postgres.DurableRepo.loadRecoveryRights execute: %w", err)
	}
	defer rows.Close()

	rights := make([]*models.Right, 0)
	for rows.Next() {
		right := &models.Right{}
		if err := rows.Scan(
			&right.Token, &right.UserID, &right.ProductID, &right.Quantity,
			&right.Status, &right.OrderID, &right.CreatedAt, &right.ExpiresAt, &right.UsedAt,
		); err != nil {
			return nil, fmt.Errorf("postgres.DurableRepo.loadRecoveryRights scan: %w", err)
		}

		rights = append(rights, right)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres.DurableRepo.loadRecoveryRights rows: %w", err)
	}

	return rights, nil
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

// IssueRightAndUpsertMembershipTx persists both sides of a newly issued right.
// Keeping them in one transaction prevents an ACTIVE right from being left
// without the membership that owns its token.
func (dr *DurableRepo) IssueRightAndUpsertMembershipTx(
	ctx context.Context,
	right *models.Right,
	membership *models.QueueMembership,
) error {
	log := logger.FromContext(ctx)

	tx, err := dr.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("postgres.DurableRepo.IssueRightAndUpsertMembershipTx begin: %w", err)
	}
	defer func() {
		if rbErr := tx.Rollback(ctx); rbErr != nil && !errors.Is(rbErr, pgx.ErrTxClosed) {
			log.Error("failed to rollback transaction", "error", rbErr)
		}
	}()

	query, args, err := dr.sq.Insert("rights").
		Columns("token", "user_id", "product_id", "quantity", "status", "order_id", "created_at", "expires_at", "used_at").
		Values(right.Token, right.UserID, right.ProductID, right.Quantity, right.Status, right.OrderID, right.CreatedAt, right.ExpiresAt, right.UsedAt).
		ToSql()
	if err != nil {
		return fmt.Errorf("postgres.DurableRepo.IssueRightAndUpsertMembershipTx right query build: %w", err)
	}

	if _, err := tx.Exec(ctx, query, args...); err != nil {
		return fmt.Errorf("postgres.DurableRepo.IssueRightAndUpsertMembershipTx right execute: %w", err)
	}

	if err := dr.upsertMembershipTx(ctx, tx, membership); err != nil {
		return fmt.Errorf("postgres.DurableRepo.IssueRightAndUpsertMembershipTx membership: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("postgres.DurableRepo.IssueRightAndUpsertMembershipTx commit: %w", err)
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

// UseRightTx atomically transitions an ACTIVE right to USED, decrements
// product_stock, writes a stock decrement outbox event, and finalizes the
// matching membership if it still owns the same token. The row lock makes
// duplicate payment webhooks idempotent.
func (dr *DurableRepo) UseRightTx(
	ctx context.Context,
	token string,
	orderID string,
	now time.Time,
) (*models.Right, bool, error) {
	log := logger.FromContext(ctx)

	tx, err := dr.pool.Begin(ctx)
	if err != nil {
		return nil, false, fmt.Errorf("postgres.DurableRepo.UseRightTx begin: %w", err)
	}
	defer func() {
		if rbErr := tx.Rollback(ctx); rbErr != nil && !errors.Is(rbErr, pgx.ErrTxClosed) {
			log.Error("failed to rollback transaction", "error", rbErr)
		}
	}()

	right, err := dr.getRightForUpdate(ctx, tx, token)
	if err != nil {
		return nil, false, err
	}

	switch right.Status {
	case models.RightStatusUsed:
		if right.OrderID == nil || *right.OrderID != orderID {
			return nil, false, models.ErrTokenUsed
		}
		return right, false, nil
	case models.RightStatusExpired:
		return nil, false, models.ErrTokenExpired
	case models.RightStatusActive:
		// Continue with the only allowed payment transition.
	default:
		return nil, false, models.ErrInvalidStatus
	}

	if !now.Before(right.ExpiresAt) {
		return nil, false, models.ErrTokenExpired
	}

	queryRight, argsRight, err := dr.sq.Update("rights").
		Set("status", models.RightStatusUsed).
		Set("order_id", orderID).
		Set("used_at", now).
		Where(sq.Eq{
			"token":  token,
			"status": models.RightStatusActive,
		}).
		ToSql()
	if err != nil {
		return nil, false, fmt.Errorf("postgres.DurableRepo.UseRightTx right query build: %w", err)
	}

	result, err := tx.Exec(ctx, queryRight, argsRight...)
	if err != nil {
		return nil, false, fmt.Errorf("postgres.DurableRepo.UseRightTx right execute: %w", err)
	}
	if result.RowsAffected() != 1 {
		return nil, false, fmt.Errorf("postgres.DurableRepo.UseRightTx right transition: %w", models.ErrInvalidStatus)
	}

	queryStock, argsStock, err := dr.sq.Update("product_stock").
		Set("product_count", sq.Expr("product_count - ?", right.Quantity)).
		Set("updated_at", now).
		Where(sq.And{
			sq.Eq{"product_id": right.ProductID},
			sq.GtOrEq{"product_count": right.Quantity},
		}).
		ToSql()
	if err != nil {
		return nil, false, fmt.Errorf("postgres.DurableRepo.UseRightTx stock query build: %w", err)
	}

	result, err = tx.Exec(ctx, queryStock, argsStock...)
	if err != nil {
		return nil, false, fmt.Errorf("postgres.DurableRepo.UseRightTx stock execute: %w", err)
	}
	if result.RowsAffected() != 1 {
		return nil, false, fmt.Errorf("postgres.DurableRepo.UseRightTx stock depleted: %w", models.ErrStockDepleted)
	}

	queryOutbox, argsOutbox, err := dr.sq.Insert("stock_decrement_outbox").
		Columns("id", "right_token", "order_id", "product_id", "quantity", "next_attempt_at", "created_at", "updated_at").
		Values(uuid.NewString(), right.Token, orderID, right.ProductID, right.Quantity, now, now, now).
		ToSql()
	if err != nil {
		return nil, false, fmt.Errorf("postgres.DurableRepo.UseRightTx outbox query build: %w", err)
	}

	if _, err = tx.Exec(ctx, queryOutbox, argsOutbox...); err != nil {
		return nil, false, fmt.Errorf("postgres.DurableRepo.UseRightTx outbox execute: %w", err)
	}

	queryMembership, argsMembership, err := dr.sq.Update("queue_memberships").
		Set("status", models.MembershipStatusPurchased).
		Set("available_quantity", nil).
		Set("current_token", nil).
		Set("expires_at", nil).
		Set("updated_at", now).
		Where(sq.Eq{
			"product_id":    right.ProductID,
			"user_id":       right.UserID,
			"status":        models.MembershipStatusRightActive,
			"current_token": token,
		}).
		ToSql()
	if err != nil {
		return nil, false, fmt.Errorf("postgres.DurableRepo.UseRightTx membership query build: %w", err)
	}

	if _, err = tx.Exec(ctx, queryMembership, argsMembership...); err != nil {
		return nil, false, fmt.Errorf("postgres.DurableRepo.UseRightTx membership execute: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		log.Error("failed to commit transaction", "error", err)
		return nil, false, fmt.Errorf("postgres.DurableRepo.UseRightTx commit: %w", err)
	}

	right.Status = models.RightStatusUsed
	right.OrderID = &orderID
	right.UsedAt = &now

	return right, true, nil
}

// ClaimStockDecrements leases due stock decrement events for delivery. Multiple
// API instances can run this safely because SKIP LOCKED gives each row to one
// worker at a time, and an expired lease can be claimed again later.
func (dr *DurableRepo) ClaimStockDecrements(
	ctx context.Context,
	now time.Time,
	leaseUntil time.Time,
	limit int,
) ([]models.StockDecrement, error) {
	if limit <= 0 {
		return nil, nil
	}

	query, args, err := dr.sq.Update("stock_decrement_outbox").
		Set("locked_until", leaseUntil).
		Set("attempts", sq.Expr("attempts + 1")).
		Set("updated_at", now).
		Where(`
			id IN (
				SELECT id
				FROM stock_decrement_outbox
				WHERE delivered_at IS NULL
					AND next_attempt_at <= ?
					AND (locked_until IS NULL OR locked_until <= ?)
				ORDER BY next_attempt_at, created_at
				FOR UPDATE SKIP LOCKED
				LIMIT ?
			)
		`, now, now, limit).
		Suffix(`
			RETURNING id::text, right_token, order_id, product_id, quantity, attempts,
				next_attempt_at, locked_until, delivered_at, last_error, created_at, updated_at
		`).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("postgres.DurableRepo.ClaimStockDecrements query build: %w", err)
	}

	rows, err := dr.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("postgres.DurableRepo.ClaimStockDecrements execute: %w", err)
	}
	defer rows.Close()

	events := make([]models.StockDecrement, 0)
	for rows.Next() {
		event := models.StockDecrement{}
		if err := rows.Scan(
			&event.ID,
			&event.RightToken,
			&event.OrderID,
			&event.ProductID,
			&event.Quantity,
			&event.Attempts,
			&event.NextAttemptAt,
			&event.LockedUntil,
			&event.DeliveredAt,
			&event.LastError,
			&event.CreatedAt,
			&event.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("postgres.DurableRepo.ClaimStockDecrements scan: %w", err)
		}

		events = append(events, event)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres.DurableRepo.ClaimStockDecrements rows: %w", err)
	}

	return events, nil
}

// MarkStockDecrementDelivered acknowledges a delivered event.
func (dr *DurableRepo) MarkStockDecrementDelivered(ctx context.Context, eventID string, now time.Time) error {
	query, args, err := dr.sq.Update("stock_decrement_outbox").
		Set("delivered_at", now).
		Set("locked_until", nil).
		Set("last_error", nil).
		Set("updated_at", now).
		Where(sq.Eq{"id": eventID}).
		Where(sq.Eq{"delivered_at": nil}).
		ToSql()
	if err != nil {
		return fmt.Errorf("postgres.DurableRepo.MarkStockDecrementDelivered query build: %w", err)
	}

	if _, err := dr.pool.Exec(ctx, query, args...); err != nil {
		return fmt.Errorf("postgres.DurableRepo.MarkStockDecrementDelivered execute: %w", err)
	}

	return nil
}

// RescheduleStockDecrement releases a failed event for a later retry.
func (dr *DurableRepo) RescheduleStockDecrement(
	ctx context.Context,
	eventID string,
	nextAttemptAt time.Time,
	lastError string,
	now time.Time,
) error {
	query, args, err := dr.sq.Update("stock_decrement_outbox").
		Set("next_attempt_at", nextAttemptAt).
		Set("locked_until", nil).
		Set("last_error", lastError).
		Set("updated_at", now).
		Where(sq.Eq{"id": eventID}).
		Where(sq.Eq{"delivered_at": nil}).
		ToSql()
	if err != nil {
		return fmt.Errorf("postgres.DurableRepo.RescheduleStockDecrement query build: %w", err)
	}

	if _, err := dr.pool.Exec(ctx, query, args...); err != nil {
		return fmt.Errorf("postgres.DurableRepo.RescheduleStockDecrement execute: %w", err)
	}

	return nil
}

// ExpireRightAndUpsertMembershipTx atomically invalidates an unpaid right and
// persists the membership state chosen by the service layer.
func (dr *DurableRepo) ExpireRightAndUpsertMembershipTx(
	ctx context.Context,
	token string,
	membership *models.QueueMembership,
) (*models.Right, bool, error) {
	log := logger.FromContext(ctx)

	tx, err := dr.pool.Begin(ctx)
	if err != nil {
		return nil, false, fmt.Errorf("postgres.DurableRepo.ExpireRightAndUpsertMembershipTx begin: %w", err)
	}
	defer func() {
		if rbErr := tx.Rollback(ctx); rbErr != nil && !errors.Is(rbErr, pgx.ErrTxClosed) {
			log.Error("failed to rollback transaction", "error", rbErr)
		}
	}()

	right, err := dr.getRightForUpdate(ctx, tx, token)
	if err != nil {
		return nil, false, err
	}

	switch right.Status {
	case models.RightStatusUsed, models.RightStatusExpired:
		return right, false, nil
	case models.RightStatusActive:
		// Continue with the only allowed expiration transition.
	default:
		return nil, false, models.ErrInvalidStatus
	}

	queryRight, argsRight, err := dr.sq.Update("rights").
		Set("status", models.RightStatusExpired).
		Where(sq.Eq{
			"token":  token,
			"status": models.RightStatusActive,
		}).
		ToSql()
	if err != nil {
		return nil, false, fmt.Errorf("postgres.DurableRepo.ExpireRightAndUpsertMembershipTx right query build: %w", err)
	}

	result, err := tx.Exec(ctx, queryRight, argsRight...)
	if err != nil {
		return nil, false, fmt.Errorf("postgres.DurableRepo.ExpireRightAndUpsertMembershipTx right execute: %w", err)
	}
	if result.RowsAffected() != 1 {
		return nil, false, fmt.Errorf("postgres.DurableRepo.ExpireRightAndUpsertMembershipTx right transition: %w", models.ErrInvalidStatus)
	}

	if err := dr.upsertMembershipTx(ctx, tx, membership); err != nil {
		return nil, false, err
	}

	if err := tx.Commit(ctx); err != nil {
		log.Error("failed to commit transaction", "error", err)
		return nil, false, fmt.Errorf("postgres.DurableRepo.ExpireRightAndUpsertMembershipTx commit: %w", err)
	}

	right.Status = models.RightStatusExpired
	return right, true, nil
}

func (dr *DurableRepo) getRightForUpdate(ctx context.Context, tx pgx.Tx, token string) (*models.Right, error) {
	query, args, err := dr.sq.Select(
		"token", "user_id", "product_id", "quantity", "status",
		"order_id", "created_at", "expires_at", "used_at",
	).
		From("rights").
		Where(sq.Eq{"token": token}).
		Suffix("FOR UPDATE").
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("postgres.DurableRepo.getRightForUpdate query build: %w", err)
	}

	var right models.Right
	err = tx.QueryRow(ctx, query, args...).Scan(
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
			return nil, fmt.Errorf("postgres.DurableRepo.getRightForUpdate not found: %w", models.ErrTokenNotFound)
		}
		return nil, fmt.Errorf("postgres.DurableRepo.getRightForUpdate scan: %w", err)
	}

	return &right, nil
}

func (dr *DurableRepo) upsertMembershipTx(
	ctx context.Context,
	tx pgx.Tx,
	membership *models.QueueMembership,
) error {
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
		return fmt.Errorf("postgres.DurableRepo.upsertMembershipTx query build: %w", err)
	}

	if _, err := tx.Exec(ctx, query, args...); err != nil {
		return fmt.Errorf("postgres.DurableRepo.upsertMembershipTx execute: %w", err)
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

// ExpireRights marks the given ACTIVE rights as EXPIRED in one statement.
//
// Recovery uses it to settle rights no live membership points at any more. Such
// a right holds nothing — the membership that owned it is already terminal — but
// left ACTIVE it would keep failing the consistency check on every restart.
func (dr *DurableRepo) ExpireRights(ctx context.Context, tokens []string) error {
	if len(tokens) == 0 {
		return nil
	}

	query, args, err := dr.sq.Update("rights").
		Set("status", models.RightStatusExpired).
		Where(sq.Eq{"token": tokens, "status": models.RightStatusActive}).
		ToSql()
	if err != nil {
		return fmt.Errorf("postgres.DurableRepo.ExpireRights query build: %w", err)
	}

	if _, err = dr.pool.Exec(ctx, query, args...); err != nil {
		return fmt.Errorf("postgres.DurableRepo.ExpireRights execute: %w", err)
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
