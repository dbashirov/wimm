package model

import "time"

type (
	TypeOfWallet    int
	TypeOfOperation int
)

const (
	TypeIncome TypeOfWallet = iota + 1
	TypeExpense
	TypeTransfer
)

func (t TypeOfWallet) String() string {
	return [...]string{"Доход", "Расход"}[t-1]
}

func (t TypeOfWallet) EnumIndex() int {
	return int(t)
}

func (t TypeOfOperation) String() string {
	return [...]string{"Доход", "Расход", "Перевод"}[t-1]
}

func (t TypeOfOperation) EnumIndex() int {
	return int(t)
}

type User struct {
	id       int
	login    string
	email    string
	password string
}

type Currency struct {
	id            int
	code          string
	charactercode string
	title         string
}

type Wallet struct {
	id          int
	currency    Currency
	user        User
	title       string
	description string
}

type Category struct {
	id         int
	title      string
	user       User
	typeWallet TypeOfWallet
}

type Operation struct {
	id               int
	typeOperation    TypeOfOperation
	date             time.Time
	user             User
	sum              float64
	wallet           Wallet
	description      string
	category         Category
	wallet_recipeint Wallet
}
