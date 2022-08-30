package model

import (
	types "wimm/internal/domain/types/model"
	user "wimm/internal/domain/user/model"
)

type Category struct {
	ID    int                `json:"id"`
	Title string             `json:"title"`
	User  user.User          `json:"user"`
	Type  types.TypeOfWallet `json:"type"`
}
