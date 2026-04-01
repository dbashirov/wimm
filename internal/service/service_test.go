package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/dbashirov/wimm/internal/model"
	"github.com/dbashirov/wimm/internal/service"
	"github.com/dbashirov/wimm/internal/storage"
	"github.com/dbashirov/wimm/internal/storage/mocks"
	"github.com/jackc/pgx/v5/pgconn"
	"go.uber.org/mock/gomock"
	"golang.org/x/crypto/bcrypt"
)

// helpers

func newSvc(t *testing.T) (
	*service.Service,
	*mocks.MockUserRepo,
	*mocks.MockWalletRepo,
	*mocks.MockCategoryRepo,
	*mocks.MockOperationRepo,
) {
	t.Helper()
	ctrl := gomock.NewController(t)
	users := mocks.NewMockUserRepo(ctrl)
	wallets := mocks.NewMockWalletRepo(ctrl)
	cats := mocks.NewMockCategoryRepo(ctrl)
	ops := mocks.NewMockOperationRepo(ctrl)
	svc := service.NewWithRepos(users, wallets, cats, ops)
	return svc, users, wallets, cats, ops
}

var ctx = context.Background()

// ── Register ──────────────────────────────────────────────────────────────────

func TestRegister_Success(t *testing.T) {
	svc, users, _, _, _ := newSvc(t)

	want := &model.User{ID: 1, Email: "a@b.com"}
	users.EXPECT().
		Create(ctx, "a@b.com", gomock.Any()). // Any — bcrypt-хеш
		Return(want, nil)

	got, err := svc.Register(ctx, "a@b.com", "password")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Email != want.Email {
		t.Errorf("email: got %q, want %q", got.Email, want.Email)
	}
}

func TestRegister_EmailTaken(t *testing.T) {
	svc, users, _, _, _ := newSvc(t)

	pgErr := &pgconn.PgError{Code: "23505"}
	users.EXPECT().Create(ctx, gomock.Any(), gomock.Any()).Return(nil, pgErr)

	_, err := svc.Register(ctx, "dup@b.com", "password")
	if !errors.Is(err, service.ErrEmailTaken) {
		t.Errorf("got %v, want ErrEmailTaken", err)
	}
}

func TestRegister_DBError(t *testing.T) {
	svc, users, _, _, _ := newSvc(t)

	dbErr := errors.New("connection refused")
	users.EXPECT().Create(ctx, gomock.Any(), gomock.Any()).Return(nil, dbErr)

	_, err := svc.Register(ctx, "a@b.com", "password")
	if !errors.Is(err, dbErr) {
		t.Errorf("got %v, want original db error", err)
	}
}

// ── Login ─────────────────────────────────────────────────────────────────────

func TestLogin_Success(t *testing.T) {
	svc, users, _, _, _ := newSvc(t)

	hash, _ := bcrypt.GenerateFromPassword([]byte("correctpass"), bcrypt.MinCost)
	want := &model.User{ID: 1, Email: "a@b.com", PasswordHash: string(hash)}
	users.EXPECT().GetByEmail(ctx, "a@b.com").Return(want, nil)

	got, err := svc.Login(ctx, "a@b.com", "correctpass")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != 1 {
		t.Errorf("got ID %d, want 1", got.ID)
	}
}

func TestLogin_WrongPassword(t *testing.T) {
	svc, users, _, _, _ := newSvc(t)

	hash, _ := bcrypt.GenerateFromPassword([]byte("correctpass"), bcrypt.MinCost)
	users.EXPECT().GetByEmail(ctx, "a@b.com").Return(
		&model.User{ID: 1, Email: "a@b.com", PasswordHash: string(hash)}, nil,
	)

	_, err := svc.Login(ctx, "a@b.com", "wrongpass")
	if !errors.Is(err, service.ErrInvalidCredentials) {
		t.Errorf("got %v, want ErrInvalidCredentials", err)
	}
}

