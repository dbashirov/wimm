package storage

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/dbashirov/wimm/internal/model"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type SessionStore struct {
	pool *pgxpool.Pool
}

// newSessionID генерирует криптографически стойкий ID сессии:
// 32 случайных байта в base64 URL-safe без padding (43 символа).
func newSessionID() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("rand.Read: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func (s *SessionStore) Create(
	ctx context.Context,
	userID uuid.UUID,
	expiresAt time.Time,
) (*model.Session, error) {
	id, err := newSessionID()
	if err != nil {
		return nil, fmt.Errorf("SessionStore.Create id: %w", err)
	}
	var sess model.Session
	err = s.pool.QueryRow(ctx,
		`INSERT INTO sessions(id, user_id, expires_at) VALUES($1, $2, $3)
		 RETURNING id, user_id, expires_at`,
		id, userID, expiresAt,
	).Scan(&sess.ID, &sess.UserID, &sess.ExpiresAt)
	if err != nil {
		return nil, fmt.Errorf("SessionStore.Create: %w", err)
	}
	return &sess, nil
}

func (s *SessionStore) Get(ctx context.Context, id string) (*model.Session, error) {
	var sess model.Session
	err := s.pool.QueryRow(ctx,
		`SELECT id, user_id, expires_at FROM sessions WHERE id=$1 AND expires_at > NOW()`,
		id,
	).Scan(&sess.ID, &sess.UserID, &sess.ExpiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("SessionStore.Get: %w", err)
	}
	return &sess, nil
}

func (s *SessionStore) Delete(ctx context.Context, id string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM sessions WHERE id=$1`, id)
	return err
}

func (s *SessionStore) DeleteExpired(ctx context.Context) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM sessions WHERE expires_at < NOW()`)
	return err
}
