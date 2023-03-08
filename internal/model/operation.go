package model

import (
	"time"
)

type Operation struct {
	ID              int             `json:"id"`
	TypeOperation   TypeOfOperation `json:"typeOfOpeartion"`
	Date            time.Time       `json:"date"`
	User            User            `json:"user"`
	Sum             float64         `json:"sum"`
	Wallet          Wallet          `json:"wallet"`
	Description     string          `json:"description,omitempty"`
	Category        Category        `json:"category,omitempty"`
	WalletRecipient Wallet          `json:"walletRecipient,omitempty"`
}
