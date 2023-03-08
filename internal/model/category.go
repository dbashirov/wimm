package model

import "context"

type Category struct {
	ID    int          `json:"id"`
	Title string       `json:"title"`
	User  User         `json:"user"`
	Type  TypeOfWallet `json:"type"`
}

type CategoryRepository interface {
	Create(ctx context.Context, user *Category) error
	Find(ctx context.Context, id int) (*Category, error)
	GetAll(ctx context.Context) ([]Category, error)
}
