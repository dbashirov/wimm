package category

import (
	"wimm/internal/app/types"
	"wimm/internal/app/user"
)

type Category struct {
	ID         int                `json:"id"`
	Title      string             `json:"title"`
	User       user.User          `json:"user"`
	TypeWallet types.TypeOfWallet `json:"typeWallet"`
}
