package storage

import (
	"context"
	"fmt"
	"time"

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
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("pgxpool.ParseConfig: %w", err)
	}
	cfg.MaxConns = 25
	cfg.MinConns = 2
	cfg.MaxConnLifetime = time.Hour
	cfg.MaxConnIdleTime = 30 * time.Minute
	cfg.HealthCheckPeriod = time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("pgxpool.NewWithConfig: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("pool.Ping: %w", err)
	}
	return newFromPool(pool), nil
}

// NewFromPool создаёт Storage из готового пула (используется в тестах).
func NewFromPool(pool *pgxpool.Pool) *Storage {
	return newFromPool(pool)
}

func newFromPool(pool *pgxpool.Pool) *Storage {
	s := &Storage{Pool: pool}
	s.Users = &UserStore{pool}
	s.Sessions = &SessionStore{pool}
	s.Wallets = &WalletStore{pool}
	s.Categories = &CategoryStore{pool}
	s.Operations = &OperationStore{pool}
	return s
}

// Ping проверяет соединение с БД (для healthcheck).
func (s *Storage) Ping(ctx context.Context) error {
	return s.Pool.Ping(ctx)
}

func (s *Storage) Close() {
	s.Pool.Close()
}
