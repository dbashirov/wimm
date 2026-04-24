package storage_test

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"

	"github.com/dbashirov/wimm/migrations"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
)

var testPool *pgxpool.Pool

func TestMain(m *testing.M) {
	ctx := context.Background()

	ctr, err := tcpostgres.Run(ctx,
		"postgres:16-alpine",
		tcpostgres.WithDatabase("wimm_test"),
		tcpostgres.WithUsername("test"),
		tcpostgres.WithPassword("test"),
		tcpostgres.BasicWaitStrategies(),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "testcontainers: %v\n", err)
		os.Exit(1)
	}
	defer ctr.Terminate(ctx) //nolint:errcheck

	dsn, err := ctr.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		fmt.Fprintf(os.Stderr, "ConnectionString: %v\n", err)
		os.Exit(1)
	}

	// Migrations via goose
	sqlDB, err := sql.Open("pgx", dsn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "sql.Open: %v\n", err)
		os.Exit(1)
	}
	provider, err := goose.NewProvider(goose.DialectPostgres, sqlDB, migrations.FS)
	if err != nil {
		fmt.Fprintf(os.Stderr, "goose.NewProvider: %v\n", err)
		os.Exit(1)
	}
	if _, err := provider.Up(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "goose Up: %v\n", err)
		os.Exit(1)
	}
	sqlDB.Close()

	testPool, err = pgxpool.New(ctx, dsn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "pgxpool.New: %v\n", err)
		os.Exit(1)
	}
	defer testPool.Close()

	os.Exit(m.Run())
}

// cleanDB очищает все таблицы между тестами.
func cleanDB(t *testing.T) {
	t.Helper()
	_, err := testPool.Exec(context.Background(),
		`TRUNCATE users, sessions, wallets, categories, operations CASCADE`)
	if err != nil {
		t.Fatalf("cleanDB: %v", err)
	}
}

// createUser создаёт пользователя и возвращает его ID.
func createUser(t *testing.T, email string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	err := testPool.QueryRow(context.Background(),
		`INSERT INTO users(email, password_hash) VALUES($1, 'x') RETURNING id`, email,
	).Scan(&id)
	if err != nil {
		t.Fatalf("createUser: %v", err)
	}
	return id
}
