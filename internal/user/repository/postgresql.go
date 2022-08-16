package user

import (
	"context"
	"wimm/internal/model"
	"wimm/internal/user"
	"wimm/pkg/client/postgresql"
)

type repository struct {
	db postgresql.Client
}

func (r *repository) Create(ctx context.Context, u *model.User) error {

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
	if err := r.db.QueryRow(ctx, q, u.Username, u.Email, u.EncryptedPassword).Scan(&u.ID); err != nil {
		return err
	}

	return nil
}

func (r *repository) Find(ctx context.Context, id int) (*model.User, error) {
	panic("implement me")
	// u := &user
}

func (r *repository) FindByEmail(ctx context.Context, email string) (*model.User, error) {
	panic("implement me")
}

func (r *repository) GetAll(ctx context.Context) ([]model.User, error) {
	q := `
		SELECT id, username, email FROM users;
	`
	rows, err := r.db.Query(ctx, q)
	if err != nil {
		return nil, err
	}
	users := make([]model.User, 0)

	for rows.Next() {
		var u model.User

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

func NewRepository(db postgresql.Client) user.Repository {
	return &repository{
		db: db,
	}
}
