package handler

import (
	"context"
	"log/slog"
	"net/http"
	"time"
)

// getHealth — liveness-проба. Возвращает 200 если процесс жив.
func (h *Handler) getHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

// getReady — readiness-проба. Возвращает 200 если БД доступна.
func (h *Handler) getReady(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := h.svc.Ping(ctx); err != nil {
		// /ready публичный: текст ошибки драйвера (адрес, имя БД) только в лог
		slog.Error("getReady Ping",
			"request_id", requestIDFromCtx(r.Context()), "err", err)
		http.Error(w, "not ready", http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ready"))
}
