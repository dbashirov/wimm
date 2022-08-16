package model

type Wallet struct {
	ID          int      `json:"id"`
	Currency    Currency `json:"currency"`
	User        User     `json:"user"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
}
