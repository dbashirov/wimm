package storage

import (
	"context"
	"errors"
	"fmt"

	"github.com/dbashirov/wimm/internal/model"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type UserStore struct {
	pool *pgxpool.Pool
}

// Create создаёт пользователя вместе со стартовым кошельком (по умолчанию)
// и категориями — одной транзакцией, чтобы не оставить полупустой аккаунт.
func (s *UserStore) Create(
	ctx context.Context,
	email, passwordHash string,
	seed model.UserSeed,
) (*model.User, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("UserStore.Create begin: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	var u model.User
	err = tx.QueryRow(ctx,
		`INSERT INTO users(email, password_hash) VALUES($1, $2)
		 RETURNING id, email, password_hash, created_at`,
		email, passwordHash,
	).Scan(&u.ID, &u.Email, &u.PasswordHash, &u.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("UserStore.Create: %w", err)
	}

	_, err = tx.Exec(ctx,
		`INSERT INTO wallets(user_id, name, currency, description, is_default)
		 VALUES($1, $2, $3, $4, true)`,
		u.ID, seed.Wallet.Name, seed.Wallet.Currency, seed.Wallet.Description,
	)
	if err != nil {
		return nil, fmt.Errorf("UserStore.Create wallet: %w", err)
	}

	if len(seed.Categories) > 0 {
		names := make([]string, len(seed.Categories))
		types := make([]string, len(seed.Categories))
		descriptions := make([]string, len(seed.Categories))
		for i, c := range seed.Categories {
			names[i], types[i], descriptions[i] = c.Name, string(c.Type), c.Description
		}
		_, err = tx.Exec(ctx,
			`INSERT INTO categories(user_id, name, type, description)
			 SELECT $1, name, type::operation_type, description
			 FROM unnest($2::text[], $3::text[], $4::text[]) AS c(name, type, description)`,
			u.ID, names, types, descriptions,
		)
		if err != nil {
			return nil, fmt.Errorf("UserStore.Create categories: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("UserStore.Create commit: %w", err)
	}
	return &u, nil
}

func (s *UserStore) GetByEmail(ctx context.Context, email string) (*model.User, error) {
	var u model.User
	err := s.pool.QueryRow(ctx,
		`SELECT id, email, password_hash, created_at FROM users WHERE email=$1`,
		email,
	).Scan(&u.ID, &u.Email, &u.PasswordHash, &u.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("UserStore.GetByEmail: %w", err)
	}
	return &u, nil
}
