package store

import (
	"context"
	"wimm/internal/app/user"
)

type UserRepository interface {
	Create(ctx context.Context, user *user.User) error
	Find(ctx context.Context, id int) (*user.User, error)
	FindByEmail(ctx context.Context, email string) (*user.User, error)
	GetAll(ctx context.Context) ([]user.User, error)
}
