package storage

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/dbashirov/wimm/internal/model"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type OperationStore struct {
	pool *pgxpool.Pool
}

const operationSelect = `
	SELECT o.id, o.user_id, o.wallet_id, o.category_id, o.type,
	       o.amount::float8, o.operated_at, o.description, o.created_at,
	       w.name, c.name, w.currency
	FROM operations o
	JOIN wallets w    ON w.id = o.wallet_id
	JOIN categories c ON c.id = o.category_id`

func (s *OperationStore) List(ctx context.Context, f model.OperationFilter) ([]model.Operation, error) {
	q := operationSelect + ` WHERE o.user_id = $1`
	args := []any{f.UserID}
	n := 2

	if f.WalletID != 0 {
		q += fmt.Sprintf(" AND o.wallet_id = $%d", n)
		args = append(args, f.WalletID)
		n++
	}
	if f.CategoryID != 0 {
		q += fmt.Sprintf(" AND o.category_id = $%d", n)
		args = append(args, f.CategoryID)
		n++
	}
	if f.Type != "" {
		q += fmt.Sprintf(" AND o.type = $%d", n)
		args = append(args, f.Type)
		n++
	}
	if !f.DateFrom.IsZero() {
		q += fmt.Sprintf(" AND o.operated_at >= $%d", n)
		args = append(args, f.DateFrom)
		n++
	}
	if !f.DateTo.IsZero() {
		q += fmt.Sprintf(" AND o.operated_at <= $%d", n)
		args = append(args, f.DateTo)
		n++
	}

	pageSize := 50
	offset := 0
	if f.Page > 1 {
		offset = (f.Page - 1) * pageSize
	}
	q += fmt.Sprintf(" ORDER BY o.operated_at DESC, o.id DESC LIMIT %d OFFSET %d", pageSize, offset)

	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("OperationStore.List: %w", err)
	}
	defer rows.Close()

	var ops []model.Operation
	for rows.Next() {
		var o model.Operation
		if err := rows.Scan(
			&o.ID, &o.UserID, &o.WalletID, &o.CategoryID, &o.Type,
			&o.Amount, &o.OperatedAt, &o.Description, &o.CreatedAt,
			&o.WalletName, &o.CategoryName, &o.Currency,
		); err != nil {
			return nil, err
		}
		ops = append(ops, o)
	}
	return ops, rows.Err()
}

func (s *OperationStore) Create(ctx context.Context, userID, walletID, categoryID int64, opType string, amount float64, operatedAt time.Time, description string) (*model.Operation, error) {
	var o model.Operation
	err := s.pool.QueryRow(ctx,
		`INSERT INTO operations(user_id, wallet_id, category_id, type, amount, operated_at, description)
		 VALUES($1, $2, $3, $4, $5, $6, $7)
		 RETURNING id, user_id, wallet_id, category_id, type, amount::float8, operated_at, description, created_at`,
		userID, walletID, categoryID, opType, amount, operatedAt, description,
	).Scan(
		&o.ID, &o.UserID, &o.WalletID, &o.CategoryID, &o.Type,
		&o.Amount, &o.OperatedAt, &o.Description, &o.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("OperationStore.Create: %w", err)
	}
	return &o, nil
}

func (s *OperationStore) GetByID(ctx context.Context, id, userID int64) (*model.Operation, error) {
	var o model.Operation
	err := s.pool.QueryRow(ctx,
		operationSelect+` WHERE o.id=$1 AND o.user_id=$2`,
		id, userID,
	).Scan(
		&o.ID, &o.UserID, &o.WalletID, &o.CategoryID, &o.Type,
		&o.Amount, &o.OperatedAt, &o.Description, &o.CreatedAt,
		&o.WalletName, &o.CategoryName, &o.Currency,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("OperationStore.GetByID: %w", err)
	}
	return &o, nil
}

func (s *OperationStore) Update(ctx context.Context, id, userID, walletID, categoryID int64, opType string, amount float64, operatedAt time.Time, description string) (*model.Operation, error) {
	var o model.Operation
	err := s.pool.QueryRow(ctx,
		`UPDATE operations
		 SET wallet_id=$1, category_id=$2, type=$3, amount=$4, operated_at=$5, description=$6
		 WHERE id=$7 AND user_id=$8
		 RETURNING id, user_id, wallet_id, category_id, type, amount::float8, operated_at, description, created_at`,
		walletID, categoryID, opType, amount, operatedAt, description, id, userID,
	).Scan(
		&o.ID, &o.UserID, &o.WalletID, &o.CategoryID, &o.Type,
		&o.Amount, &o.OperatedAt, &o.Description, &o.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("OperationStore.Update: %w", err)
	}
	return &o, nil
}

func (s *OperationStore) Delete(ctx context.Context, id, userID int64) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM operations WHERE id=$1 AND user_id=$2`, id, userID)
	return checkRowsAffected(tag, err)
}

func (s *OperationStore) DashboardData(ctx context.Context, userID int64) ([]model.CategoryAmount, []model.MonthlyTotal, error) {
	// Expenses by category (current month)
	catRows, err := s.pool.Query(ctx, `
		SELECT c.name, COALESCE(SUM(o.amount), 0)::float8
		FROM categories c
		JOIN operations o ON o.category_id = c.id AND o.user_id = c.user_id
		WHERE c.user_id = $1 AND o.type = 'expense'
		  AND date_trunc('month', o.operated_at) = date_trunc('month', CURRENT_DATE)
		GROUP BY c.name
		ORDER BY SUM(o.amount) DESC`,
		userID,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("DashboardData categories: %w", err)
	}
	defer catRows.Close()
	var catAmounts []model.CategoryAmount
	for catRows.Next() {
		var ca model.CategoryAmount
		if err := catRows.Scan(&ca.Name, &ca.Amount); err != nil {
			return nil, nil, err
		}
		catAmounts = append(catAmounts, ca)
	}
	if err := catRows.Err(); err != nil {
		return nil, nil, err
	}

	// Monthly income vs expense (last 6 months)
	monthRows, err := s.pool.Query(ctx, `
		SELECT to_char(date_trunc('month', operated_at), 'YYYY-MM'),
		       SUM(CASE WHEN type='income'  THEN amount ELSE 0 END)::float8,
		       SUM(CASE WHEN type='expense' THEN amount ELSE 0 END)::float8
		FROM operations
		WHERE user_id = $1
		  AND operated_at >= date_trunc('month', CURRENT_DATE) - INTERVAL '5 months'
		GROUP BY date_trunc('month', operated_at)
		ORDER BY date_trunc('month', operated_at)`,
		userID,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("DashboardData monthly: %w", err)
	}
	defer monthRows.Close()
	var monthly []model.MonthlyTotal
	for monthRows.Next() {
		var mt model.MonthlyTotal
		if err := monthRows.Scan(&mt.Month, &mt.Income, &mt.Expense); err != nil {
			return nil, nil, err
		}
		monthly = append(monthly, mt)
	}
	return catAmounts, monthly, monthRows.Err()
}
