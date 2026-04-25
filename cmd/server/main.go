package main

import (
	"context"
	"errors"
	"io/fs"
	"log"
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
	dbURL := mustEnv("DB_URL")
	addr := envOr("SERVER_ADDR", ":8080")
	sessionTTL := 30 * 24 * time.Hour

	ctx, stop := signal.NotifyContext(context.Background(),
		syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	store, err := storage.New(ctx, dbURL)
	if err != nil {
		log.Fatalf("storage.New: %v", err)
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
	})
	if err != nil {
		log.Fatalf("handler.New: %v", err)
	}

	staticFS, err := fs.Sub(web.FS, "static")
	if err != nil {
		log.Fatalf("fs.Sub: %v", err)
	}

	mux := http.NewServeMux()
	mux.Handle("GET /static/",
		http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))
	mux.Handle("/", h.Routes())

	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}

	serverErr := make(chan error, 1)
	go func() {
		log.Printf("Listening on %s", addr)
		err := srv.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	}()

	select {
	case err := <-serverErr:
		log.Fatalf("ListenAndServe: %v", err)
	case <-ctx.Done():
		log.Println("Shutdown signal received")
	}

	shutdownCtx, cancel := context.WithTimeout(
		context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("srv.Shutdown: %v", err)
	}
	log.Println("Server stopped")
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
				log.Printf("CleanExpired: %v", err)
			}
			cancel()
		}
	}
}

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("required env var %s is not set", key)
	}
	return v
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
