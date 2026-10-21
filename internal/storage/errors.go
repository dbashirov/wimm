package storage

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
)

var ErrNotFound = errors.New("not found")

// ErrCategoryInUse - по категории есть операции, её тип менять нельзя.
var ErrCategoryInUse = errors.New("category has operations")

// ErrCategoryTypeMismatch - запись операции отклонил FK категории: тип
// категории успели сменить после проверки в сервисе.
var ErrCategoryTypeMismatch = errors.New("category type mismatch")

// categoryOwnerFK - FK из миграции 009: (category_id, user_id, type).
const categoryOwnerFK = "operations_category_owner_fkey"

func isFKViolation(err error, constraint string) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23503" &&
		pgErr.ConstraintName == constraint
}

func checkRowsAffected(tag pgconn.CommandTag, err error) error {
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
