package sqlstore

import (
	"context"
	"wimm/internal/user"
)

type UserRepository struct {
	store *Store
}

func (r *UserRepository) Create(ctx context.Context, u *user.User) error {

	if err := u.BeforeCreate(); err != nil {
		return err
	}

	q := `
		INSERT INTO users
			(username, email, encrypted_password)
		VALUES
			($1, $2, $3)
		RETURNING id
	`
	if err := r.store.db.QueryRow(ctx, q, u.Username, u.Email, u.EncryptedPassword).Scan(&u.ID); err != nil {
		return err
	}

	return nil
}

func (r *UserRepository) Find(ctx context.Context, id int) (*user.User, error) {
	panic("implement me")
	// u := &user
}

func (r *UserRepository) FindByEmail(ctx context.Context, email string) (*user.User, error) {
	panic("implement me")
}

func (r *UserRepository) GetAll(ctx context.Context) ([]user.User, error) {
	q := `
		SELECT id, username, email FROM users;
	`
	rows, err := r.store.db.Query(ctx, q)
	if err != nil {
		return nil, err
	}
	users := make([]user.User, 0)

	for rows.Next() {
		var u user.User

		err = rows.Scan(&u.ID, &u.Username, &u.Email)
		if err != nil {
			return nil, err
		}

		users = append(users, u)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return users, nil
}

// func NewRepository(client postgresql.Client) user.UserRepository {
// 	return &repository{
// 		client: client,
// 	}
// }
