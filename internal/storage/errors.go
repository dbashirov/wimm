package storage

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
)

var ErrNotFound = errors.New("not found")

// ErrCategoryInUse - по категории есть операции, её тип менять нельзя.
var ErrCategoryInUse = errors.New("category has operations")

func checkRowsAffected(tag pgconn.CommandTag, err error) error {
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
