package main

import (
	"context"
	"log"
	"net/http"
	"os"
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

	// App storage
	ctx := context.Background()
	store, err := storage.New(ctx, dbURL)
	if err != nil {
		log.Fatalf("storage.New: %v", err)
	}
	defer store.Close()

	svc := service.New(store)
	sm := session.NewManager(store.Sessions, sessionTTL)

	// Background: clean expired sessions hourly
	go func() {
		for range time.Tick(time.Hour) {
			if err := sm.CleanExpired(context.Background()); err != nil {
				log.Printf("CleanExpired: %v", err)
			}
		}
	}()

	h, err := handler.New(svc, sm, web.FS)
	if err != nil {
		log.Fatalf("handler.New: %v", err)
	}

	// Serve static files
	mux := http.NewServeMux()
	mux.Handle("GET /static/", http.FileServer(http.FS(web.FS)))
	mux.Handle("/", h.Routes())

	log.Printf("Listening on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("ListenAndServe: %v", err)
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
