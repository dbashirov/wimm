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

// UserSeed — стартовые кошелёк и категории, создаются вместе с пользователем.
type UserSeed struct {
	Wallet     WalletInput
	Categories []CategoryInput
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

// Granularity — шаг временной шкалы отчёта, совпадает с полем date_trunc.
type Granularity string

const (
	GranularityDay   Granularity = "day"
	GranularityWeek  Granularity = "week"
	GranularityMonth Granularity = "month"
	GranularityYear  Granularity = "year"
)

// ReportFilter — параметры отчёта по категориям.
type ReportFilter struct {
	UserID      uuid.UUID
	CategoryIDs []uuid.UUID
	WalletIDs   []uuid.UUID
	Type        OperationType // "" — оба типа
	DateFrom    time.Time
	DateTo      time.Time   // эксклюзивная граница
	Granularity Granularity // "" — авто
}

// ReportRow — сырая строка агрегации из БД.
// Имена db-тегов совпадают с алиасами колонок для pgx.RowToStructByName.
type ReportRow struct {
	Bucket     time.Time     `db:"bucket"`
	CategoryID uuid.UUID     `db:"category_id"`
	Name       string        `db:"name"`
	Type       OperationType `db:"type"`
	Amount     float64       `db:"amount"`
}

// ReportBounds — фактические границы данных под фильтр.
type ReportBounds struct {
	From  time.Time
	To    time.Time
	Count int
}

// ReportCategory — итог по одной категории за весь период.
type ReportCategory struct {
	ID    uuid.UUID `json:"id"`
	Name  string    `json:"name"`
	Total float64   `json:"total"`
	Share float64   `json:"share"` // % от итога секции; 0 при нулевом итоге
}

// ReportBucket — один период на временной шкале, выровнен по Report.Labels.
type ReportBucket struct {
	Amounts []float64 `json:"amounts"` // выровнены по ReportSection.Categories
	Total   float64   `json:"total"`
}

// ReportSection — отчёт по одному типу операций.
// json-теги фиксируют контракт с графиками в web/templates/reports.html:
// секция уезжает в браузер целиком через хелпер jsonData.
type ReportSection struct {
	Categories []ReportCategory `json:"categories"`
	Buckets    []ReportBucket   `json:"buckets"`
	Total      float64          `json:"total"`
	Average    float64          `json:"average"` // Total / len(Buckets)
}

// Report — данные страницы отчёта.
type Report struct {
	Granularity Granularity
	Labels      []string
	Expense     *ReportSection
	Income      *ReportSection
	Net         []float64 // доход − расход по бакетам
	TotalNet    float64
	Bounds      ReportBounds
	Notice      string // «шаг укрупнён до месяцев»; пустая строка — сообщать нечего
}
