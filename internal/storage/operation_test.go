package storage_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/dbashirov/wimm/internal/model"
	"github.com/dbashirov/wimm/internal/storage"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
)

// opFixture создаёт пользователя, кошелёк и категорию, возвращает их ID.
func opFixture(t *testing.T, email string) (userID uuid.UUID, walletID, categoryID uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	s := storage.NewFromPool(testPool)

	userID = createUser(t, email)
	w, err := s.Wallets.Create(ctx, userID, model.WalletInput{Name: "Main", Currency: "RUB"})
	if err != nil {
		t.Fatalf("fixture wallet: %v", err)
	}
	c, err := s.Categories.Create(ctx, userID, model.CategoryInput{Name: "Food", Type: model.TypeExpense})
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

	op, err := s.Operations.Create(ctx, userID, model.OperationInput{
		WalletID: walletID, CategoryID: catID,
		Type: model.TypeExpense, Amount: 500.0,
		OperatedAt: now, Description: "lunch",
	})
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

	op, _ := s.Operations.Create(ctx, userID, model.OperationInput{
		WalletID: walletID, CategoryID: catID,
		Type: model.TypeExpense, Amount: 100, OperatedAt: time.Now(),
	})

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
	w2, _ := s.Wallets.Create(ctx, userID, model.WalletInput{Name: "Second", Currency: "USD"})

	now := time.Now()
	s.Operations.Create(ctx, userID, model.OperationInput{ //nolint:errcheck
		WalletID: walletID, CategoryID: catID,
		Type: model.TypeExpense, Amount: 100, OperatedAt: now,
	})
	s.Operations.Create(ctx, userID, model.OperationInput{ //nolint:errcheck
		WalletID: walletID, CategoryID: catID,
		Type: model.TypeExpense, Amount: 200, OperatedAt: now,
	})
	s.Operations.Create(ctx, userID, model.OperationInput{ //nolint:errcheck
		WalletID: w2.ID, CategoryID: catID,
		Type: model.TypeExpense, Amount: 300, OperatedAt: now,
	})

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

	s.Operations.Create(ctx, userID, model.OperationInput{ //nolint:errcheck
		WalletID: walletID, CategoryID: catID,
		Type: model.TypeExpense, Amount: 100,
		OperatedAt: yesterday, Description: "yesterday",
	})
	s.Operations.Create(ctx, userID, model.OperationInput{ //nolint:errcheck
		WalletID: walletID, CategoryID: catID,
		Type: model.TypeExpense, Amount: 200,
		OperatedAt: today, Description: "today",
	})

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

	now := time.Now()
	s.Operations.Create(ctx, u1, model.OperationInput{ //nolint:errcheck
		WalletID: w1, CategoryID: c1,
		Type: model.TypeExpense, Amount: 100, OperatedAt: now,
	})
	s.Operations.Create(ctx, u1, model.OperationInput{ //nolint:errcheck
		WalletID: w1, CategoryID: c1,
		Type: model.TypeExpense, Amount: 200, OperatedAt: now,
	})
	s.Operations.Create(ctx, u2, model.OperationInput{ //nolint:errcheck
		WalletID: w2, CategoryID: c2,
		Type: model.TypeExpense, Amount: 300, OperatedAt: now,
	})

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
	op, _ := s.Operations.Create(ctx, userID, model.OperationInput{
		WalletID: walletID, CategoryID: catID,
		Type: model.TypeExpense, Amount: 100, OperatedAt: now, Description: "old",
	})

	updated, err := s.Operations.Update(ctx, op.ID, userID, model.OperationInput{
		WalletID: walletID, CategoryID: catID,
		Type: model.TypeExpense, Amount: 999, OperatedAt: now, Description: "new",
	})
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
	op, _ := s.Operations.Create(ctx, userID, model.OperationInput{
		WalletID: walletID, CategoryID: catID,
		Type: model.TypeExpense, Amount: 100, OperatedAt: time.Now(),
	})

	err := s.Operations.Delete(ctx, op.ID, other)
	if err != storage.ErrNotFound {
		t.Errorf("got %v, want ErrNotFound", err)
	}
}

