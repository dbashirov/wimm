package session

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/dbashirov/wimm/internal/model"
	"github.com/dbashirov/wimm/internal/storage"
	"github.com/google/uuid"
)

const cookieName = "session_id"

type Manager struct {
	store  *storage.SessionStore
	ttl    time.Duration
	secure bool
}

func NewManager(store *storage.SessionStore, ttl time.Duration, secure bool) *Manager {
	return &Manager{store: store, ttl: ttl, secure: secure}
}

func (m *Manager) Create(ctx context.Context, w http.ResponseWriter, userID uuid.UUID) error {
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
		Secure:   m.secure,
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

// Delete удаляет сессию. Cookie очищается до обращения к БД: при сбое
// удаления браузер всё равно выходит, а ошибка возвращается вызывающему.
func (m *Manager) Delete(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
	cookie, err := r.Cookie(cookieName)
	if err != nil {
		return nil
	}
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		Secure:   m.secure,
	})
	return m.store.Delete(ctx, cookie.Value)
}

func (m *Manager) CleanExpired(ctx context.Context) error {
	return m.store.DeleteExpired(ctx)
}

var ErrNoSession = errors.New("no valid session")
