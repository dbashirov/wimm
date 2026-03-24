package model

import "time"

type User struct {
	ID           int64
	Email        string
	PasswordHash string
	CreatedAt    time.Time
}

type Session struct {
	ID        string
	UserID    int64
	ExpiresAt time.Time
}

type Wallet struct {
	ID        int64
	UserID    int64
	Name      string
	Currency  string
	CreatedAt time.Time
}

type WalletBalance struct {
	Wallet
	Income  float64
	Expense float64
	Balance float64
}

type Category struct {
	ID     int64
	UserID int64
	Name   string
	Type   string // "income" or "expense"
}

type Operation struct {
	ID           int64
	UserID       int64
	WalletID     int64
	CategoryID   int64
	Type         string // "income" or "expense"
	Amount       float64
	OperatedAt   time.Time
	Description  string
	CreatedAt    time.Time
	WalletName   string
	CategoryName string
	Currency     string
}

type OperationFilter struct {
	UserID     int64
	WalletID   int64
	CategoryID int64
	Type       string
	DateFrom   time.Time
	DateTo     time.Time
	Page       int
}

type DashboardData struct {
	WalletBalances   []WalletBalance
	CategoryExpenses []CategoryAmount
	MonthlyTotals    []MonthlyTotal
}

type CategoryAmount struct {
	Name   string
	Amount float64
}

type MonthlyTotal struct {
	Month   string
	Income  float64
	Expense float64
}
