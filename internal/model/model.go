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
	Hidden      bool
	IsDefault   bool
	SortOrder   int
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

// WalletInput — данные для создания/обновления кошелька.
type WalletInput struct {
	Name        string
	Currency    string
	Description string
}

// CategoryInput — данные для создания категории.
type CategoryInput struct {
	Name        string
	Type        OperationType
	Description string
}

// OperationInput — данные для создания/обновления операции.
type OperationInput struct {
	WalletID    uuid.UUID
	CategoryID  uuid.UUID
	Type        OperationType
	Amount      float64
	OperatedAt  time.Time
	Description string
}

// TransferInput — данные для создания перевода.
type TransferInput struct {
	FromWalletID uuid.UUID
	ToWalletID   uuid.UUID
	Amount       float64
	OperatedAt   time.Time
	Description  string
}

type DashboardData struct {
	WalletBalances  []WalletBalance
	TotalBalance    float64
	MonthlyExpenses []MonthlyCategoryData
	MonthlyIncomes  []MonthlyCategoryData
	MonthlyTotals   []MonthlyTotal
}

type CategoryAmount struct {
	Name   string
	Amount float64
}

// MonthlyCategoryData — агрегат по категориям за один месяц.
type MonthlyCategoryData struct {
	Label string          // "Февраль", "Март", "Текущий месяц"
	Total float64
	Items []CategoryAmount
}

type MonthlyTotal struct {
	Month   string
	Income  float64
	Expense float64
	Balance float64 // нарастающий итог на конец месяца
}
