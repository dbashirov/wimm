package storage

//go:generate go run go.uber.org/mock/mockgen@v0.6.0 -destination=mocks/mock_repos.go -package=mocks -source=iface.go

import (
	"context"
	"time"

	"github.com/dbashirov/wimm/internal/model"
)

type UserRepo interface {
	Create(ctx context.Context, email, passwordHash string) (*model.User, error)
	GetByEmail(ctx context.Context, email string) (*model.User, error)
}

type WalletRepo interface {
	ListByUser(ctx context.Context, userID int64) ([]model.Wallet, error)
	ListWithBalance(ctx context.Context, userID int64) ([]model.WalletBalance, error)
	Create(ctx context.Context, userID int64, name, currency string) (*model.Wallet, error)
	GetByID(ctx context.Context, id, userID int64) (*model.Wallet, error)
	Update(ctx context.Context, id, userID int64, name, currency string) (*model.Wallet, error)
	Delete(ctx context.Context, id, userID int64) error
}

type CategoryRepo interface {
	ListByUser(ctx context.Context, userID int64) ([]model.Category, error)
	ListByType(ctx context.Context, userID int64, catType string) ([]model.Category, error)
	Create(ctx context.Context, userID int64, name, catType string) (*model.Category, error)
	GetByID(ctx context.Context, id, userID int64) (*model.Category, error)
	Update(ctx context.Context, id, userID int64, name string) (*model.Category, error)
	Delete(ctx context.Context, id, userID int64) error
}

type OperationRepo interface {
	List(ctx context.Context, filter model.OperationFilter) ([]model.Operation, error)
	Create(ctx context.Context, userID, walletID, categoryID int64, opType string, amount float64, operatedAt time.Time, description string) (*model.Operation, error)
	GetByID(ctx context.Context, id, userID int64) (*model.Operation, error)
	Update(ctx context.Context, id, userID, walletID, categoryID int64, opType string, amount float64, operatedAt time.Time, description string) (*model.Operation, error)
	Delete(ctx context.Context, id, userID int64) error
	DashboardData(ctx context.Context, userID int64) ([]model.CategoryAmount, []model.MonthlyTotal, error)
}
