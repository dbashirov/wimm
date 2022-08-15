package category

import (
	"wimm/internal/domain/types"
	"wimm/internal/domain/user"
)

type Category struct {
	ID         int                `json:"id"`
	Title      string             `json:"title"`
	User       user.User          `json:"user"`
	TypeWallet types.TypeOfWallet `json:"typeWallet"`
}
