package sqlstore

import (
	"wimm/internal/store"

	"github.com/jackc/pgx/v4/pgxpool"
)

type Store struct {
	db             *pgxpool.Pool
	userRepository *UserRepository
}

func New(db *pgxpool.Pool) *Store {
	return &Store{
		db: db,
	}
}

func (s *Store) User() store.UserRepository {
	if s.userRepository != nil {
		return s.userRepository
	}

	s.userRepository = &UserRepository{
		store: s,
	}

	return s.userRepository
}
