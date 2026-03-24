package session

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/dbashirov/wimm/internal/model"
	"github.com/dbashirov/wimm/internal/storage"
)

const cookieName = "session_id"

type Manager struct {
	store *storage.SessionStore
	ttl   time.Duration
}

func NewManager(store *storage.SessionStore, ttl time.Duration) *Manager {
	return &Manager{store: store, ttl: ttl}
}

func (m *Manager) Create(ctx context.Context, w http.ResponseWriter, userID int64) error {
	expiresAt := time.Now().Add(m.ttl)
	sess, err := m.store.Create(ctx, userID, expiresAt)
	if err != nil {
		return err
	}
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    sess.ID,
		Path:     "/",
		Expires:  sess.ExpiresAt,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	})
	return nil
}

func (m *Manager) Get(ctx context.Context, r *http.Request) (*model.Session, error) {
	cookie, err := r.Cookie(cookieName)
	if err != nil {
		return nil, ErrNoSession
	}
	sess, err := m.store.Get(ctx, cookie.Value)
	if errors.Is(err, storage.ErrNotFound) {
		return nil, ErrNoSession
	}
	return sess, err
}

func (m *Manager) Delete(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
	cookie, err := r.Cookie(cookieName)
	if err != nil {
		return nil
	}
	if err := m.store.Delete(ctx, cookie.Value); err != nil {
		return err
	}
	http.SetCookie(w, &http.Cookie{
		Name:    cookieName,
		Value:   "",
		Path:    "/",
		MaxAge:  -1,
		Expires: time.Unix(0, 0),
	})
	return nil
}

func (m *Manager) CleanExpired(ctx context.Context) error {
	return m.store.DeleteExpired(ctx)
}

var ErrNoSession = errors.New("no valid session")
