package storage_test

import (
	"context"
	"database/sql"
	"net/url"
	"testing"

	"github.com/dbashirov/wimm/migrations"
	"github.com/jackc/pgx/v5"
	"github.com/pressly/goose/v3"
)

// TestMigration008_AmountToMinor — суммы из NUMERIC(15,2) переезжают в копейки
// без потерь. У 008 нет секции Down, поэтому миграции накатываются в отдельной
// БД: общую схему пакета откатить до старой колонки было бы нечем.
func TestMigration008_AmountToMinor(t *testing.T) {
	ctx := context.Background()
	sqlDB := openFreshDatabase(t, "wimm_migration_008")

	provider, err := goose.NewProvider(goose.DialectPostgres, sqlDB, migrations.FS)
	if err != nil {
		t.Fatalf("goose.NewProvider: %v", err)
	}
	if _, err := provider.UpTo(ctx, 1); err != nil {
		t.Fatalf("goose UpTo 1: %v", err)
	}

	_, err = sqlDB.ExecContext(ctx, `
		WITH u AS (
			INSERT INTO users(email, password_hash)
			VALUES('migration@test.com', 'x') RETURNING id
		), w AS (
			INSERT INTO wallets(user_id, name) SELECT id, 'Main' FROM u
			RETURNING id, user_id
		)
		INSERT INTO operations(user_id, wallet_id, type, amount, operated_at)
		SELECT w.user_id, w.id, v.type, v.amount, now()
		FROM w, (VALUES
			('expense'::operation_type, 12.34),
			('income', 9999999999999.99),
			('income', 0.01)
		) AS v(type, amount)`)
	if err != nil {
		t.Fatalf("insert operations: %v", err)
	}

	if _, err := provider.Up(ctx); err != nil {
		t.Fatalf("goose Up: %v", err)
	}

	rows, err := sqlDB.QueryContext(ctx,
		`SELECT amount_minor FROM operations ORDER BY amount_minor`)
	if err != nil {
		t.Fatalf("select amount_minor: %v", err)
	}
	defer rows.Close()

	var got []int64
	for rows.Next() {
		var v int64
		if err := rows.Scan(&v); err != nil {
			t.Fatalf("scan: %v", err)
		}
		got = append(got, v)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows: %v", err)
	}

	want := []int64{1, 1234, 999999999999999}
	if len(got) != len(want) {
		t.Fatalf("amount_minor = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("amount_minor[%d] = %d, want %d", i, got[i], want[i])
		}
	}
}

// openFreshDatabase создаёт пустую БД в тестовом контейнере и открывает к ней
// соединение; БД удаляется после теста.
func openFreshDatabase(t *testing.T, name string) *sql.DB {
	t.Helper()
	ctx := context.Background()
	quoted := pgx.Identifier{name}.Sanitize()

	drop := func() {
		if _, err := testPool.Exec(ctx, `DROP DATABASE IF EXISTS `+quoted+` WITH (FORCE)`); err != nil {
			t.Errorf("DROP DATABASE %s: %v", name, err)
		}
	}
	drop()
	if _, err := testPool.Exec(ctx, `CREATE DATABASE `+quoted); err != nil {
		t.Fatalf("CREATE DATABASE %s: %v", name, err)
	}
	t.Cleanup(drop)

	dsn, err := url.Parse(testDSN)
	if err != nil {
		t.Fatalf("url.Parse(testDSN): %v", err)
	}
	dsn.Path = "/" + name

	sqlDB, err := sql.Open("pgx", dsn.String())
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	t.Cleanup(func() { sqlDB.Close() })
	return sqlDB
}
