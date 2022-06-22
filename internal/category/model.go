package category

import (
	"wimm/internal/model"
	"wimm/internal/user"
)

type Category struct {
	ID         int                `json:"id"`
	Title      string             `json:"title"`
	User       user.User          `json:"user"`
	TypeWallet model.TypeOfWallet `json:"typeWallet"`
}
