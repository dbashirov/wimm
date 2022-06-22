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
	q := `
		INSERT INTO users
			(username, email)
		VALUES
			($1, $2)
		RETURNING id
	`
	if err := r.client.QueryRow(ctx, q, u.Username, u.Email).Scan(&u.ID); err != nil {
		return err
	}
	return nil
}

func (r *repository) Find(ctx context.Context, id int) (user.User, error) {
	panic("implement me")
}

func (r *repository) FindByEmail(ctx context.Context, email string) (user.User, error) {
	panic("implement me")
}

func NewRepository(client postgresql.Client) user.Repository {
	return &repository{
		client: client,
	}
}
