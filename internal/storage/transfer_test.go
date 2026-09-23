package storage_test

import (
	"context"
	"testing"
	"time"

	"github.com/dbashirov/wimm/internal/model"
	"github.com/dbashirov/wimm/internal/storage"
)

// TestOperationStore_CrossCurrencyTransfer — у списания и зачисления свои суммы,
// каждая в валюте своего кошелька, и балансы кошельков считаются по ним.
func TestOperationStore_CrossCurrencyTransfer(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	s := storage.NewFromPool(testPool)

	userID := createUser(t, "fx@test.com")
	rub, err := s.Wallets.Create(ctx, userID, model.WalletInput{Name: "Карта", Currency: "RUB"})
	if err != nil {
		t.Fatalf("wallet RUB: %v", err)
	}
	usd, err := s.Wallets.Create(ctx, userID, model.WalletInput{Name: "Доллары", Currency: "USD"})
	if err != nil {
		t.Fatalf("wallet USD: %v", err)
	}

	expense, income, err := s.Operations.CreateTransfer(ctx, userID, model.TransferInput{
		FromWalletID: rub.ID,
		ToWalletID:   usd.ID,
		Amount:       900050,
		ToAmount:     10001,
		OperatedAt:   time.Now().UTC().Truncate(time.Minute),
	})
	if err != nil {
		t.Fatalf("CreateTransfer: %v", err)
	}
	if expense.Amount != 900050 || expense.Currency != "RUB" {
		t.Errorf("списание = %d %s, want 900050 RUB", expense.Amount, expense.Currency)
	}
	if income.Amount != 10001 || income.Currency != "USD" {
		t.Errorf("зачисление = %d %s, want 10001 USD", income.Amount, income.Currency)
	}

	balances := func() map[string]model.Money {
		t.Helper()
		list, err := s.Wallets.ListWithBalance(ctx, userID)
		if err != nil {
			t.Fatalf("ListWithBalance: %v", err)
		}
		got := make(map[string]model.Money, len(list))
		for _, wb := range list {
			got[wb.Currency] = wb.Balance
		}
		return got
	}
	if got := balances(); got["RUB"] != -900050 || got["USD"] != 10001 {
		t.Errorf("балансы после перевода = %v", got)
	}

	_, _, err = s.Operations.UpdateTransfer(ctx, *expense.TransferID, userID, model.TransferInput{
		FromWalletID: rub.ID,
		ToWalletID:   usd.ID,
		Amount:       450000,
		ToAmount:     5000,
		OperatedAt:   expense.OperatedAt,
	})
	if err != nil {
		t.Fatalf("UpdateTransfer: %v", err)
	}
	gotExp, gotInc, err := s.Operations.GetTransfer(ctx, *expense.TransferID, userID)
	if err != nil {
		t.Fatalf("GetTransfer: %v", err)
	}
	if gotExp.Amount != 450000 || gotInc.Amount != 5000 {
		t.Errorf("после правки: списание %d, зачисление %d; want 450000 / 5000",
			gotExp.Amount, gotInc.Amount)
	}
	if got := balances(); got["RUB"] != -450000 || got["USD"] != 5000 {
		t.Errorf("балансы после правки = %v", got)
	}
}
