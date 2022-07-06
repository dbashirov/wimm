package user

import "context"

type Repository interface {
	Create(ctx context.Context, user *User) error
	Find(ctx context.Context, id int) (*User, error)
	FindByEmail(ctx context.Context, email string) (*User, error)
}
