package handler

import (
	"context"
	"errors"
	"net/http"

	"github.com/dbashirov/wimm/internal/session"
)

type contextKey string

const userIDKey contextKey = "userID"

func userIDFromCtx(ctx context.Context) int64 {
	id, _ := ctx.Value(userIDKey).(int64)
	return id
}

func (h *Handler) auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sess, err := h.sessions.Get(r.Context(), r)
		if err != nil {
			if errors.Is(err, session.ErrNoSession) {
				http.Redirect(w, r, "/login", http.StatusFound)
				return
			}
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		ctx := context.WithValue(r.Context(), userIDKey, sess.UserID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
