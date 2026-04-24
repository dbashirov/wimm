package storage_test

import (
	"context"
	"testing"

	"github.com/dbashirov/wimm/internal/storage"
)

func TestWalletStore_CreateAndGet(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	s := storage.NewFromPool(testPool)

	userID := createUser(t, "wallet@test.com")
	w, err := s.Wallets.Create(ctx, userID, "Cash", "RUB", "")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if w.Name != "Cash" || w.Currency != "RUB" || w.UserID != userID {
		t.Errorf("unexpected wallet: %+v", w)
	}

	got, err := s.Wallets.GetByID(ctx, w.ID, userID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.ID != w.ID {
		t.Errorf("got ID %v, want %v", got.ID, w.ID)
	}
}

func TestWalletStore_GetByID_OtherUser(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	s := storage.NewFromPool(testPool)

	owner := createUser(t, "owner@test.com")
	other := createUser(t, "other@test.com")
	w, _ := s.Wallets.Create(ctx, owner, "Savings", "USD", "")

	_, err := s.Wallets.GetByID(ctx, w.ID, other)
	if err != storage.ErrNotFound {
		t.Errorf("got %v, want ErrNotFound", err)
	}
}

func TestWalletStore_ListByUser(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	s := storage.NewFromPool(testPool)

	u1 := createUser(t, "u1@test.com")
	u2 := createUser(t, "u2@test.com")
	s.Wallets.Create(ctx, u1, "W1", "RUB", "") //nolint:errcheck
	s.Wallets.Create(ctx, u1, "W2", "USD", "") //nolint:errcheck
	s.Wallets.Create(ctx, u2, "W3", "EUR", "") //nolint:errcheck

	wallets, err := s.Wallets.ListByUser(ctx, u1)
	if err != nil {
		t.Fatalf("ListByUser: %v", err)
	}
	if len(wallets) != 2 {
		t.Errorf("got %d wallets, want 2", len(wallets))
	}
}

func TestWalletStore_Update(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	s := storage.NewFromPool(testPool)

	userID := createUser(t, "upd@test.com")
	w, _ := s.Wallets.Create(ctx, userID, "Old", "RUB", "")

	updated, err := s.Wallets.Update(ctx, w.ID, userID, "New", "USD", "")
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Name != "New" || updated.Currency != "USD" {
		t.Errorf("unexpected: %+v", updated)
	}
}

func TestWalletStore_Update_OtherUser(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	s := storage.NewFromPool(testPool)

	owner := createUser(t, "own@test.com")
	other := createUser(t, "oth@test.com")
	w, _ := s.Wallets.Create(ctx, owner, "Mine", "RUB", "")

	_, err := s.Wallets.Update(ctx, w.ID, other, "Stolen", "USD", "")
	if err != storage.ErrNotFound {
		t.Errorf("got %v, want ErrNotFound", err)
	}
}

func TestWalletStore_Delete(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	s := storage.NewFromPool(testPool)

	userID := createUser(t, "del@test.com")
	w, _ := s.Wallets.Create(ctx, userID, "ToDelete", "RUB", "")

	if err := s.Wallets.Delete(ctx, w.ID, userID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	_, err := s.Wallets.GetByID(ctx, w.ID, userID)
	if err != storage.ErrNotFound {
		t.Errorf("after delete: got %v, want ErrNotFound", err)
	}
}

func TestWalletStore_Delete_OtherUser(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	s := storage.NewFromPool(testPool)

	owner := createUser(t, "own2@test.com")
	other := createUser(t, "oth2@test.com")
	w, _ := s.Wallets.Create(ctx, owner, "Mine", "RUB", "")

	err := s.Wallets.Delete(ctx, w.ID, other)
	if err != storage.ErrNotFound {
		t.Errorf("got %v, want ErrNotFound", err)
	}
}