func TestLogin_UserNotFound(t *testing.T) {
	svc, users, _, _, _ := newSvc(t)

	users.EXPECT().GetByEmail(ctx, "noone@b.com").Return(nil, storage.ErrNotFound)

	_, err := svc.Login(ctx, "noone@b.com", "pass")
	if !errors.Is(err, service.ErrInvalidCredentials) {
		t.Errorf("got %v, want ErrInvalidCredentials", err)
	}
}

// ── CreateOperation ───────────────────────────────────────────────────────────

func TestCreateOperation_WalletNotFound(t *testing.T) {
	svc, _, wallets, _, _ := newSvc(t)

	wallets.EXPECT().GetByID(ctx, int64(99), int64(1)).Return(nil, storage.ErrNotFound)

	_, err := svc.CreateOperation(ctx, 1, 99, 10, model.TypeExpense, 100, time.Now(), "")
	if !errors.Is(err, service.ErrNotFound) {
		t.Errorf("got %v, want ErrNotFound", err)
	}
}

func TestCreateOperation_CategoryNotFound(t *testing.T) {
	svc, _, wallets, cats, _ := newSvc(t)

	wallets.EXPECT().GetByID(ctx, int64(1), int64(1)).Return(&model.Wallet{ID: 1}, nil)
	cats.EXPECT().GetByID(ctx, int64(99), int64(1)).Return(nil, storage.ErrNotFound)

	_, err := svc.CreateOperation(ctx, 1, 1, 99, model.TypeExpense, 100, time.Now(), "")
	if !errors.Is(err, service.ErrNotFound) {
		t.Errorf("got %v, want ErrNotFound", err)
	}
}

func TestCreateOperation_WrongCategoryType(t *testing.T) {
	svc, _, wallets, cats, _ := newSvc(t)

	wallets.EXPECT().GetByID(ctx, int64(1), int64(1)).Return(&model.Wallet{ID: 1}, nil)
	cats.EXPECT().GetByID(ctx, int64(1), int64(1)).Return(&model.Category{ID: 1, Type: model.TypeIncome}, nil)

	_, err := svc.CreateOperation(ctx, 1, 1, 1, model.TypeExpense, 100, time.Now(), "")
	if !errors.Is(err, service.ErrWrongCategoryType) {
		t.Errorf("got %v, want ErrWrongCategoryType", err)
	}
}

func TestCreateOperation_Success(t *testing.T) {
	svc, _, wallets, cats, ops := newSvc(t)

	now := time.Now()
	wallets.EXPECT().GetByID(ctx, int64(1), int64(1)).Return(&model.Wallet{ID: 1}, nil)
	cats.EXPECT().GetByID(ctx, int64(1), int64(1)).Return(&model.Category{ID: 1, Type: model.TypeExpense}, nil)

	want := &model.Operation{ID: 42, UserID: 1, WalletID: 1, CategoryID: 1, Type: model.TypeExpense, Amount: 100}
	ops.EXPECT().
		Create(ctx, int64(1), int64(1), int64(1), model.TypeExpense, float64(100), now, "coffee").
		Return(want, nil)

	got, err := svc.CreateOperation(ctx, 1, 1, 1, model.TypeExpense, 100, now, "coffee")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != 42 {
		t.Errorf("got ID %d, want 42", got.ID)
	}
}

// ── UpdateOperation ───────────────────────────────────────────────────────────

func TestUpdateOperation_WalletNotFound(t *testing.T) {
	svc, _, wallets, _, _ := newSvc(t)

	wallets.EXPECT().GetByID(ctx, int64(99), int64(1)).Return(nil, storage.ErrNotFound)

	_, err := svc.UpdateOperation(ctx, 5, 1, 99, 10, model.TypeExpense, 100, time.Now(), "")
	if !errors.Is(err, service.ErrNotFound) {
		t.Errorf("got %v, want ErrNotFound", err)
	}
}

func TestUpdateOperation_WrongCategoryType(t *testing.T) {
	svc, _, wallets, cats, _ := newSvc(t)

	wallets.EXPECT().GetByID(ctx, int64(1), int64(1)).Return(&model.Wallet{ID: 1}, nil)
	cats.EXPECT().GetByID(ctx, int64(1), int64(1)).Return(&model.Category{ID: 1, Type: model.TypeIncome}, nil)

	_, err := svc.UpdateOperation(ctx, 5, 1, 1, 1, model.TypeExpense, 100, time.Now(), "")
	if !errors.Is(err, service.ErrWrongCategoryType) {
		t.Errorf("got %v, want ErrWrongCategoryType", err)
	}
}

