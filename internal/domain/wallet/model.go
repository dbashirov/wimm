package wallet

import (
	"wimm/internal/domain/currency"
	"wimm/internal/domain/user"
)

type Wallet struct {
	ID          int               `json:"id"`
	Currency    currency.Currency `json:"currency"`
	User        user.User         `json:"user"`
	Title       string            `json:"title"`
	Description string            `json:"description"`
}
