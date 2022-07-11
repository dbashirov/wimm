package user

import (
	"context"
	"wimm/internal/user"
	"wimm/pkg/client/postgresql"
)

type repository struct {
	client postgresql.Client
}

func (r *repository) Create(ctx context.Context, u *user.User) error {

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
	if err := r.client.QueryRow(ctx, q, u.Username, u.Email, u.EncryptedPassword).Scan(&u.ID); err != nil {
		return err
	}

	return nil
}

func (r *repository) Find(ctx context.Context, id int) (*user.User, error) {
	panic("implement me")
	// u := &user
}

func (r *repository) FindByEmail(ctx context.Context, email string) (*user.User, error) {
	panic("implement me")
}

func (r *repository) GetAll(ctx context.Context) ([]user.User, error) {
	q := `
		SELECT id, username, email FROM users;
	`
	rows, err := r.client.Query(ctx, q)
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

func NewRepository(client postgresql.Client) user.Repository {
	return &repository{
		client: client,
	}
}
