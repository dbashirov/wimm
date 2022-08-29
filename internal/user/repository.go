package user

import (
	"context"
	"wimm/internal/model"
)

type Repository interface {
	Create(ctx context.Context, user model.User) error
	Find(ctx context.Context, id int) (*model.User, error)
	FindByEmail(ctx context.Context, email string) (*model.User, error)
	GetAll(ctx context.Context) ([]model.User, error)
}
