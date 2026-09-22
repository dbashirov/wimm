package storage_test

import (
	"context"
	"testing"

	"github.com/dbashirov/wimm/internal/model"
	"github.com/dbashirov/wimm/internal/storage"
)

func TestUserStore_CreateWithSeed(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	s := storage.NewFromPool(testPool)

	seed := model.UserSeed{
		Wallet: model.WalletInput{Name: "Основной", Currency: "RUB"},
		Categories: []model.CategoryInput{
			{Name: "Продукты", Type: model.TypeExpense},
			{Name: "Зарплата", Type: model.TypeIncome, Description: "аванс и расчёт"},
		},
	}
	u, err := s.Users.Create(ctx, "seed@test.com", "hash", seed)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	wallets, err := s.Wallets.ListByUser(ctx, u.ID)
	if err != nil {
		t.Fatalf("ListByUser wallets: %v", err)
	}
	if len(wallets) != 1 {
		t.Fatalf("wallets: got %d, want 1", len(wallets))
	}
	if w := wallets[0]; w.Name != "Основной" || w.Currency != "RUB" || !w.IsDefault {
		t.Errorf("unexpected wallet: %+v", w)
	}

	cats, err := s.Categories.ListByUser(ctx, u.ID)
	if err != nil {
		t.Fatalf("ListByUser categories: %v", err)
	}
	got := make(map[string]model.Category, len(cats))
	for _, c := range cats {
		got[c.Name] = c
	}
	for _, want := range seed.Categories {
		c, ok := got[want.Name]
		if !ok {
			t.Errorf("категория %q не создана", want.Name)
			continue
		}
		if c.Type != want.Type || c.Description != want.Description {
			t.Errorf("категория %q: got %+v, want %+v", want.Name, c, want)
		}
	}
	if len(cats) != len(seed.Categories) {
		t.Errorf("categories: got %d, want %d", len(cats), len(seed.Categories))
	}
}
