package model

type Category struct {
	ID    int          `json:"id"`
	Title string       `json:"title"`
	User  User         `json:"user"`
	Type  TypeOfWallet `json:"type"`
}
