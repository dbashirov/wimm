package wallet

import (
	"wimm/internal/currency"
	"wimm/internal/user"
)

type Wallet struct {
	Id          int               `json:"id"`
	Currency    currency.Currency `json:"currency"`
	User        user.User         `json:"user"`
	Title       string            `json:"title"`
	Description string            `json:"description"`
}