// insertBypassingFK вставляет операцию в обход FK (session_replication_role
// отключает их триггеры): после миграции 009 иначе такую строку не создать.
func insertBypassingFK(
	t *testing.T,
	userID, walletID, categoryID uuid.UUID,
) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	conn, err := testPool.Acquire(ctx)
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	defer conn.Release()
	if _, err := conn.Exec(ctx, `SET session_replication_role = replica`); err != nil {
		t.Fatalf("SET replica: %v", err)
	}
	defer conn.Exec(ctx, `SET session_replication_role = DEFAULT`) //nolint:errcheck

	var id uuid.UUID
	err = conn.QueryRow(ctx,
		`INSERT INTO operations(user_id, wallet_id, category_id, type, amount_minor, operated_at)
		 VALUES($1, $2, $3, 'expense', 100, now()) RETURNING id`,
		userID, walletID, categoryID,
	).Scan(&id)
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
	return id
}

// TestOperationStore_ForeignRefsHidden - если в БД всё же окажется операция
// со ссылкой на чужие кошелёк или категорию, чтение не раскрывает их названий.
func TestOperationStore_ForeignRefsHidden(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	s := storage.NewFromPool(testPool)

	owner, walletID, _ := opFixture(t, "owner-refs@test.com")
	_, foreignWallet, foreignCat := opFixture(t, "foreign-refs@test.com")

	opID := insertBypassingFK(t, owner, walletID, foreignCat)
	got, err := s.Operations.GetByID(ctx, opID, owner)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.CategoryName != "" {
		t.Errorf("GetByID вернул чужую категорию %q", got.CategoryName)
	}

	// операция в чужом кошельке не показывается вовсе
	opID = insertBypassingFK(t, owner, foreignWallet, foreignCat)
	if _, err := s.Operations.GetByID(ctx, opID, owner); err != storage.ErrNotFound {
		t.Errorf("операция в чужом кошельке: err = %v, want ErrNotFound", err)
	}
}

// TestOwnerForeignKeys - миграция 009: БД сама не даёт сослаться на чужие
// кошелёк и категорию и разойтись типу операции с типом категории.
func TestOwnerForeignKeys(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	s := storage.NewFromPool(testPool)

	owner, walletID, catID := opFixture(t, "fk-owner@test.com")
	_, foreignWallet, foreignCat := opFixture(t, "fk-foreign@test.com")
	income := createCategory(t, owner, "Зарплата", model.TypeIncome)

	isFKViolation := func(err error) bool {
		var pgErr *pgconn.PgError
		return errors.As(err, &pgErr) && pgErr.Code == "23503"
	}
	create := func(wallet, category uuid.UUID) error {
		_, err := s.Operations.Create(ctx, owner, model.OperationInput{
			WalletID: wallet, CategoryID: category, Type: model.TypeExpense,
			Amount: 100, OperatedAt: time.Now(),
		})
		return err
	}

	if err := create(walletID, foreignCat); !isFKViolation(err) {
		t.Errorf("чужая категория: err = %v, want FK violation", err)
	}
	if err := create(foreignWallet, catID); !isFKViolation(err) {
		t.Errorf("чужой кошелёк: err = %v, want FK violation", err)
	}
	if err := create(walletID, income); !isFKViolation(err) {
		t.Errorf("расход в категории дохода: err = %v, want FK violation", err)
	}
	_, _, err := s.Operations.CreateTransfer(ctx, owner, model.TransferInput{
		FromWalletID: walletID, ToWalletID: foreignWallet,
		Amount: 100, ToAmount: 100, OperatedAt: time.Now(),
	})
	if !isFKViolation(err) {
		t.Errorf("перевод в чужой кошелёк: err = %v, want FK violation", err)
	}

	// смена типа категории с операциями в обход проверки NOT EXISTS
	if err := create(walletID, catID); err != nil {
		t.Fatalf("Create: %v", err)
	}
	_, err = testPool.Exec(ctx, `UPDATE categories SET type = 'income' WHERE id = $1`, catID)
	if !isFKViolation(err) {
		t.Errorf("смена типа категории с операциями: err = %v, want FK violation", err)
	}

	// составные FK не мешают каскадному удалению пользователя
	if _, err := testPool.Exec(ctx, `DELETE FROM users WHERE id = $1`, owner); err != nil {
		t.Errorf("DELETE user: %v", err)
	}
}
