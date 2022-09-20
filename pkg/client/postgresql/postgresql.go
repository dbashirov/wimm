package postgresql

import (
	"context"
	"fmt"
	"log"
	"time"
	"wimm/config"
	repeatable "wimm/pkg/utils/repeatable"

	"github.com/jackc/pgconn"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
)

type Client interface {
	Exec(ctx context.Context, sql string, arguments ...interface{}) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row
	Begin(ctx context.Context) (pgx.Tx, error)
	BeginTx(ctx context.Context, txOptions pgx.TxOptions) (pgx.Tx, error)
}

func NewClient(ctx context.Context, sc config.StorageConfig, maxAttempts int) (pool *pgxpool.Pool, err error) {
	dsn := fmt.Sprintf("host=%s port=%s dbname=%s user=%s sslmode=disable", sc.Host, sc.Port, sc.Database, sc.Username)

	err = repeatable.DoWithTries(func() error {
		ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()

		pool, err = pgxpool.Connect(ctx, dsn)
		if err != nil {
			return err
		}
		return nil
	}, 3, 5*time.Second)

	if err != nil {
		log.Fatalln("error do with tries postresql")
	}

	return pool, nil
}
