package wallet

import (
	"wimm/internal/app/currency"
	"wimm/internal/app/user"
)

type Wallet struct {
	ID          int               `json:"id"`
	Currency    currency.Currency `json:"currency"`
	User        user.User         `json:"user"`
	Title       string            `json:"title"`
	Description string            `json:"description"`
}
