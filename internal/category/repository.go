package category

import (
	"context"
	"wimm/internal/model"
)

type Repository interface {
	Create(ctx context.Context, user *model.Category) error
	Find(ctx context.Context, id int) (*model.Category, error)
	GetAll(ctx context.Context) ([]model.Category, error)
}