package model

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

type OperationType string

const (
	TypeIncome  OperationType = "income"
	TypeExpense OperationType = "expense"
)

// Scan реализует sql.Scanner для совместимости с pgx v5.
func (t *OperationType) Scan(src any) error {
	s, ok := src.(string)
	if !ok {
		return fmt.Errorf("OperationType: expected string, got %T", src)
	}
	*t = OperationType(s)
	return nil
}

type User struct {
	ID           uuid.UUID
	Email        string
	PasswordHash string
	CreatedAt    time.Time
}

type Session struct {
	ID        string
	UserID    uuid.UUID
	ExpiresAt time.Time
}

type Wallet struct {
	ID          uuid.UUID
	UserID      uuid.UUID
	Name        string
	Currency    string
	Description string
	CreatedAt   time.Time
}

type WalletBalance struct {
	Wallet
	Income  float64
	Expense float64
	Balance float64
}

type Category struct {
	ID          uuid.UUID
	UserID      uuid.UUID
	Name        string
	Type        OperationType
	Description string
}

type Operation struct {
	ID                 uuid.UUID
	UserID             uuid.UUID
	WalletID           uuid.UUID
	CategoryID         uuid.UUID // uuid.Nil for transfers
	Type               OperationType
	Amount             float64
	OperatedAt         time.Time
	Description        string
	CreatedAt          time.Time
	TransferID         *uuid.UUID
	TransferWalletName string
	WalletName         string
	CategoryName       string
	Currency           string
}

type OperationFilter struct {
	UserID     uuid.UUID
	WalletID   uuid.UUID
	CategoryID uuid.UUID
	Type       OperationType
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
