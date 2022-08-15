package operations

import (
	"time"
	"wimm/internal/domain/category"
	"wimm/internal/domain/types"
	"wimm/internal/domain/user"
	"wimm/internal/domain/wallet"
)

type Operation struct {
	ID              int                   `json:"id"`
	TypeOperation   types.TypeOfOperation `json:"typeOfOpeartion"`
	Date            time.Time             `json:"date"`
	User            user.User             `json:"user"`
	Sum             float64               `json:"sum"`
	Wallet          wallet.Wallet         `json:"wallet"`
	Description     string                `json:"description"`
	Category        category.Category     `json:"category"`
	WalletRecipient wallet.Wallet         `json:"walletRecipient"`
}
