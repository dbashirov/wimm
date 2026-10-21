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
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"go.uber.org/mock/gomock"
	"golang.org/x/crypto/bcrypt"
)

// test UUIDs
var (
	wID1   = uuid.MustParse("10000000-0000-0000-0000-000000000001")
	wID2   = uuid.MustParse("10000000-0000-0000-0000-000000000002")
	wID99  = uuid.MustParse("10000000-0000-0000-0000-000000000099")
	cID1   = uuid.MustParse("20000000-0000-0000-0000-000000000001")
	cID5   = uuid.MustParse("20000000-0000-0000-0000-000000000005")
	cID10  = uuid.MustParse("20000000-0000-0000-0000-000000000010")
	cID99  = uuid.MustParse("20000000-0000-0000-0000-000000000099")
	opID1  = uuid.MustParse("30000000-0000-0000-0000-000000000001")
	opID5  = uuid.MustParse("30000000-0000-0000-0000-000000000005")
	opID7  = uuid.MustParse("30000000-0000-0000-0000-000000000007")
	opID99 = uuid.MustParse("30000000-0000-0000-0000-000000000099")
	uID1   = uuid.MustParse("00000000-0000-0000-0000-000000000001")
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

	want := &model.User{ID: uID1, Email: "a@b.com"}
	users.EXPECT().
		Create(ctx, "a@b.com", gomock.Any(), gomock.Any()). // Any — bcrypt-хеш
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
	users.EXPECT().Create(ctx, gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, pgErr)

	_, err := svc.Register(ctx, "dup@b.com", "password")
	if !errors.Is(err, service.ErrEmailTaken) {
		t.Errorf("got %v, want ErrEmailTaken", err)
	}
}

func TestRegister_DBError(t *testing.T) {
	svc, users, _, _, _ := newSvc(t)

	dbErr := errors.New("connection refused")
	users.EXPECT().Create(ctx, gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, dbErr)

	_, err := svc.Register(ctx, "a@b.com", "password")
	if !errors.Is(err, dbErr) {
		t.Errorf("got %v, want original db error", err)
	}
}

// ── Login ─────────────────────────────────────────────────────────────────────

func TestLogin_Success(t *testing.T) {
	svc, users, _, _, _ := newSvc(t)

	hash, _ := bcrypt.GenerateFromPassword([]byte("correctpass"), bcrypt.MinCost)
	want := &model.User{ID: uID1, Email: "a@b.com", PasswordHash: string(hash)}
	users.EXPECT().GetByEmail(ctx, "a@b.com").Return(want, nil)

	got, err := svc.Login(ctx, "a@b.com", "correctpass")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != uID1 {
		t.Errorf("got ID %v, want %v", got.ID, uID1)
	}
}