func TestUpdateOperation_OperationNotFound(t *testing.T) {
	svc, _, wallets, cats, ops := newSvc(t)

	now := time.Now()
	wallets.EXPECT().GetByID(ctx, int64(1), int64(1)).Return(&model.Wallet{ID: 1}, nil)
	cats.EXPECT().GetByID(ctx, int64(1), int64(1)).Return(&model.Category{ID: 1, Type: model.TypeExpense}, nil)
	ops.EXPECT().
		Update(ctx, int64(99), int64(1), int64(1), int64(1), model.TypeExpense, float64(100), now, "").
		Return(nil, storage.ErrNotFound)

	_, err := svc.UpdateOperation(ctx, 99, 1, 1, 1, model.TypeExpense, 100, now, "")
	if !errors.Is(err, service.ErrNotFound) {
		t.Errorf("got %v, want ErrNotFound", err)
	}
}

// ── Delete* ───────────────────────────────────────────────────────────────────

func TestDeleteWallet_NotFound(t *testing.T) {
	svc, _, wallets, _, _ := newSvc(t)

	wallets.EXPECT().Delete(ctx, int64(1), int64(1)).Return(storage.ErrNotFound)

	err := svc.DeleteWallet(ctx, 1, 1)
	if !errors.Is(err, service.ErrNotFound) {
		t.Errorf("got %v, want ErrNotFound", err)
	}
}

func TestDeleteWallet_Success(t *testing.T) {
	svc, _, wallets, _, _ := newSvc(t)

	wallets.EXPECT().Delete(ctx, int64(1), int64(1)).Return(nil)

	if err := svc.DeleteWallet(ctx, 1, 1); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestDeleteCategory_NotFound(t *testing.T) {
	svc, _, _, cats, _ := newSvc(t)

	cats.EXPECT().Delete(ctx, int64(5), int64(1)).Return(storage.ErrNotFound)

	err := svc.DeleteCategory(ctx, 5, 1)
	if !errors.Is(err, service.ErrNotFound) {
		t.Errorf("got %v, want ErrNotFound", err)
	}
}

func TestDeleteOperation_NotFound(t *testing.T) {
	svc, _, _, _, ops := newSvc(t)

	ops.EXPECT().Delete(ctx, int64(7), int64(1)).Return(storage.ErrNotFound)

	err := svc.DeleteOperation(ctx, 7, 1)
	if !errors.Is(err, service.ErrNotFound) {
		t.Errorf("got %v, want ErrNotFound", err)
	}
}

// ── GetWallet / GetCategory / GetOperation ────────────────────────────────────

func TestGetWallet_NotFound(t *testing.T) {
	svc, _, wallets, _, _ := newSvc(t)

	wallets.EXPECT().GetByID(ctx, int64(1), int64(1)).Return(nil, storage.ErrNotFound)

	_, err := svc.GetWallet(ctx, 1, 1)
	if !errors.Is(err, service.ErrNotFound) {
		t.Errorf("got %v, want ErrNotFound", err)
	}
}

func TestGetCategory_NotFound(t *testing.T) {
	svc, _, _, cats, _ := newSvc(t)

	cats.EXPECT().GetByID(ctx, int64(1), int64(1)).Return(nil, storage.ErrNotFound)

	_, err := svc.GetCategory(ctx, 1, 1)
	if !errors.Is(err, service.ErrNotFound) {
		t.Errorf("got %v, want ErrNotFound", err)
	}
}

func TestGetOperation_NotFound(t *testing.T) {
	svc, _, _, _, ops := newSvc(t)

	ops.EXPECT().GetByID(ctx, int64(1), int64(1)).Return(nil, storage.ErrNotFound)

	_, err := svc.GetOperation(ctx, 1, 1)
	if !errors.Is(err, service.ErrNotFound) {
		t.Errorf("got %v, want ErrNotFound", err)
	}
}
