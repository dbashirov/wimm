package storage

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Storage struct {
	Pool       *pgxpool.Pool
	Users      *UserStore
	Sessions   *SessionStore
	Wallets    *WalletStore
	Categories *CategoryStore
	Operations *OperationStore
}

func New(ctx context.Context, dsn string) (*Storage, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("pgxpool.New: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("pool.Ping: %w", err)
	}
	s := &Storage{Pool: pool}
	s.Users = &UserStore{pool}
	s.Sessions = &SessionStore{pool}
	s.Wallets = &WalletStore{pool}
	s.Categories = &CategoryStore{pool}
	s.Operations = &OperationStore{pool}
	return s, nil
}

// NewFromPool создаёт Storage из готового пула (используется в тестах).
func NewFromPool(pool *pgxpool.Pool) *Storage {
	s := &Storage{Pool: pool}
	s.Users = &UserStore{pool}
	s.Sessions = &SessionStore{pool}
	s.Wallets = &WalletStore{pool}
	s.Categories = &CategoryStore{pool}
	s.Operations = &OperationStore{pool}
	return s
}

func (s *Storage) Close() {
	s.Pool.Close()
}
