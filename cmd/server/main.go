package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/dbashirov/wimm/internal/handler"
	"github.com/dbashirov/wimm/internal/service"
	"github.com/dbashirov/wimm/internal/session"
	"github.com/dbashirov/wimm/internal/storage"
	"github.com/dbashirov/wimm/migrations"
	"github.com/dbashirov/wimm/web"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

func main() {
	dbURL := mustEnv("DB_URL")
	addr := envOr("SERVER_ADDR", ":8080")
	sessionTTL := 30 * 24 * time.Hour

	// Run migrations
	sqlDB, err := sql.Open("pgx", dbURL)
	if err != nil {
		log.Fatalf("sql.Open: %v", err)
	}
	if err := runMigrations(sqlDB); err != nil {
		log.Fatalf("migrations: %v", err)
	}
	sqlDB.Close()

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

func runMigrations(db *sql.DB) error {
	provider, err := goose.NewProvider(goose.DialectPostgres, db, migrations.FS)
	if err != nil {
		return fmt.Errorf("goose.NewProvider: %w", err)
	}
	results, err := provider.Up(context.Background())
	if err != nil {
		return fmt.Errorf("provider.Up: %w", err)
	}
	for _, r := range results {
		log.Printf("migration: %s applied in %s", r.Source.Path, r.Duration)
	}
	return nil
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
