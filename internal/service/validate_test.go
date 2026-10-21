package service_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/dbashirov/wimm/internal/model"
	"github.com/dbashirov/wimm/internal/service"
	"go.uber.org/mock/gomock"
)

// isValidation - ошибка ввода, которую хендлер отдаёт как 400.
func isValidation(err error) bool {
	var ve *service.ValidationError
	return errors.As(err, &ve)
}

// Невалидный ввод не доходит до репозиториев: моки без EXPECT упадут на вызове.

func TestRegister_Validation(t *testing.T) {
	tests := []struct {
		name     string
		email    string
		password string
		wantErr  error // nil - ожидается ValidationError
	}{
		{"короткий пароль", "a@b.com", "1234567", nil},
		{"пароль длиннее 72 байт", "a@b.com", strings.Repeat("п", 37), nil},
		{"длинный email", strings.Repeat("a", 250) + "@b.com", "password", service.ErrInvalidEmail},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc, _, _, _, _ := newSvc(t)
			_, err := svc.Register(ctx, tc.email, tc.password)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Errorf("err = %v, want %v", err, tc.wantErr)
				}
				return
			}
			if !isValidation(err) {
				t.Errorf("err = %v, want ValidationError", err)
			}
		})
	}
}

func TestCreateWallet_Validation(t *testing.T) {
	tests := []struct {
		name string
		inp  model.WalletInput
	}{
		{"пустое название", model.WalletInput{Name: "  ", Currency: "RUB"}},
		{"длинное название", model.WalletInput{Name: strings.Repeat("я", 101), Currency: "RUB"}},
		{"валюта из двух букв", model.WalletInput{Name: "A", Currency: "RU"}},
		{"цифра в валюте", model.WalletInput{Name: "A", Currency: "R1B"}},
		{"кириллица в валюте", model.WalletInput{Name: "A", Currency: "РУБ"}},
		{"длинное описание", model.WalletInput{
			Name: "A", Currency: "RUB", Description: strings.Repeat("я", 501),
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc, _, _, _, _ := newSvc(t)
			if _, err := svc.CreateWallet(ctx, uID1, tc.inp); !isValidation(err) {
				t.Errorf("CreateWallet: err = %v, want ValidationError", err)
			}
			if _, err := svc.UpdateWallet(ctx, wID1, uID1, tc.inp); !isValidation(err) {
				t.Errorf("UpdateWallet: err = %v, want ValidationError", err)
			}
		})
	}
}

func TestCreateWallet_Normalizes(t *testing.T) {
	svc, _, wallets, _, _ := newSvc(t)
	want := model.WalletInput{Name: "Карта", Currency: "USD"}
	wallets.EXPECT().Create(ctx, uID1, want).Return(&model.Wallet{}, nil)

	_, err := svc.CreateWallet(ctx, uID1, model.WalletInput{Name: " Карта ", Currency: " usd"})
	if err != nil {
		t.Fatalf("CreateWallet: %v", err)
	}
}

func TestCreateCategory_Validation(t *testing.T) {
	tests := []struct {
		name string
		inp  model.CategoryInput
	}{
		{"пустое название", model.CategoryInput{Name: "", Type: model.TypeExpense}},
		{"неизвестный тип", model.CategoryInput{Name: "A", Type: "transfer"}},
		{"длинное описание", model.CategoryInput{
			Name: "A", Type: model.TypeIncome, Description: strings.Repeat("я", 501),
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc, _, _, _, _ := newSvc(t)
			if _, err := svc.CreateCategory(ctx, uID1, tc.inp); !isValidation(err) {
				t.Errorf("err = %v, want ValidationError", err)
			}
		})
	}
}

func TestUpdateCategory_Validation(t *testing.T) {
	svc, _, _, _, _ := newSvc(t)
	if _, err := svc.UpdateCategory(ctx, cID1, uID1, " ", ""); !isValidation(err) {
		t.Errorf("err = %v, want ValidationError", err)
	}
}

func TestListCategoriesByType_Validation(t *testing.T) {
	svc, _, _, _, _ := newSvc(t)
	if _, err := svc.ListCategoriesByType(ctx, uID1, "bogus"); !isValidation(err) {
		t.Errorf("err = %v, want ValidationError", err)
	}
}

func TestOperationDescription_Validation(t *testing.T) {
	long := strings.Repeat("я", 501)
	svc, _, _, _, _ := newSvc(t)

	op := model.OperationInput{
		WalletID: wID1, CategoryID: cID1, Type: model.TypeExpense,
		Amount: 100, Description: long,
	}
	if _, err := svc.CreateOperation(ctx, uID1, op); !isValidation(err) {
		t.Errorf("CreateOperation: err = %v, want ValidationError", err)
	}
	if _, err := svc.UpdateOperation(ctx, opID1, uID1, op); !isValidation(err) {
		t.Errorf("UpdateOperation: err = %v, want ValidationError", err)
	}

	tr := model.TransferInput{FromWalletID: wID1, ToWalletID: wID2, Amount: 100, Description: long}
	if _, _, err := svc.CreateTransfer(ctx, uID1, tr); !isValidation(err) {
		t.Errorf("CreateTransfer: err = %v, want ValidationError", err)
	}
}

// TestDescription_MaxLenAllowed - граница включительно: 500 символов проходят.
func TestDescription_MaxLenAllowed(t *testing.T) {
	svc, _, wallets, _, _ := newSvc(t)
	inp := model.WalletInput{Name: "A", Currency: "RUB", Description: strings.Repeat("я", 500)}
	wallets.EXPECT().Create(ctx, uID1, gomock.Any()).Return(&model.Wallet{}, nil)

	if _, err := svc.CreateWallet(ctx, uID1, inp); err != nil {
		t.Errorf("CreateWallet: %v", err)
	}
}
