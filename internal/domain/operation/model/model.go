package model

import (
	"time"
	modelCategory "wimm/internal/domain/category/model"
	modelTypes "wimm/internal/domain/types/model"
	modelUser "wimm/internal/domain/user/model"
	modelWallet "wimm/internal/domain/wallet/model"
)

type Operation struct {
	ID              int                        `json:"id"`
	TypeOperation   modelTypes.TypeOfOperation `json:"typeOfOpeartion"`
	Date            time.Time                  `json:"date"`
	User            modelUser.User             `json:"user"`
	Sum             float64                    `json:"sum"`
	Wallet          modelWallet.Wallet         `json:"wallet"`
	Description     string                     `json:"description,omitempty"`
	Category        modelCategory.Category     `json:"category,omitempty"`
	WalletRecipient modelWallet.Wallet         `json:"walletRecipient,omitempty"`
}
