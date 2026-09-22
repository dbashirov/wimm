package main

import (
	"context"
	"errors"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/dbashirov/wimm/internal/handler"
	"github.com/dbashirov/wimm/internal/service"
	"github.com/dbashirov/wimm/internal/session"
	"github.com/dbashirov/wimm/internal/storage"
	"github.com/dbashirov/wimm/web"
)

func main() {
	setupLogger()

	dbURL := mustEnv("DB_URL")
	addr := envOr("SERVER_ADDR", ":8080")
	sessionTTL := 30 * 24 * time.Hour

	ctx, stop := signal.NotifyContext(context.Background(),
		syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	store, err := storage.New(ctx, dbURL)
	if err != nil {
		slog.Error("storage.New", "err", err)
		os.Exit(1)
	}
	defer store.Close()

	svc := service.New(store)
	secureCookie := os.Getenv("COOKIE_INSECURE") != "true"
	sm := session.NewManager(store.Sessions, sessionTTL, secureCookie)

	go cleanSessionsLoop(ctx, sm)

	h, err := handler.New(svc, sm, web.FS, handler.Config{
		CSRFSecret:    os.Getenv("CSRF_SECRET"),
		SecureCookies: secureCookie,
		HSTS:          os.Getenv("HSTS") == "true",
		// по умолчанию — nginx на том же хосте (docs/DEPLOY.md)
		TrustedProxies: envOr("TRUSTED_PROXIES", "127.0.0.1,::1"),
	})
	if err != nil {
		slog.Error("handler.New", "err", err)
		os.Exit(1)
	}

	srv := &http.Server{
		Addr:              addr,
		Handler:           h.Routes(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    1 << 20,
		ErrorLog:          log.New(slogWriter{}, "", 0),
	}

	serverErr := make(chan error, 1)
	go func() {
		slog.Info("listening", "addr", addr)
		err := srv.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	}()

	select {
	case err := <-serverErr:
		slog.Error("ListenAndServe", "err", err)
		os.Exit(1)
	case <-ctx.Done():
		slog.Info("shutdown signal received")
	}

	shutdownCtx, cancel := context.WithTimeout(
		context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("srv.Shutdown", "err", err)
	}
	slog.Info("server stopped")
}

// setupLogger конфигурирует slog: JSON-handler в проде (LOG_FORMAT=json),
// человекочитаемый text-handler по умолчанию. Уровень из LOG_LEVEL.
func setupLogger() {
	level := slog.LevelInfo
	switch os.Getenv("LOG_LEVEL") {
	case "debug", "DEBUG":
		level = slog.LevelDebug
	case "warn", "WARN":
		level = slog.LevelWarn
	case "error", "ERROR":
		level = slog.LevelError
	}
	opts := &slog.HandlerOptions{Level: level}
	var h slog.Handler
	if os.Getenv("LOG_FORMAT") == "json" {
		h = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		h = slog.NewTextHandler(os.Stdout, opts)
	}
	slog.SetDefault(slog.New(h))
}

// slogWriter перенаправляет вывод стандартного log.Logger в slog.
// Используется для http.Server.ErrorLog, чтобы все ошибки шли единым форматом.
type slogWriter struct{}

func (slogWriter) Write(p []byte) (int, error) {
	slog.Error("http", "msg", string(p))
	return len(p), nil
}

func cleanSessionsLoop(ctx context.Context, sm *session.Manager) {
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			cleanCtx, cancel := context.WithTimeout(
				context.Background(), 30*time.Second)
			if err := sm.CleanExpired(cleanCtx); err != nil {
				slog.Error("CleanExpired", "err", err)
			}
			cancel()
		}
	}
}

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		slog.Error("required env var not set", "key", key)
		os.Exit(1)
	}
	return v
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
