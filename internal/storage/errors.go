package storage

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
)

var ErrNotFound = errors.New("not found")

func checkRowsAffected(tag pgconn.CommandTag, err error) error {
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
