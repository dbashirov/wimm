package sqlstore

import "github.com/jackc/pgx/v4/pgxpool"

type Store struct {
	db             *pgxpool.Pool
	userRepository *UserRepository
}
