package storage_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/dbashirov/wimm/internal/model"
	"github.com/dbashirov/wimm/internal/storage"
)

func TestCategoryStore_CreateAndGet(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	s := storage.NewFromPool(testPool)

	userID := createUser(t, "cat@test.com")
	c, err := s.Categories.Create(ctx, userID, model.CategoryInput{Name: "Food", Type: model.TypeExpense})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if c.Name != "Food" || c.Type != model.TypeExpense || c.UserID != userID {
		t.Errorf("unexpected category: %+v", c)
	}

	got, err := s.Categories.GetByID(ctx, c.ID, userID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.ID != c.ID {
		t.Errorf("got ID %v, want %v", got.ID, c.ID)
	}
}

func TestCategoryStore_GetByID_OtherUser(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	s := storage.NewFromPool(testPool)

	owner := createUser(t, "catown@test.com")
	other := createUser(t, "catoth@test.com")
	c, _ := s.Categories.Create(ctx, owner, model.CategoryInput{Name: "Salary", Type: model.TypeIncome})

	_, err := s.Categories.GetByID(ctx, c.ID, other)
	if err != storage.ErrNotFound {
		t.Errorf("got %v, want ErrNotFound", err)
	}
}

func TestCategoryStore_ListByType(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	s := storage.NewFromPool(testPool)

	userID := createUser(t, "catlist@test.com")
	s.Categories.Create(ctx, userID, model.CategoryInput{Name: "Food", Type: model.TypeExpense})      //nolint:errcheck
	s.Categories.Create(ctx, userID, model.CategoryInput{Name: "Transport", Type: model.TypeExpense}) //nolint:errcheck
	s.Categories.Create(ctx, userID, model.CategoryInput{Name: "Salary", Type: model.TypeIncome})     //nolint:errcheck

	expenses, err := s.Categories.ListByType(ctx, userID, model.TypeExpense)
	if err != nil {
		t.Fatalf("ListByType: %v", err)
	}
	if len(expenses) != 2 {
		t.Errorf("got %d expenses, want 2", len(expenses))
	}

	incomes, err := s.Categories.ListByType(ctx, userID, model.TypeIncome)
	if err != nil {
		t.Fatalf("ListByType income: %v", err)
	}
	if len(incomes) != 1 {
		t.Errorf("got %d incomes, want 1", len(incomes))
	}
}

func TestCategoryStore_Update(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	s := storage.NewFromPool(testPool)

	userID := createUser(t, "catupd@test.com")
	c, _ := s.Categories.Create(ctx, userID, model.CategoryInput{Name: "Old", Type: model.TypeExpense})

	updated, err := s.Categories.Update(ctx, c.ID, userID, "New", model.TypeExpense)
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Name != "New" {
		t.Errorf("got name %q, want %q", updated.Name, "New")
	}
	if updated.Type != model.TypeExpense {
		t.Errorf("type changed unexpectedly: %q", updated.Type)
	}
}

func TestCategoryStore_UpdateType(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	s := storage.NewFromPool(testPool)

	userID := createUser(t, "cattype@test.com")
	wallet, err := s.Wallets.Create(ctx, userID, model.WalletInput{Name: "W", Currency: "RUB"})
	if err != nil {
		t.Fatalf("Wallets.Create: %v", err)
	}
	empty, _ := s.Categories.Create(ctx, userID,
		model.CategoryInput{Name: "Пустая", Type: model.TypeExpense, Description: "оставить"})
	used, _ := s.Categories.Create(ctx, userID,
		model.CategoryInput{Name: "С операциями", Type: model.TypeExpense})
	_, err = s.Operations.Create(ctx, userID, model.OperationInput{
		WalletID: wallet.ID, CategoryID: used.ID, Type: model.TypeExpense,
		Amount: 100, OperatedAt: time.Now(),
	})
	if err != nil {
		t.Fatalf("Operations.Create: %v", err)
	}

	t.Run("без операций тип меняется, описание сохраняется", func(t *testing.T) {
		got, err := s.Categories.Update(ctx, empty.ID, userID, "Пустая", model.TypeIncome)
		if err != nil {
			t.Fatalf("Update: %v", err)
		}
		if got.Type != model.TypeIncome || got.Description != "оставить" {
			t.Errorf("got type %q description %q", got.Type, got.Description)
		}
	})
	t.Run("с операциями тип не меняется", func(t *testing.T) {
		_, err := s.Categories.Update(ctx, used.ID, userID, "С операциями", model.TypeIncome)
		if !errors.Is(err, storage.ErrCategoryInUse) {
			t.Fatalf("err = %v, want ErrCategoryInUse", err)
		}
		got, _ := s.Categories.GetByID(ctx, used.ID, userID)
		if got.Type != model.TypeExpense {
			t.Errorf("тип изменился: %q", got.Type)
		}
	})
	t.Run("с операциями название меняется", func(t *testing.T) {
		got, err := s.Categories.Update(ctx, used.ID, userID, "Переименована", model.TypeExpense)
		if err != nil {
			t.Fatalf("Update: %v", err)
		}
		if got.Name != "Переименована" {
			t.Errorf("name = %q", got.Name)
		}
	})
	t.Run("чужая категория", func(t *testing.T) {
		other := createUser(t, "cattype-other@test.com")
		_, err := s.Categories.Update(ctx, used.ID, other, "X", model.TypeIncome)
		if !errors.Is(err, storage.ErrNotFound) {
			t.Errorf("err = %v, want ErrNotFound", err)
		}
	})
}

func TestCategoryStore_Delete_OtherUser(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	s := storage.NewFromPool(testPool)

	owner := createUser(t, "catdel@test.com")
	other := createUser(t, "catdel2@test.com")
	c, _ := s.Categories.Create(ctx, owner, model.CategoryInput{Name: "Mine", Type: model.TypeExpense})

	err := s.Categories.Delete(ctx, c.ID, other)
	if err != storage.ErrNotFound {
		t.Errorf("got %v, want ErrNotFound", err)
	}
}
