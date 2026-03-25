package storage_test

import (
	"context"
	"testing"
	"time"

	"github.com/dbashirov/wimm/internal/model"
	"github.com/dbashirov/wimm/internal/storage"
)

// fixture создаёт пользователя, кошелёк и категорию, возвращает их ID.
func opFixture(t *testing.T, email string) (userID, walletID, categoryID int64) {
	t.Helper()
	ctx := context.Background()
	s := storage.NewFromPool(testPool)

	userID = createUser(t, email)
	w, err := s.Wallets.Create(ctx, userID, "Main", "RUB")
	if err != nil {
		t.Fatalf("fixture wallet: %v", err)
	}
	c, err := s.Categories.Create(ctx, userID, "Food", "expense")
	if err != nil {
		t.Fatalf("fixture category: %v", err)
	}
	return userID, w.ID, c.ID
}

func TestOperationStore_CreateAndGet(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	s := storage.NewFromPool(testPool)

	userID, walletID, catID := opFixture(t, "op@test.com")
	now := time.Now().Truncate(time.Minute)

	op, err := s.Operations.Create(ctx, userID, walletID, catID, "expense", 500.0, now, "lunch")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if op.Amount != 500.0 || op.Description != "lunch" {
		t.Errorf("unexpected op: %+v", op)
	}

	got, err := s.Operations.GetByID(ctx, op.ID, userID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.WalletName != "Main" || got.CategoryName != "Food" {
		t.Errorf("joined fields: wallet=%q category=%q", got.WalletName, got.CategoryName)
	}
}

func TestOperationStore_GetByID_OtherUser(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	s := storage.NewFromPool(testPool)

	userID, walletID, catID := opFixture(t, "opown@test.com")
	other := createUser(t, "opoth@test.com")

	op, _ := s.Operations.Create(ctx, userID, walletID, catID, "expense", 100, time.Now(), "")

	_, err := s.Operations.GetByID(ctx, op.ID, other)
	if err != storage.ErrNotFound {
		t.Errorf("got %v, want ErrNotFound", err)
	}
}

func TestOperationStore_List_FilterByWallet(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	s := storage.NewFromPool(testPool)

	userID, walletID, catID := opFixture(t, "oplist@test.com")
	w2, _ := s.Wallets.Create(ctx, userID, "Second", "USD")

	now := time.Now()
	s.Operations.Create(ctx, userID, walletID, catID, "expense", 100, now, "") //nolint:errcheck
	s.Operations.Create(ctx, userID, walletID, catID, "expense", 200, now, "") //nolint:errcheck
	s.Operations.Create(ctx, userID, w2.ID, catID, "expense", 300, now, "")   //nolint:errcheck

	ops, err := s.Operations.List(ctx, model.OperationFilter{UserID: userID, WalletID: walletID})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(ops) != 2 {
		t.Errorf("got %d ops, want 2", len(ops))
	}
}

func TestOperationStore_List_FilterByDateRange(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	s := storage.NewFromPool(testPool)

	userID, walletID, catID := opFixture(t, "opdate@test.com")
	today := time.Now().Truncate(24 * time.Hour)
	yesterday := today.AddDate(0, 0, -1)
	tomorrow := today.AddDate(0, 0, 1)

	s.Operations.Create(ctx, userID, walletID, catID, "expense", 100, yesterday, "yesterday") //nolint:errcheck
	s.Operations.Create(ctx, userID, walletID, catID, "expense", 200, today, "today")         //nolint:errcheck

	// Фильтр: только сегодня (DateTo = начало завтра, SQL: < DateTo)
	ops, err := s.Operations.List(ctx, model.OperationFilter{
		UserID:   userID,
		DateFrom: today,
		DateTo:   tomorrow,
	})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(ops) != 1 || ops[0].Description != "today" {
		t.Errorf("expected only today's op, got %d ops", len(ops))
	}
}

func TestOperationStore_List_IsolatesByUser(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	s := storage.NewFromPool(testPool)

	u1, w1, c1 := opFixture(t, "opiso1@test.com")
	u2, w2, c2 := opFixture(t, "opiso2@test.com")

	s.Operations.Create(ctx, u1, w1, c1, "expense", 100, time.Now(), "") //nolint:errcheck
	s.Operations.Create(ctx, u1, w1, c1, "expense", 200, time.Now(), "") //nolint:errcheck
	s.Operations.Create(ctx, u2, w2, c2, "expense", 300, time.Now(), "") //nolint:errcheck

	ops, _ := s.Operations.List(ctx, model.OperationFilter{UserID: u1})
	if len(ops) != 2 {
		t.Errorf("user1: got %d ops, want 2", len(ops))
	}

	ops, _ = s.Operations.List(ctx, model.OperationFilter{UserID: u2})
	if len(ops) != 1 {
		t.Errorf("user2: got %d ops, want 1", len(ops))
	}
}

func TestOperationStore_Update(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	s := storage.NewFromPool(testPool)

	userID, walletID, catID := opFixture(t, "opupd@test.com")
	now := time.Now().Truncate(time.Minute)
	op, _ := s.Operations.Create(ctx, userID, walletID, catID, "expense", 100, now, "old")

	updated, err := s.Operations.Update(ctx, op.ID, userID, walletID, catID, "expense", 999, now, "new")
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Amount != 999 || updated.Description != "new" {
		t.Errorf("unexpected update: %+v", updated)
	}
}

func TestOperationStore_Delete_OtherUser(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	s := storage.NewFromPool(testPool)

	userID, walletID, catID := opFixture(t, "opdel@test.com")
	other := createUser(t, "opdel2@test.com")
	op, _ := s.Operations.Create(ctx, userID, walletID, catID, "expense", 100, time.Now(), "")

	err := s.Operations.Delete(ctx, op.ID, other)
	if err != storage.ErrNotFound {
		t.Errorf("got %v, want ErrNotFound", err)
	}
}