func TestLogin_WrongPassword(t *testing.T) {
	svc, users, _, _, _ := newSvc(t)

	hash, _ := bcrypt.GenerateFromPassword([]byte("correctpass"), bcrypt.MinCost)
	users.EXPECT().GetByEmail(ctx, "a@b.com").Return(
		&model.User{ID: uID1, Email: "a@b.com", PasswordHash: string(hash)}, nil,
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

	wallets.EXPECT().GetByID(ctx, wID99, uID1).Return(nil, storage.ErrNotFound)

	inp := model.OperationInput{WalletID: wID99, CategoryID: cID10, Type: model.TypeExpense, Amount: 100}
	_, err := svc.CreateOperation(ctx, uID1, inp)
	if !errors.Is(err, service.ErrNotFound) {
		t.Errorf("got %v, want ErrNotFound", err)
	}
}

func TestCreateOperation_CategoryNotFound(t *testing.T) {
	svc, _, wallets, cats, _ := newSvc(t)

	wallets.EXPECT().GetByID(ctx, wID1, uID1).Return(&model.Wallet{ID: wID1}, nil)
	cats.EXPECT().GetByID(ctx, cID99, uID1).Return(nil, storage.ErrNotFound)

	inp := model.OperationInput{WalletID: wID1, CategoryID: cID99, Type: model.TypeExpense, Amount: 100}
	_, err := svc.CreateOperation(ctx, uID1, inp)
	if !errors.Is(err, service.ErrNotFound) {
		t.Errorf("got %v, want ErrNotFound", err)
	}
}

func TestCreateOperation_WrongCategoryType(t *testing.T) {
	svc, _, wallets, cats, _ := newSvc(t)

	wallets.EXPECT().GetByID(ctx, wID1, uID1).Return(&model.Wallet{ID: wID1}, nil)
	cats.EXPECT().GetByID(ctx, cID1, uID1).Return(&model.Category{ID: cID1, Type: model.TypeIncome}, nil)

	inp := model.OperationInput{WalletID: wID1, CategoryID: cID1, Type: model.TypeExpense, Amount: 100}
	_, err := svc.CreateOperation(ctx, uID1, inp)
	if !errors.Is(err, service.ErrWrongCategoryType) {
		t.Errorf("got %v, want ErrWrongCategoryType", err)
	}
}

func TestCreateOperation_Success(t *testing.T) {
	svc, _, wallets, cats, ops := newSvc(t)

	now := time.Now()
	wallets.EXPECT().GetByID(ctx, wID1, uID1).Return(&model.Wallet{ID: wID1}, nil)
	cats.EXPECT().GetByID(ctx, cID1, uID1).Return(&model.Category{ID: cID1, Type: model.TypeExpense}, nil)

	inp := model.OperationInput{
		WalletID: wID1, CategoryID: cID1,
		Type: model.TypeExpense, Amount: 100,
		OperatedAt: now, Description: "coffee",
	}
	want := &model.Operation{ID: opID1, UserID: uID1, WalletID: wID1, CategoryID: cID1, Type: model.TypeExpense, Amount: 100}
	ops.EXPECT().Create(ctx, uID1, inp).Return(want, nil)

	got, err := svc.CreateOperation(ctx, uID1, inp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != opID1 {
		t.Errorf("got ID %v, want %v", got.ID, opID1)
	}
}

// ── UpdateOperation ───────────────────────────────────────────────────────────

func TestUpdateOperation_WalletNotFound(t *testing.T) {
	svc, _, wallets, _, _ := newSvc(t)

	wallets.EXPECT().GetByID(ctx, wID99, uID1).Return(nil, storage.ErrNotFound)

	inp := model.OperationInput{WalletID: wID99, CategoryID: cID10, Type: model.TypeExpense, Amount: 100}
	_, err := svc.UpdateOperation(ctx, opID5, uID1, inp)
	if !errors.Is(err, service.ErrNotFound) {
		t.Errorf("got %v, want ErrNotFound", err)
	}
}

func TestUpdateOperation_WrongCategoryType(t *testing.T) {
	svc, _, wallets, cats, _ := newSvc(t)

	wallets.EXPECT().GetByID(ctx, wID1, uID1).Return(&model.Wallet{ID: wID1}, nil)
	cats.EXPECT().GetByID(ctx, cID1, uID1).Return(&model.Category{ID: cID1, Type: model.TypeIncome}, nil)

	inp := model.OperationInput{WalletID: wID1, CategoryID: cID1, Type: model.TypeExpense, Amount: 100}
	_, err := svc.UpdateOperation(ctx, opID5, uID1, inp)
	if !errors.Is(err, service.ErrWrongCategoryType) {
		t.Errorf("got %v, want ErrWrongCategoryType", err)
	}
}

func TestUpdateOperation_OperationNotFound(t *testing.T) {
	svc, _, wallets, cats, ops := newSvc(t)

	now := time.Now()
	wallets.EXPECT().GetByID(ctx, wID1, uID1).Return(&model.Wallet{ID: wID1}, nil)
	cats.EXPECT().GetByID(ctx, cID1, uID1).Return(&model.Category{ID: cID1, Type: model.TypeExpense}, nil)

	inp := model.OperationInput{
		WalletID: wID1, CategoryID: cID1,
		Type: model.TypeExpense, Amount: 100, OperatedAt: now,
	}
	ops.EXPECT().Update(ctx, opID99, uID1, inp).Return(nil, storage.ErrNotFound)

	_, err := svc.UpdateOperation(ctx, opID99, uID1, inp)
	if !errors.Is(err, service.ErrNotFound) {
		t.Errorf("got %v, want ErrNotFound", err)
	}
}

// ── Delete* ───────────────────────────────────────────────────────────────────

func TestDeleteWallet_NotFound(t *testing.T) {
	svc, _, wallets, _, _ := newSvc(t)

	wallets.EXPECT().Delete(ctx, wID1, uID1).Return(storage.ErrNotFound)

	err := svc.DeleteWallet(ctx, wID1, uID1)
	if !errors.Is(err, service.ErrNotFound) {
		t.Errorf("got %v, want ErrNotFound", err)
	}
}

func TestDeleteWallet_Success(t *testing.T) {
	svc, _, wallets, _, _ := newSvc(t)

	wallets.EXPECT().Delete(ctx, wID1, uID1).Return(nil)

	if err := svc.DeleteWallet(ctx, wID1, uID1); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestDeleteCategory_NotFound(t *testing.T) {
	svc, _, _, cats, _ := newSvc(t)

	cats.EXPECT().Delete(ctx, cID5, uID1).Return(storage.ErrNotFound)

	err := svc.DeleteCategory(ctx, cID5, uID1)
	if !errors.Is(err, service.ErrNotFound) {
		t.Errorf("got %v, want ErrNotFound", err)
	}
}

func TestDeleteOperation_NotFound(t *testing.T) {
	svc, _, _, _, ops := newSvc(t)

	ops.EXPECT().Delete(ctx, opID7, uID1).Return(storage.ErrNotFound)

	err := svc.DeleteOperation(ctx, opID7, uID1)
	if !errors.Is(err, service.ErrNotFound) {
		t.Errorf("got %v, want ErrNotFound", err)
	}
}

// ── GetWallet / GetCategory / GetOperation ────────────────────────────────────

func TestGetWallet_NotFound(t *testing.T) {
	svc, _, wallets, _, _ := newSvc(t)

	wallets.EXPECT().GetByID(ctx, wID1, uID1).Return(nil, storage.ErrNotFound)

	_, err := svc.GetWallet(ctx, wID1, uID1)
	if !errors.Is(err, service.ErrNotFound) {
		t.Errorf("got %v, want ErrNotFound", err)
	}
}

func TestGetCategory_NotFound(t *testing.T) {
	svc, _, _, cats, _ := newSvc(t)

	cats.EXPECT().GetByID(ctx, cID1, uID1).Return(nil, storage.ErrNotFound)

	_, err := svc.GetCategory(ctx, cID1, uID1)
	if !errors.Is(err, service.ErrNotFound) {
		t.Errorf("got %v, want ErrNotFound", err)
	}
}

func TestGetOperation_NotFound(t *testing.T) {
	svc, _, _, _, ops := newSvc(t)

	ops.EXPECT().GetByID(ctx, opID1, uID1).Return(nil, storage.ErrNotFound)

	_, err := svc.GetOperation(ctx, opID1, uID1)
	if !errors.Is(err, service.ErrNotFound) {
		t.Errorf("got %v, want ErrNotFound", err)
	}
}

// ── Transfers ─────────────────────────────────────────────────────────────────

func TestCreateTransfer_SameCurrencyCopiesAmount(t *testing.T) {
	svc, _, wallets, _, ops := newSvc(t)

	wallets.EXPECT().GetByID(ctx, wID1, uID1).Return(&model.Wallet{ID: wID1, Currency: "RUB"}, nil)
	wallets.EXPECT().GetByID(ctx, wID2, uID1).Return(&model.Wallet{ID: wID2, Currency: "RUB"}, nil)

	// присланная сумма зачисления в одной валюте игнорируется
	inp := model.TransferInput{FromWalletID: wID1, ToWalletID: wID2, Amount: 50000, ToAmount: 1}
	want := inp
	want.ToAmount = 50000
	ops.EXPECT().CreateTransfer(ctx, uID1, want).Return(&model.Operation{}, &model.Operation{}, nil)

	if _, _, err := svc.CreateTransfer(ctx, uID1, inp); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCreateTransfer_CrossCurrency(t *testing.T) {
	svc, _, wallets, _, ops := newSvc(t)

	wallets.EXPECT().GetByID(ctx, wID1, uID1).Return(&model.Wallet{ID: wID1, Currency: "RUB"}, nil)
	wallets.EXPECT().GetByID(ctx, wID2, uID1).Return(&model.Wallet{ID: wID2, Currency: "USD"}, nil)

	inp := model.TransferInput{FromWalletID: wID1, ToWalletID: wID2, Amount: 900000, ToAmount: 10000}
	ops.EXPECT().CreateTransfer(ctx, uID1, inp).Return(&model.Operation{}, &model.Operation{}, nil)

	if _, _, err := svc.CreateTransfer(ctx, uID1, inp); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCreateTransfer_CrossCurrencyWithoutToAmount(t *testing.T) {
	svc, _, wallets, _, _ := newSvc(t)

	wallets.EXPECT().GetByID(ctx, wID1, uID1).Return(&model.Wallet{ID: wID1, Currency: "RUB"}, nil)
	wallets.EXPECT().GetByID(ctx, wID2, uID1).Return(&model.Wallet{ID: wID2, Currency: "USD"}, nil)

	inp := model.TransferInput{FromWalletID: wID1, ToWalletID: wID2, Amount: 900000}
	_, _, err := svc.CreateTransfer(ctx, uID1, inp)
	if !errors.Is(err, service.ErrToAmountRequired) {
		t.Errorf("got %v, want ErrToAmountRequired", err)
	}
}

func TestUpdateTransfer_CrossCurrencyWithoutToAmount(t *testing.T) {
	svc, _, wallets, _, _ := newSvc(t)

	wallets.EXPECT().GetByID(ctx, wID1, uID1).Return(&model.Wallet{ID: wID1, Currency: "EUR"}, nil)
	wallets.EXPECT().GetByID(ctx, wID2, uID1).Return(&model.Wallet{ID: wID2, Currency: "RUB"}, nil)

	inp := model.TransferInput{FromWalletID: wID1, ToWalletID: wID2, Amount: 10000}
	_, _, err := svc.UpdateTransfer(ctx, opID1, uID1, inp)
	if !errors.Is(err, service.ErrToAmountRequired) {
		t.Errorf("got %v, want ErrToAmountRequired", err)
	}
}

func TestCreateTransfer_SameWallet(t *testing.T) {
	svc, _, _, _, _ := newSvc(t)

	inp := model.TransferInput{FromWalletID: wID1, ToWalletID: wID1, Amount: 100}
	_, _, err := svc.CreateTransfer(ctx, uID1, inp)
	if !errors.Is(err, service.ErrSameWallet) {
		t.Errorf("got %v, want ErrSameWallet", err)
	}
}

// ── Wallets reorder ───────────────────────────────────────────────────────────

func TestReorderWallets(t *testing.T) {
	tooMany := make([]uuid.UUID, 1001)
	for i := range tooMany {
		tooMany[i] = uuid.Must(uuid.NewV7())
	}
	tests := []struct {
		name    string
		ids     []uuid.UUID
		wantErr bool
	}{
		{name: "корректный список", ids: []uuid.UUID{wID1, wID2}},
		{name: "пустой список", ids: nil, wantErr: true},
		{name: "повтор id", ids: []uuid.UUID{wID1, wID2, wID1}, wantErr: true},
		{name: "больше лимита", ids: tooMany, wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc, _, wallets, _, _ := newSvc(t)
			if !tc.wantErr {
				wallets.EXPECT().Reorder(ctx, uID1, tc.ids).Return(nil)
			}
			err := svc.ReorderWallets(ctx, uID1, tc.ids)
			if !tc.wantErr {
				if err != nil {
					t.Errorf("err = %v, want nil", err)
				}
				return
			}
			var ve *service.ValidationError
			if !errors.As(err, &ve) {
				t.Errorf("err = %v, want ValidationError", err)
			}
		})
	}
}

// TestCreateOperation_CategoryTypeRace - FK отклонил запись: тип категории
// сменили между проверкой и INSERT. Для пользователя это та же ошибка типа.
func TestCreateOperation_CategoryTypeRace(t *testing.T) {
	svc, _, wallets, cats, ops := newSvc(t)
	inp := model.OperationInput{
		WalletID: wID1, CategoryID: cID1, Type: model.TypeExpense, Amount: 100,
	}
	wallets.EXPECT().GetByID(ctx, wID1, uID1).Return(&model.Wallet{ID: wID1}, nil)
	cats.EXPECT().GetByID(ctx, cID1, uID1).
		Return(&model.Category{ID: cID1, Type: model.TypeExpense}, nil)
	ops.EXPECT().Create(ctx, uID1, inp).Return(nil, storage.ErrCategoryTypeMismatch)

	if _, err := svc.CreateOperation(ctx, uID1, inp); !errors.Is(err, service.ErrWrongCategoryType) {
		t.Errorf("err = %v, want ErrWrongCategoryType", err)
	}
}

func TestReorderWallets_RepoError(t *testing.T) {
	svc, _, wallets, _, _ := newSvc(t)
	dbErr := errors.New("connection refused")
	wallets.EXPECT().Reorder(ctx, uID1, []uuid.UUID{wID1}).Return(dbErr)

	if err := svc.ReorderWallets(ctx, uID1, []uuid.UUID{wID1}); !errors.Is(err, dbErr) {
		t.Errorf("err = %v, want ошибку репозитория", err)
	}
}

// ── Dashboard ─────────────────────────────────────────────────────────────────

func TestDashboardData_TotalsByCurrency(t *testing.T) {
	svc, _, wallets, _, ops := newSvc(t)

	wallets.EXPECT().ListWithBalance(ctx, uID1).Return([]model.WalletBalance{
		{Wallet: model.Wallet{Currency: "RUB"}, Balance: 100000},
		{Wallet: model.Wallet{Currency: "USD"}, Balance: 5000},
		{Wallet: model.Wallet{Currency: "RUB"}, Balance: -30000},
	}, nil)
	ops.EXPECT().DashboardData(ctx, uID1).Return(nil, nil, nil, nil)

	got, err := svc.DashboardData(ctx, uID1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []model.CurrencyAmount{
		{Currency: "RUB", Amount: 70000},
		{Currency: "USD", Amount: 5000},
	}
	if len(got.TotalBalances) != len(want) {
		t.Fatalf("TotalBalances = %+v, want %+v", got.TotalBalances, want)
	}
	for i := range want {
		if got.TotalBalances[i] != want[i] {
			t.Errorf("TotalBalances[%d] = %+v, want %+v", i, got.TotalBalances[i], want[i])
		}
	}
}
