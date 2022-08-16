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
	Description     string          `json:"description"`
	Category        Category        `json:"category"`
	WalletRecipient Wallet          `json:"walletRecipient"`
}
