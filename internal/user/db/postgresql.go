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

func NewRepository(client postgresql.Client) user.Repository {
	return &repository{
		client: client,
	}
}
