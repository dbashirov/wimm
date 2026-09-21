package storage_test

import (
	"context"
	"testing"
	"time"

	"github.com/dbashirov/wimm/internal/model"
	"github.com/dbashirov/wimm/internal/storage"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// reportFixture — набор ID для наполнения отчёта операциями.
type reportFixture struct {
	userID  uuid.UUID
	wallet1 uuid.UUID
	wallet2 uuid.UUID
	expense uuid.UUID
	income  uuid.UUID
}

// newReportFixture создаёт пользователя, два кошелька
// и две категории — расходную и доходную.
func newReportFixture(t *testing.T, email string) reportFixture {
	t.Helper()
	ctx := context.Background()
	s := storage.NewFromPool(testPool)

	f := reportFixture{userID: createUser(t, email)}

	w1, err := s.Wallets.Create(ctx, f.userID, model.WalletInput{Name: "Main", Currency: "RUB"})
	if err != nil {
		t.Fatalf("fixture wallet1: %v", err)
	}
	w2, err := s.Wallets.Create(ctx, f.userID, model.WalletInput{Name: "Card", Currency: "RUB"})
	if err != nil {
		t.Fatalf("fixture wallet2: %v", err)
	}
	exp, err := s.Categories.Create(ctx, f.userID,
		model.CategoryInput{Name: "Food", Type: model.TypeExpense})
	if err != nil {
		t.Fatalf("fixture expense category: %v", err)
	}
	inc, err := s.Categories.Create(ctx, f.userID,
		model.CategoryInput{Name: "Salary", Type: model.TypeIncome})
	if err != nil {
		t.Fatalf("fixture income category: %v", err)
	}

	f.wallet1, f.wallet2, f.expense, f.income = w1.ID, w2.ID, exp.ID, inc.ID
	return f
}

// categorizeTransfers проставляет переводным операциям категории:
// иначе их отсекает INNER JOIN categories, а не условие transfer_id IS NULL.
func categorizeTransfers(t *testing.T, fx reportFixture) {
	t.Helper()
	_, err := testPool.Exec(context.Background(),
		`UPDATE operations
		 SET category_id = CASE WHEN type = 'income' THEN $2::uuid ELSE $3::uuid END
		 WHERE user_id = $1 AND transfer_id IS NOT NULL`,
		fx.userID, fx.income, fx.expense)
	if err != nil {
		t.Fatalf("categorizeTransfers: %v", err)
	}
}

// createOp — операция с явной датой; упрощает наполнение фикстур отчёта.
func createOp(
	t *testing.T,
	userID uuid.UUID,
	walletID uuid.UUID,
	categoryID uuid.UUID,
	opType model.OperationType,
	amount float64,
	at time.Time,
) {
	t.Helper()
	s := storage.NewFromPool(testPool)
	_, err := s.Operations.Create(context.Background(), userID, model.OperationInput{
		WalletID: walletID, CategoryID: categoryID,
		Type: opType, Amount: amount, OperatedAt: at,
	})
	if err != nil {
		t.Fatalf("createOp: %v", err)
	}
}

func utcDate(year int, month time.Month, day int) time.Time {
	return time.Date(year, month, day, 12, 0, 0, 0, time.UTC)
}

// bucketKey — ключ для сверки строк отчёта: бакет в UTC + имя категории.
func bucketKey(row model.ReportRow) string {
	return row.Bucket.UTC().Format("2006-01-02") + "/" + row.Name
}

func TestOperationStore_ReportBounds(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	s := storage.NewFromPool(testPool)

	fx := newReportFixture(t, "bounds@test.com")
	first := utcDate(2025, time.January, 10)
	last := utcDate(2025, time.June, 20)
	createOp(t, fx.userID, fx.wallet1, fx.expense, model.TypeExpense, 100, first)
	createOp(t, fx.userID, fx.wallet1, fx.expense, model.TypeExpense, 200, utcDate(2025, time.March, 5))
	createOp(t, fx.userID, fx.wallet2, fx.income, model.TypeIncome, 300, last)

	got, err := s.Operations.ReportBounds(ctx, model.ReportFilter{UserID: fx.userID})
	if err != nil {
		t.Fatalf("ReportBounds: %v", err)
	}
	if got.Count != 3 {
		t.Errorf("Count = %d, want 3", got.Count)
	}
	if !got.From.Equal(first) {
		t.Errorf("From = %v, want %v", got.From, first)
	}
	if !got.To.Equal(last) {
		t.Errorf("To = %v, want %v", got.To, last)
	}
}

func TestOperationStore_ReportBounds_Empty(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	s := storage.NewFromPool(testPool)

	fx := newReportFixture(t, "boundsempty@test.com")
	createOp(t, fx.userID, fx.wallet1, fx.expense, model.TypeExpense, 100, utcDate(2025, time.January, 10))

	got, err := s.Operations.ReportBounds(ctx, model.ReportFilter{
		UserID:      fx.userID,
		CategoryIDs: []uuid.UUID{uuid.Must(uuid.NewV7())},
	})
	if err != nil {
		t.Fatalf("ReportBounds: %v", err)
	}
	if got.Count != 0 {
		t.Errorf("Count = %d, want 0", got.Count)
	}
	if !got.From.IsZero() || !got.To.IsZero() {
		t.Errorf("bounds = %v..%v, want zero", got.From, got.To)
	}
}

func TestOperationStore_ReportBounds_NilSlicesDoNotFilter(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	s := storage.NewFromPool(testPool)

	fx := newReportFixture(t, "boundsnil@test.com")
	createOp(t, fx.userID, fx.wallet1, fx.expense, model.TypeExpense, 100, utcDate(2025, time.February, 1))
	createOp(t, fx.userID, fx.wallet2, fx.income, model.TypeIncome, 500, utcDate(2025, time.February, 2))

	got, err := s.Operations.ReportBounds(ctx, model.ReportFilter{
		UserID:      fx.userID,
		CategoryIDs: nil,
		WalletIDs:   nil,
	})
	if err != nil {
		t.Fatalf("ReportBounds: %v", err)
	}
	if got.Count != 2 {
		t.Errorf("Count = %d, want 2 (nil-слайсы не должны фильтровать)", got.Count)
	}
}

func TestOperationStore_ReportBounds_FilterByCategoryAndWallet(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	s := storage.NewFromPool(testPool)

	fx := newReportFixture(t, "boundsfilter@test.com")
	target := utcDate(2025, time.April, 7)
	createOp(t, fx.userID, fx.wallet1, fx.expense, model.TypeExpense, 100, target)
	createOp(t, fx.userID, fx.wallet2, fx.expense, model.TypeExpense, 200, utcDate(2025, time.May, 7))
	createOp(t, fx.userID, fx.wallet1, fx.income, model.TypeIncome, 300, utcDate(2025, time.June, 7))

	got, err := s.Operations.ReportBounds(ctx, model.ReportFilter{
		UserID:      fx.userID,
		CategoryIDs: []uuid.UUID{fx.expense},
		WalletIDs:   []uuid.UUID{fx.wallet1},
	})
	if err != nil {
		t.Fatalf("ReportBounds: %v", err)
	}
	if got.Count != 1 {
		t.Fatalf("Count = %d, want 1", got.Count)
	}
	if !got.From.Equal(target) || !got.To.Equal(target) {
		t.Errorf("bounds = %v..%v, want %v", got.From, got.To, target)
	}
}

func TestOperationStore_ReportBounds_TypeFilter(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	s := storage.NewFromPool(testPool)

	fx := newReportFixture(t, "boundstype@test.com")
	createOp(t, fx.userID, fx.wallet1, fx.expense, model.TypeExpense, 100, utcDate(2025, time.March, 1))
	createOp(t, fx.userID, fx.wallet1, fx.income, model.TypeIncome, 900, utcDate(2025, time.March, 2))

	// пустой Type не должен ронять запрос на приведении к enum
	both, err := s.Operations.ReportBounds(ctx, model.ReportFilter{UserID: fx.userID, Type: ""})
	if err != nil {
		t.Fatalf("ReportBounds empty type: %v", err)
	}
	if both.Count != 2 {
		t.Errorf("Count = %d, want 2", both.Count)
	}

	onlyIncome, err := s.Operations.ReportBounds(ctx, model.ReportFilter{
		UserID: fx.userID,
		Type:   model.TypeIncome,
	})
	if err != nil {
		t.Fatalf("ReportBounds income: %v", err)
	}
	if onlyIncome.Count != 1 {
		t.Errorf("income Count = %d, want 1", onlyIncome.Count)
	}
}

func TestOperationStore_ReportBounds_ZeroDates(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	s := storage.NewFromPool(testPool)

	fx := newReportFixture(t, "boundsdates@test.com")
	first := utcDate(2020, time.January, 15)
	last := utcDate(2025, time.December, 31)
	createOp(t, fx.userID, fx.wallet1, fx.expense, model.TypeExpense, 100, first)
	createOp(t, fx.userID, fx.wallet1, fx.expense, model.TypeExpense, 200, last)

	got, err := s.Operations.ReportBounds(ctx, model.ReportFilter{
		UserID:   fx.userID,
		DateFrom: time.Time{},
		DateTo:   time.Time{},
	})
	if err != nil {
		t.Fatalf("ReportBounds: %v", err)
	}
	if got.Count != 2 {
		t.Fatalf("Count = %d, want 2 (нулевые даты не должны обрезать выборку)", got.Count)
	}
	if !got.From.Equal(first) || !got.To.Equal(last) {
		t.Errorf("bounds = %v..%v, want %v..%v", got.From, got.To, first, last)
	}
}

func TestOperationStore_ReportBounds_DateRange(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	s := storage.NewFromPool(testPool)

	fx := newReportFixture(t, "boundsrange@test.com")
	createOp(t, fx.userID, fx.wallet1, fx.expense, model.TypeExpense, 100, utcDate(2025, time.January, 5))
	inRange := utcDate(2025, time.March, 5)
	createOp(t, fx.userID, fx.wallet1, fx.expense, model.TypeExpense, 200, inRange)
	createOp(t, fx.userID, fx.wallet1, fx.expense, model.TypeExpense, 300, utcDate(2025, time.June, 5))

	got, err := s.Operations.ReportBounds(ctx, model.ReportFilter{
		UserID:   fx.userID,
		DateFrom: time.Date(2025, time.February, 1, 0, 0, 0, 0, time.UTC),
		DateTo:   time.Date(2025, time.April, 1, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("ReportBounds: %v", err)
	}
	if got.Count != 1 {
		t.Fatalf("Count = %d, want 1", got.Count)
	}
	if !got.From.Equal(inRange) {
		t.Errorf("From = %v, want %v", got.From, inRange)
	}
}

func TestOperationStore_ReportBounds_OtherUser(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	s := storage.NewFromPool(testPool)

	fx := newReportFixture(t, "boundsown@test.com")
	createOp(t, fx.userID, fx.wallet1, fx.expense, model.TypeExpense, 100, utcDate(2025, time.March, 1))
	other := newReportFixture(t, "boundsoth@test.com")

	got, err := s.Operations.ReportBounds(ctx, model.ReportFilter{UserID: other.userID})
	if err != nil {
		t.Fatalf("ReportBounds: %v", err)
	}
	if got.Count != 0 {
		t.Errorf("Count = %d, want 0 (чужие операции не должны попадать в отчёт)", got.Count)
	}
}

func TestOperationStore_ReportRows_Month(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	s := storage.NewFromPool(testPool)

	fx := newReportFixture(t, "rowsmonth@test.com")
	createOp(t, fx.userID, fx.wallet1, fx.expense, model.TypeExpense, 100, utcDate(2025, time.January, 5))
	createOp(t, fx.userID, fx.wallet1, fx.expense, model.TypeExpense, 50, utcDate(2025, time.January, 20))
	createOp(t, fx.userID, fx.wallet2, fx.income, model.TypeIncome, 900, utcDate(2025, time.February, 3))

	rows, err := s.Operations.ReportRows(ctx,
		model.ReportFilter{UserID: fx.userID}, model.GranularityMonth)
	if err != nil {
		t.Fatalf("ReportRows: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("len(rows) = %d, want 2: %+v", len(rows), rows)
	}

	got := map[string]model.ReportRow{}
	for _, row := range rows {
		got[bucketKey(row)] = row
	}

	jan, ok := got["2025-01-01/Food"]
	if !ok {
		t.Fatalf("нет бакета 2025-01 Food: %+v", rows)
	}
	if jan.Amount != 150 {
		t.Errorf("январь Amount = %v, want 150", jan.Amount)
	}
	if jan.CategoryID != fx.expense {
		t.Errorf("январь CategoryID = %v, want %v", jan.CategoryID, fx.expense)
	}
	if jan.Type != model.TypeExpense {
		t.Errorf("январь Type = %q, want %q", jan.Type, model.TypeExpense)
	}

	feb, ok := got["2025-02-01/Salary"]
	if !ok {
		t.Fatalf("нет бакета 2025-02 Salary: %+v", rows)
	}
	if feb.Amount != 900 || feb.Type != model.TypeIncome {
		t.Errorf("февраль = %v/%q, want 900/%q", feb.Amount, feb.Type, model.TypeIncome)
	}
}

func TestOperationStore_ReportRows_Granularities(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	s := storage.NewFromPool(testPool)

	fx := newReportFixture(t, "rowsgran@test.com")
	// 5 января 2025 — воскресенье, 6 января — понедельник: разные недели
	createOp(t, fx.userID, fx.wallet1, fx.expense, model.TypeExpense, 100, utcDate(2025, time.January, 5))
	createOp(t, fx.userID, fx.wallet1, fx.expense, model.TypeExpense, 50, utcDate(2025, time.January, 6))
	createOp(t, fx.userID, fx.wallet1, fx.expense, model.TypeExpense, 70, utcDate(2025, time.February, 10))
	createOp(t, fx.userID, fx.wallet1, fx.expense, model.TypeExpense, 10, utcDate(2026, time.March, 1))

	tests := []struct {
		name    string
		gran    model.Granularity
		want    int
		buckets []string
	}{
		{"day", model.GranularityDay, 4,
			[]string{"2025-01-05", "2025-01-06", "2025-02-10", "2026-03-01"}},
		{"week", model.GranularityWeek, 4,
			[]string{"2024-12-30", "2025-01-06", "2025-02-10", "2026-02-23"}},
		{"month", model.GranularityMonth, 3,
			[]string{"2025-01-01", "2025-02-01", "2026-03-01"}},
		{"year", model.GranularityYear, 2,
			[]string{"2025-01-01", "2026-01-01"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rows, err := s.Operations.ReportRows(ctx,
				model.ReportFilter{UserID: fx.userID}, tc.gran)
			if err != nil {
				t.Fatalf("ReportRows: %v", err)
			}
			if len(rows) != tc.want {
				t.Fatalf("len(rows) = %d, want %d: %+v", len(rows), tc.want, rows)
			}
			for i, row := range rows {
				if got := row.Bucket.UTC().Format("2006-01-02"); got != tc.buckets[i] {
					t.Errorf("bucket[%d] = %s, want %s", i, got, tc.buckets[i])
				}
			}
		})
	}

	t.Run("месячные суммы схлопывают дни", func(t *testing.T) {
		rows, err := s.Operations.ReportRows(ctx,
			model.ReportFilter{UserID: fx.userID}, model.GranularityMonth)
		if err != nil {
			t.Fatalf("ReportRows: %v", err)
		}
		if rows[0].Amount != 150 {
			t.Errorf("январь = %v, want 150", rows[0].Amount)
		}
	})
}

func TestOperationStore_ReportRows_Filters(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	s := storage.NewFromPool(testPool)

	fx := newReportFixture(t, "rowsfilter@test.com")
	transport := createCategory(t, fx.userID, "Transport", model.TypeExpense)
	fun := createCategory(t, fx.userID, "Fun", model.TypeExpense)

	at := utcDate(2025, time.May, 15)
	createOp(t, fx.userID, fx.wallet1, fx.expense, model.TypeExpense, 100, at)
	createOp(t, fx.userID, fx.wallet2, transport, model.TypeExpense, 200, at)
	createOp(t, fx.userID, fx.wallet2, fun, model.TypeExpense, 300, at)
	createOp(t, fx.userID, fx.wallet1, fx.income, model.TypeIncome, 900, at)

	t.Run("несколько категорий", func(t *testing.T) {
		rows, err := s.Operations.ReportRows(ctx, model.ReportFilter{
			UserID:      fx.userID,
			CategoryIDs: []uuid.UUID{fx.expense, transport},
		}, model.GranularityMonth)
		if err != nil {
			t.Fatalf("ReportRows: %v", err)
		}
		if len(rows) != 2 {
			t.Fatalf("len(rows) = %d, want 2: %+v", len(rows), rows)
		}
		// сортировка по amount DESC внутри бакета
		if rows[0].Name != "Transport" || rows[1].Name != "Food" {
			t.Errorf("порядок = %s, %s; want Transport, Food", rows[0].Name, rows[1].Name)
		}
	})

	t.Run("несколько кошельков", func(t *testing.T) {
		rows, err := s.Operations.ReportRows(ctx, model.ReportFilter{
			UserID:    fx.userID,
			WalletIDs: []uuid.UUID{fx.wallet2},
		}, model.GranularityMonth)
		if err != nil {
			t.Fatalf("ReportRows: %v", err)
		}
		if len(rows) != 2 {
			t.Fatalf("len(rows) = %d, want 2: %+v", len(rows), rows)
		}
		for _, row := range rows {
			if row.Name == "Food" || row.Name == "Salary" {
				t.Errorf("операция чужого кошелька попала в отчёт: %+v", row)
			}
		}
	})

	t.Run("тип операции", func(t *testing.T) {
		rows, err := s.Operations.ReportRows(ctx, model.ReportFilter{
			UserID: fx.userID,
			Type:   model.TypeIncome,
		}, model.GranularityMonth)
		if err != nil {
			t.Fatalf("ReportRows: %v", err)
		}
		if len(rows) != 1 || rows[0].Type != model.TypeIncome || rows[0].Amount != 900 {
			t.Fatalf("rows = %+v, want одну строку дохода на 900", rows)
		}
	})

	t.Run("пустой фильтр отдаёт всё", func(t *testing.T) {
		rows, err := s.Operations.ReportRows(ctx,
			model.ReportFilter{UserID: fx.userID}, model.GranularityMonth)
		if err != nil {
			t.Fatalf("ReportRows: %v", err)
		}
		if len(rows) != 4 {
			t.Errorf("len(rows) = %d, want 4: %+v", len(rows), rows)
		}
	})
}

func TestOperationStore_ReportRows_DateBoundaries(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	s := storage.NewFromPool(testPool)

	fx := newReportFixture(t, "rowsbounds@test.com")
	// 30 марта 2025 — воскресенье (конец недели и месяца), 31 марта — понедельник
	monthEnd := time.Date(2025, time.March, 30, 23, 59, 0, 0, time.UTC)
	monthStart := time.Date(2025, time.March, 31, 0, 0, 0, 0, time.UTC)
	createOp(t, fx.userID, fx.wallet1, fx.expense, model.TypeExpense, 100, monthEnd)
	createOp(t, fx.userID, fx.wallet1, fx.expense, model.TypeExpense, 200, monthStart)
	createOp(t, fx.userID, fx.wallet1, fx.expense, model.TypeExpense, 300, utcDate(2025, time.April, 5))

	t.Run("недельные бакеты разводят воскресенье и понедельник", func(t *testing.T) {
		rows, err := s.Operations.ReportRows(ctx,
			model.ReportFilter{UserID: fx.userID}, model.GranularityWeek)
		if err != nil {
			t.Fatalf("ReportRows: %v", err)
		}
		if len(rows) != 2 {
			t.Fatalf("len(rows) = %d, want 2: %+v", len(rows), rows)
		}
		want := []struct {
			bucket string
			amount float64
		}{
			{"2025-03-24", 100}, // воскресенье 30 марта — ещё прошлая неделя
			{"2025-03-31", 500}, // понедельник 31 марта и суббота 5 апреля — одна неделя
		}
		for i, row := range rows {
			if got := row.Bucket.UTC().Format("2006-01-02"); got != want[i].bucket {
				t.Errorf("bucket[%d] = %s, want %s", i, got, want[i].bucket)
			}
			if row.Amount != want[i].amount {
				t.Errorf("amount[%d] = %v, want %v", i, row.Amount, want[i].amount)
			}
		}
	})

	t.Run("месячные бакеты разводят 30 и 31 марта корректно", func(t *testing.T) {
		rows, err := s.Operations.ReportRows(ctx,
			model.ReportFilter{UserID: fx.userID}, model.GranularityMonth)
		if err != nil {
			t.Fatalf("ReportRows: %v", err)
		}
		if len(rows) != 2 {
			t.Fatalf("len(rows) = %d, want 2: %+v", len(rows), rows)
		}
		if rows[0].Amount != 300 || rows[1].Amount != 300 {
			t.Errorf("суммы = %v, %v; want 300 (март) и 300 (апрель)", rows[0].Amount, rows[1].Amount)
		}
	})

	t.Run("DateTo эксклюзивна", func(t *testing.T) {
		rows, err := s.Operations.ReportRows(ctx, model.ReportFilter{
			UserID:   fx.userID,
			DateFrom: time.Date(2025, time.March, 31, 0, 0, 0, 0, time.UTC),
			DateTo:   time.Date(2025, time.April, 1, 0, 0, 0, 0, time.UTC),
		}, model.GranularityMonth)
		if err != nil {
			t.Fatalf("ReportRows: %v", err)
		}
		if len(rows) != 1 {
			t.Fatalf("len(rows) = %d, want 1: %+v", len(rows), rows)
		}
		if rows[0].Amount != 200 {
			t.Errorf("Amount = %v, want 200 (граница DateFrom включительна, DateTo — нет)", rows[0].Amount)
		}
	})
}

func TestOperationStore_ReportRows_OtherUser(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	s := storage.NewFromPool(testPool)

	fx := newReportFixture(t, "rowsown@test.com")
	createOp(t, fx.userID, fx.wallet1, fx.expense, model.TypeExpense, 100, utcDate(2025, time.March, 1))
	other := newReportFixture(t, "rowsoth@test.com")
	createOp(t, other.userID, other.wallet1, other.expense, model.TypeExpense, 777, utcDate(2025, time.March, 2))

	rows, err := s.Operations.ReportRows(ctx,
		model.ReportFilter{UserID: other.userID}, model.GranularityMonth)
	if err != nil {
		t.Fatalf("ReportRows: %v", err)
	}
	if len(rows) != 1 || rows[0].Amount != 777 {
		t.Fatalf("rows = %+v, want только операцию второго пользователя", rows)
	}
}

func TestOperationStore_ReportRows_ExcludesTransfers(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	s := storage.NewFromPool(testPool)

	fx := newReportFixture(t, "rowstransfer@test.com")
	createOp(t, fx.userID, fx.wallet1, fx.expense, model.TypeExpense, 100, utcDate(2025, time.March, 1))
	_, _, err := s.Operations.CreateTransfer(ctx, fx.userID, model.TransferInput{
		FromWalletID: fx.wallet1,
		ToWalletID:   fx.wallet2,
		Amount:       5000,
		OperatedAt:   utcDate(2025, time.March, 2),
	})
	if err != nil {
		t.Fatalf("CreateTransfer: %v", err)
	}
	// без категорий переводы отсекал бы сам INNER JOIN, и проверка
	// transfer_id IS NULL оставалась бы непокрытой
	categorizeTransfers(t, fx)

	rows, err := s.Operations.ReportRows(ctx,
		model.ReportFilter{UserID: fx.userID}, model.GranularityMonth)
	if err != nil {
		t.Fatalf("ReportRows: %v", err)
	}
	if len(rows) != 1 || rows[0].Amount != 100 {
		t.Fatalf("rows = %+v, want одну строку на 100 (переводы исключены)", rows)
	}
}

func TestOperationStore_ReportRows_NullOperatedAt(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	s := storage.NewFromPool(testPool)

	fx := newReportFixture(t, "rowsnullat@test.com")
	createOp(t, fx.userID, fx.wallet1, fx.expense, model.TypeExpense, 100, utcDate(2025, time.March, 1))
	_, err := testPool.Exec(ctx,
		`INSERT INTO operations(user_id, wallet_id, category_id, type, amount, operated_at)
		 VALUES($1, $2, $3, 'expense', 999, NULL)`,
		fx.userID, fx.wallet1, fx.expense)
	if err != nil {
		t.Fatalf("insert NULL operated_at: %v", err)
	}

	rows, err := s.Operations.ReportRows(ctx,
		model.ReportFilter{UserID: fx.userID}, model.GranularityMonth)
	if err != nil {
		t.Fatalf("ReportRows: %v", err)
	}
	if len(rows) != 1 || rows[0].Amount != 100 {
		t.Fatalf("rows = %+v, want одну строку на 100 (NULL operated_at пропущен)", rows)
	}

	bounds, err := s.Operations.ReportBounds(ctx, model.ReportFilter{UserID: fx.userID})
	if err != nil {
		t.Fatalf("ReportBounds: %v", err)
	}
	if bounds.Count != 1 {
		t.Errorf("Bounds.Count = %d, want 1", bounds.Count)
	}
}

// TestOperationStore_ReportRows_SessionTimezone — бакеты не должны зависеть
// от таймзоны сессии БД: Go-сетка строится строго в UTC, и без
// AT TIME ZONE 'UTC' операция у границы суток уехала бы в соседний бакет.
func TestOperationStore_ReportRows_SessionTimezone(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()

	fx := newReportFixture(t, "rowstz@test.com")
	// 23:30 UTC 31 марта — в Asia/Novosibirsk это уже 1 апреля
	at := time.Date(2025, time.March, 31, 23, 30, 0, 0, time.UTC)
	createOp(t, fx.userID, fx.wallet1, fx.expense, model.TypeExpense, 100, at)

	cfg, err := pgxpool.ParseConfig(testDSN)
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}
	cfg.ConnConfig.RuntimeParams["timezone"] = "Asia/Novosibirsk"
	tzPool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatalf("pgxpool.NewWithConfig: %v", err)
	}
	defer tzPool.Close()

	store := storage.NewFromPool(tzPool).Operations
	for _, g := range []model.Granularity{model.GranularityDay, model.GranularityMonth} {
		t.Run(string(g), func(t *testing.T) {
			rows, err := store.ReportRows(ctx, model.ReportFilter{UserID: fx.userID}, g)
			if err != nil {
				t.Fatalf("ReportRows: %v", err)
			}
			if len(rows) != 1 {
				t.Fatalf("rows = %+v, want одну строку", rows)
			}
			want := at.Truncate(24 * time.Hour)
			if g == model.GranularityMonth {
				want = time.Date(2025, time.March, 1, 0, 0, 0, 0, time.UTC)
			}
			if !rows[0].Bucket.UTC().Equal(want) {
				t.Errorf("Bucket = %v, want %v — бакет уехал вслед за таймзоной сессии",
					rows[0].Bucket.UTC(), want)
			}
		})
	}
}

// TestOperationStore_ReportRows_IncludesHiddenWallets — скрытые кошельки убраны
// только из фильтра в UI; при пустом фильтре их операции обязаны попадать
// в отчёт. Тест страхует от появления условия по wallets.hidden в SQL.
func TestOperationStore_ReportRows_IncludesHiddenWallets(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	s := storage.NewFromPool(testPool)
	fx := newReportFixture(t, "hiddenwallet@test.com")

	if _, err := s.Wallets.ToggleHidden(ctx, fx.wallet2, fx.userID); err != nil {
		t.Fatalf("ToggleHidden: %v", err)
	}
	wallet, err := s.Wallets.GetByID(ctx, fx.wallet2, fx.userID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if !wallet.Hidden {
		t.Fatal("кошелёк не стал скрытым — фикстура неверна")
	}

	createOp(t, fx.userID, fx.wallet1, fx.expense, model.TypeExpense, 100, utcDate(2026, 3, 10))
	createOp(t, fx.userID, fx.wallet2, fx.expense, model.TypeExpense, 700, utcDate(2026, 3, 11))

	rows, err := s.Operations.ReportRows(ctx,
		model.ReportFilter{UserID: fx.userID}, model.GranularityMonth)
	if err != nil {
		t.Fatalf("ReportRows: %v", err)
	}
	var total float64
	for _, row := range rows {
		total += row.Amount
	}
	if total != 800 {
		t.Errorf("итог %.0f, ожидалось 800 — операции скрытого кошелька потеряны", total)
	}

	bounds, err := s.Operations.ReportBounds(ctx, model.ReportFilter{UserID: fx.userID})
	if err != nil {
		t.Fatalf("ReportBounds: %v", err)
	}
	if bounds.Count != 2 {
		t.Errorf("Count = %d, ожидалось 2", bounds.Count)
	}
}
