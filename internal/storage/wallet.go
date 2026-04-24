package storage

import (
	"context"
	"errors"
	"fmt"

	"github.com/dbashirov/wimm/internal/model"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type WalletStore struct {
	pool *pgxpool.Pool
}

func (s *WalletStore) ListByUser(ctx context.Context, userID uuid.UUID) ([]model.Wallet, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, user_id, name, currency, description, created_at
		 FROM wallets WHERE user_id=$1 ORDER BY name`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("WalletStore.ListByUser: %w", err)
	}
	defer rows.Close()
	var wallets []model.Wallet
	for rows.Next() {
		var w model.Wallet
		if err := rows.Scan(
			&w.ID, &w.UserID, &w.Name, &w.Currency, &w.Description, &w.CreatedAt,
		); err != nil {
			return nil, err
		}
		wallets = append(wallets, w)
	}
	return wallets, rows.Err()
}

func (s *WalletStore) ListWithBalance(ctx context.Context, userID uuid.UUID) ([]model.WalletBalance, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT w.id, w.user_id, w.name, w.currency, w.description, w.created_at,
		        COALESCE(SUM(CASE WHEN o.type=$2 THEN o.amount ELSE 0 END), 0)::float8,
		        COALESCE(SUM(CASE WHEN o.type=$3 THEN o.amount ELSE 0 END), 0)::float8
		 FROM wallets w
		 LEFT JOIN operations o ON o.wallet_id = w.id AND o.user_id = w.user_id
		 WHERE w.user_id = $1
		 GROUP BY w.id
		 ORDER BY w.name`,
		userID, model.TypeIncome, model.TypeExpense,
	)
	if err != nil {
		return nil, fmt.Errorf("WalletStore.ListWithBalance: %w", err)
	}
	defer rows.Close()
	var result []model.WalletBalance
	for rows.Next() {
		var wb model.WalletBalance
		if err := rows.Scan(
			&wb.ID, &wb.UserID, &wb.Name, &wb.Currency, &wb.Description, &wb.CreatedAt,
			&wb.Income, &wb.Expense,
		); err != nil {
			return nil, err
		}
		wb.Balance = wb.Income - wb.Expense
		result = append(result, wb)
	}
	return result, rows.Err()
}

func (s *WalletStore) Create(ctx context.Context, userID uuid.UUID, name, currency, description string) (*model.Wallet, error) {
	var w model.Wallet
	err := s.pool.QueryRow(ctx,
		`INSERT INTO wallets(user_id, name, currency, description) VALUES($1, $2, $3, $4)
		 RETURNING id, user_id, name, currency, description, created_at`,
		userID, name, currency, description,
	).Scan(&w.ID, &w.UserID, &w.Name, &w.Currency, &w.Description, &w.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("WalletStore.Create: %w", err)
	}
	return &w, nil
}

func (s *WalletStore) GetByID(ctx context.Context, id uuid.UUID, userID uuid.UUID) (*model.Wallet, error) {
	var w model.Wallet
	err := s.pool.QueryRow(ctx,
		`SELECT id, user_id, name, currency, description, created_at
		 FROM wallets WHERE id=$1 AND user_id=$2`,
		id, userID,
	).Scan(&w.ID, &w.UserID, &w.Name, &w.Currency, &w.Description, &w.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("WalletStore.GetByID: %w", err)
	}
	return &w, nil
}

func (s *WalletStore) Update(ctx context.Context, id uuid.UUID, userID uuid.UUID, name, currency, description string) (*model.Wallet, error) {
	var w model.Wallet
	err := s.pool.QueryRow(ctx,
		`UPDATE wallets SET name=$1, currency=$2, description=$3
		 WHERE id=$4 AND user_id=$5
		 RETURNING id, user_id, name, currency, description, created_at`,
		name, currency, description, id, userID,
	).Scan(&w.ID, &w.UserID, &w.Name, &w.Currency, &w.Description, &w.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("WalletStore.Update: %w", err)
	}
	return &w, nil
}

func (s *WalletStore) Delete(ctx context.Context, id uuid.UUID, userID uuid.UUID) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM wallets WHERE id=$1 AND user_id=$2`, id, userID)
	return checkRowsAffected(tag, err)
}
