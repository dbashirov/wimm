package model

import (
	modelCurrency "wimm/internal/domain/currency/model"
	modelUser "wimm/internal/domain/user/model"
)

type Wallet struct {
	ID          int                    `json:"id"`
	Currency    modelCurrency.Currency `json:"currency"`
	User        modelUser.User         `json:"user"`
	Title       string                 `json:"title"`
	Description string                 `json:"description,omitempty"`
}
