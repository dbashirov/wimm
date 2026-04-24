package storage

//go:generate go run go.uber.org/mock/mockgen@v0.6.0 -destination=mocks/mock_repos.go -package=mocks -source=iface.go

import (
	"context"

	"github.com/dbashirov/wimm/internal/model"
	"github.com/google/uuid"
)

type UserRepo interface {
	Create(ctx context.Context, email, passwordHash string) (*model.User, error)
	GetByEmail(ctx context.Context, email string) (*model.User, error)
}

type WalletRepo interface {
	ListByUser(ctx context.Context, userID uuid.UUID) ([]model.Wallet, error)
	ListWithBalance(ctx context.Context, userID uuid.UUID) ([]model.WalletBalance, error)
	Create(ctx context.Context, userID uuid.UUID, inp model.WalletInput) (*model.Wallet, error)
	GetByID(ctx context.Context, id uuid.UUID, userID uuid.UUID) (*model.Wallet, error)
	Update(ctx context.Context, id uuid.UUID, userID uuid.UUID, inp model.WalletInput) (*model.Wallet, error)
	Delete(ctx context.Context, id uuid.UUID, userID uuid.UUID) error
}

type CategoryRepo interface {
	ListByUser(ctx context.Context, userID uuid.UUID) ([]model.Category, error)
	ListByType(ctx context.Context, userID uuid.UUID, catType model.OperationType) ([]model.Category, error)
	Create(ctx context.Context, userID uuid.UUID, inp model.CategoryInput) (*model.Category, error)
	GetByID(ctx context.Context, id uuid.UUID, userID uuid.UUID) (*model.Category, error)
	Update(ctx context.Context, id uuid.UUID, userID uuid.UUID, name, description string) (*model.Category, error)
	Delete(ctx context.Context, id uuid.UUID, userID uuid.UUID) error
}

type OperationRepo interface {
	List(ctx context.Context, filter model.OperationFilter) ([]model.Operation, error)
	Create(ctx context.Context, userID uuid.UUID, inp model.OperationInput) (*model.Operation, error)
	GetByID(ctx context.Context, id uuid.UUID, userID uuid.UUID) (*model.Operation, error)
	Update(ctx context.Context, id uuid.UUID, userID uuid.UUID, inp model.OperationInput) (*model.Operation, error)
	Delete(ctx context.Context, id uuid.UUID, userID uuid.UUID) error
	CreateTransfer(ctx context.Context, userID uuid.UUID, inp model.TransferInput) (*model.Operation, *model.Operation, error)
	DeleteTransfer(ctx context.Context, transferID uuid.UUID, userID uuid.UUID) error
	DashboardData(ctx context.Context, userID uuid.UUID) ([]model.CategoryAmount, []model.MonthlyTotal, error)
}
