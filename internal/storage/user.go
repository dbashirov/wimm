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

func (s *UserStore) Create(ctx context.Context, email, passwordHash string) (*model.User, error) {
	var u model.User
	err := s.pool.QueryRow(ctx,
		`INSERT INTO users(email, password_hash) VALUES($1, $2)
		 RETURNING id, email, password_hash, created_at`,
		email, passwordHash,
	).Scan(&u.ID, &u.Email, &u.PasswordHash, &u.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("UserStore.Create: %w", err)
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
