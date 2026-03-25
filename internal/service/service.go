package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/dbashirov/wimm/internal/model"
	"github.com/dbashirov/wimm/internal/storage"
	"github.com/jackc/pgx/v5/pgconn"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrEmailTaken        = errors.New("email already registered")
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrNotFound          = errors.New("not found")
	ErrWrongCategoryType = errors.New("category type must match operation type")
)

type Service struct {
	store *storage.Storage
}

func New(store *storage.Storage) *Service {
	return &Service{store: store}
}

// --- Users ---

func (s *Service) Register(ctx context.Context, email, password string) (*model.User, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		return nil, fmt.Errorf("bcrypt: %w", err)
	}
	user, err := s.store.Users.Create(ctx, email, string(hash))
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, ErrEmailTaken
		}
		return nil, err
	}
	return user, nil
}

func (s *Service) Login(ctx context.Context, email, password string) (*model.User, error) {
	user, err := s.store.Users.GetByEmail(ctx, email)
	if err != nil {
		return nil, ErrInvalidCredentials
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, ErrInvalidCredentials
	}
	return user, nil
}

// --- Wallets ---

func (s *Service) ListWallets(ctx context.Context, userID int64) ([]model.Wallet, error) {
	return s.store.Wallets.ListByUser(ctx, userID)
}

func (s *Service) ListWalletsWithBalance(ctx context.Context, userID int64) ([]model.WalletBalance, error) {
	return s.store.Wallets.ListWithBalance(ctx, userID)
}

func (s *Service) CreateWallet(ctx context.Context, userID int64, name, currency string) (*model.Wallet, error) {
	return s.store.Wallets.Create(ctx, userID, name, currency)
}

func (s *Service) GetWallet(ctx context.Context, id, userID int64) (*model.Wallet, error) {
	w, err := s.store.Wallets.GetByID(ctx, id, userID)
	if errors.Is(err, storage.ErrNotFound) {
		return nil, ErrNotFound
	}
	return w, err
}

func (s *Service) UpdateWallet(ctx context.Context, id, userID int64, name, currency string) (*model.Wallet, error) {
	w, err := s.store.Wallets.Update(ctx, id, userID, name, currency)
	if errors.Is(err, storage.ErrNotFound) {
		return nil, ErrNotFound
	}
	return w, err
}

func (s *Service) DeleteWallet(ctx context.Context, id, userID int64) error {
	return s.store.Wallets.Delete(ctx, id, userID)
}

// --- Categories ---

func (s *Service) ListCategories(ctx context.Context, userID int64) ([]model.Category, error) {
	return s.store.Categories.ListByUser(ctx, userID)
}

func (s *Service) ListCategoriesByType(ctx context.Context, userID int64, catType string) ([]model.Category, error) {
	return s.store.Categories.ListByType(ctx, userID, catType)
}

func (s *Service) CreateCategory(ctx context.Context, userID int64, name, catType string) (*model.Category, error) {
	return s.store.Categories.Create(ctx, userID, name, catType)
}

func (s *Service) GetCategory(ctx context.Context, id, userID int64) (*model.Category, error) {
	c, err := s.store.Categories.GetByID(ctx, id, userID)
	if errors.Is(err, storage.ErrNotFound) {
		return nil, ErrNotFound
	}
	return c, err
}

func (s *Service) UpdateCategory(ctx context.Context, id, userID int64, name string) (*model.Category, error) {
	c, err := s.store.Categories.Update(ctx, id, userID, name)
	if errors.Is(err, storage.ErrNotFound) {
		return nil, ErrNotFound
	}
	return c, err
}

func (s *Service) DeleteCategory(ctx context.Context, id, userID int64) error {
	return s.store.Categories.Delete(ctx, id, userID)
}

// --- Operations ---

func (s *Service) ListOperations(ctx context.Context, filter model.OperationFilter) ([]model.Operation, error) {
	return s.store.Operations.List(ctx, filter)
}

func (s *Service) CreateOperation(ctx context.Context, userID, walletID, categoryID int64, opType string, amount float64, operatedAt time.Time, description string) (*model.Operation, error) {
	cat, err := s.store.Categories.GetByID(ctx, categoryID, userID)
	if errors.Is(err, storage.ErrNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if cat.Type != opType {
		return nil, ErrWrongCategoryType
	}
	return s.store.Operations.Create(ctx, userID, walletID, categoryID, opType, amount, operatedAt, description)
}

func (s *Service) GetOperation(ctx context.Context, id, userID int64) (*model.Operation, error) {
	o, err := s.store.Operations.GetByID(ctx, id, userID)
	if errors.Is(err, storage.ErrNotFound) {
		return nil, ErrNotFound
	}
	return o, err
}

func (s *Service) UpdateOperation(ctx context.Context, id, userID, walletID, categoryID int64, opType string, amount float64, operatedAt time.Time, description string) (*model.Operation, error) {
	cat, err := s.store.Categories.GetByID(ctx, categoryID, userID)
	if errors.Is(err, storage.ErrNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if cat.Type != opType {
		return nil, ErrWrongCategoryType
	}
	o, err := s.store.Operations.Update(ctx, id, userID, walletID, categoryID, opType, amount, operatedAt, description)
	if errors.Is(err, storage.ErrNotFound) {
		return nil, ErrNotFound
	}
	return o, err
}

func (s *Service) DeleteOperation(ctx context.Context, id, userID int64) error {
	return s.store.Operations.Delete(ctx, id, userID)
}

// --- Dashboard ---

func (s *Service) DashboardData(ctx context.Context, userID int64) (*model.DashboardData, error) {
	wallets, err := s.store.Wallets.ListWithBalance(ctx, userID)
	if err != nil {
		return nil, err
	}
	cats, monthly, err := s.store.Operations.DashboardData(ctx, userID)
	if err != nil {
		return nil, err
	}
	return &model.DashboardData{
		WalletBalances:   wallets,
		CategoryExpenses: cats,
		MonthlyTotals:    monthly,
	}, nil
}
