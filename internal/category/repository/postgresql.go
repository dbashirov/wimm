package category

import (
	"context"
	"wimm/internal/category"
	"wimm/internal/model"
	"wimm/pkg/client/postgresql"
)

type repository struct {
	db postgresql.Client
}

func (r *repository) Create(ctx context.Context, c *model.Category) error {

	q := `
		INSERT INTO categories
			(title, id_user, type_wallet)
		VALUES
			($1, $2, $3)
		RETURNING id
	`
	if err := r.db.QueryRow(ctx, q, c.Title, c.User.ID, c.TypeWallet).Scan(&c.ID); err != nil {
		return err
	}

	return nil

}

func (r *repository) Find(ctx context.Context, id int) (*model.Category, error) {
	panic("implement me")
}

func (r *repository) GetAll(ctx context.Context) ([]model.Category, error) {
	q := `
		SELECT id, title, user, typeWallet FROM users;
	`
	rows, err := r.db.Query(ctx, q)
	if err != nil {
		return nil, err
	}
	categories := make([]model.Category, 0)

	for rows.Next() {
		var c model.Category

		err = rows.Scan(&c.ID, &c.Title, &c.User, &c.TypeWallet)
		if err != nil {
			return nil, err
		}

		categories = append(categories, c)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}

	return categories, nil
}

func NewRepository(db postgresql.Client) category.Repository {
	return &repository{
		db: db,
	}
}
