package storage_test

import (
	"context"
	"testing"

	"github.com/dbashirov/wimm/internal/model"
	"github.com/dbashirov/wimm/internal/storage"
)

func TestCategoryStore_CreateAndGet(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	s := storage.NewFromPool(testPool)

	userID := createUser(t, "cat@test.com")
	c, err := s.Categories.Create(ctx, userID, "Food", model.TypeExpense, "")
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
	c, _ := s.Categories.Create(ctx, owner, "Salary", model.TypeIncome, "")

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
	s.Categories.Create(ctx, userID, "Food", model.TypeExpense, "")      //nolint:errcheck
	s.Categories.Create(ctx, userID, "Transport", model.TypeExpense, "") //nolint:errcheck
	s.Categories.Create(ctx, userID, "Salary", model.TypeIncome, "")    //nolint:errcheck

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
	c, _ := s.Categories.Create(ctx, userID, "Old", model.TypeExpense, "")

	updated, err := s.Categories.Update(ctx, c.ID, userID, "New", "")
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Name != "New" {
		t.Errorf("got name %q, want %q", updated.Name, "New")
	}
	// Тип не меняется при обновлении
	if updated.Type != model.TypeExpense {
		t.Errorf("type changed unexpectedly: %q", updated.Type)
	}
}

func TestCategoryStore_Delete_OtherUser(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	s := storage.NewFromPool(testPool)

	owner := createUser(t, "catdel@test.com")
	other := createUser(t, "catdel2@test.com")
	c, _ := s.Categories.Create(ctx, owner, "Mine", model.TypeExpense, "")

	err := s.Categories.Delete(ctx, c.ID, other)
	if err != storage.ErrNotFound {
		t.Errorf("got %v, want ErrNotFound", err)
	}
}
