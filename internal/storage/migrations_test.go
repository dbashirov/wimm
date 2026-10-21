package storage_test

import (
	"context"
	"database/sql"
	"fmt"
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

// TestMigration009_OwnerFK - на корректных данных (обычные операции и
// переводы с NULL в category_id/transfer_wallet_id) миграция проходит, а при
// расхождении типа операции и категории падает, ничего не применив.
func TestMigration009_OwnerFK(t *testing.T) {
	const seed = `
		WITH u AS (
			INSERT INTO users(email, password_hash) VALUES('m009@test.com', 'x')
			RETURNING id
		), w AS (
			INSERT INTO wallets(user_id, name)
			SELECT id, n FROM u, (VALUES ('A'), ('B')) AS v(n)
			RETURNING id, user_id, name
		), c AS (
			INSERT INTO categories(user_id, name, type)
			SELECT id, 'Еда', 'expense'::operation_type FROM u RETURNING id, user_id
		)
		INSERT INTO operations(user_id, wallet_id, category_id, transfer_id,
		                       transfer_wallet_id, type, amount_minor, operated_at)
		SELECT a.user_id, a.id, c.id, NULL::uuid, NULL::uuid, '%s'::operation_type, 100, now()
		FROM w a, c WHERE a.name = 'A'
		UNION ALL
		SELECT a.user_id, a.id, NULL, gen_random_uuid(), b.id, 'expense'::operation_type, 50, now()
		FROM w a, w b WHERE a.name = 'A' AND b.name = 'B'`

	tests := []struct {
		name    string
		opType  string
		wantErr bool
	}{
		{"корректные данные", "expense", false},
		{"тип операции не совпадает с категорией", "income", true},
	}
	for i, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			sqlDB := openFreshDatabase(t, fmt.Sprintf("wimm_migration_009_%d", i))
			provider, err := goose.NewProvider(goose.DialectPostgres, sqlDB, migrations.FS)
			if err != nil {
				t.Fatalf("goose.NewProvider: %v", err)
			}
			if _, err := provider.UpTo(ctx, 8); err != nil {
				t.Fatalf("goose UpTo 8: %v", err)
			}
			if _, err := sqlDB.ExecContext(ctx, fmt.Sprintf(seed, tc.opType)); err != nil {
				t.Fatalf("seed: %v", err)
			}
			_, err = provider.Up(ctx)
			if (err != nil) != tc.wantErr {
				t.Fatalf("goose Up: err = %v, wantErr %v", err, tc.wantErr)
			}
			version, err := provider.GetDBVersion(ctx)
			if err != nil {
				t.Fatalf("GetDBVersion: %v", err)
			}
			want := int64(9)
			if tc.wantErr {
				want = 8
			}
			if version != want {
				t.Errorf("версия схемы = %d, want %d", version, want)
			}
		})
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
