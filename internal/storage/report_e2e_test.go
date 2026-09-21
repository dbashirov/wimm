package storage_test

import (
	"context"
	"math"
	"slices"
	"testing"
	"time"

	"github.com/dbashirov/wimm/internal/model"
	"github.com/dbashirov/wimm/internal/service"
	"github.com/dbashirov/wimm/internal/storage"
	"github.com/google/uuid"
)

// Сквозная проверка отчёта: Service.Report поверх живой БД против прямых
// SQL-запросов к той же выборке. Тест живёт в пакете storage_test, потому что
// только здесь поднят PostgreSQL через testcontainers.

// reportE2EFixture — фикстура сквозного отчёта: две дополнительные категории
// расходов поверх базовой.
type reportE2EFixture struct {
	reportFixture
	transport uuid.UUID
	fun       uuid.UUID
}

// utcStart — полночь UTC, граница периода в фильтре отчёта.
func utcStart(year int, month time.Month, day int) time.Time {
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}

// seedReportE2E наполняет БД операциями за январь–апрель 2025:
// март намеренно пуст, переводы и операции чужого пользователя — шум.
func seedReportE2E(t *testing.T) reportE2EFixture {
	t.Helper()
	ctx := context.Background()
	s := storage.NewFromPool(testPool)

	fx := reportE2EFixture{reportFixture: newReportFixture(t, "e2e@test.com")}
	fx.transport = createCategory(t, fx.userID, "Транспорт", model.TypeExpense)
	fx.fun = createCategory(t, fx.userID, "Развлечения", model.TypeExpense)

	createOp(t, fx.userID, fx.wallet1, fx.expense, model.TypeExpense, 100, utcDate(2025, time.January, 10))
	createOp(t, fx.userID, fx.wallet1, fx.transport, model.TypeExpense, 50, utcDate(2025, time.January, 20))
	createOp(t, fx.userID, fx.wallet2, fx.expense, model.TypeExpense, 200, utcDate(2025, time.February, 5))
	createOp(t, fx.userID, fx.wallet2, fx.fun, model.TypeExpense, 70, utcDate(2025, time.February, 14))
	createOp(t, fx.userID, fx.wallet1, fx.expense, model.TypeExpense, 300, utcDate(2025, time.April, 15))
	createOp(t, fx.userID, fx.wallet1, fx.income, model.TypeIncome, 1000, utcDate(2025, time.January, 25))
	createOp(t, fx.userID, fx.wallet2, fx.income, model.TypeIncome, 2000, utcDate(2025, time.April, 1))

	_, _, err := s.Operations.CreateTransfer(ctx, fx.userID, model.TransferInput{
		FromWalletID: fx.wallet1,
		ToWalletID:   fx.wallet2,
		Amount:       5000,
		OperatedAt:   utcDate(2025, time.February, 10),
	})
	if err != nil {
		t.Fatalf("CreateTransfer: %v", err)
	}
	// без категорий переводы отсекал бы сам INNER JOIN categories,
	// и условие transfer_id IS NULL осталось бы непроверенным
	categorizeTransfers(t, fx.reportFixture)

	other := newReportFixture(t, "e2e-other@test.com")
	createOp(t, other.userID, other.wallet1, other.expense, model.TypeExpense, 999, utcDate(2025, time.February, 1))

	return fx
}

// scalarFromSQL выполняет прямой запрос к той же БД — эталон для сверки сумм.
func scalarFromSQL(t *testing.T, query string, args ...any) float64 {
	t.Helper()
	var value float64
	if err := testPool.QueryRow(context.Background(), query, args...).Scan(&value); err != nil {
		t.Fatalf("прямой SQL-запрос: %v", err)
	}
	return value
}

func approxEqual(got, want float64) bool {
	return math.Abs(got-want) < 1e-6
}

func TestReportE2E_AgainstDirectSQL(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	fx := seedReportE2E(t)
	svc := service.New(storage.NewFromPool(testPool))

	const sumByCategories = `
		SELECT COALESCE(SUM(o.amount), 0)::float8
		FROM operations o
		WHERE o.user_id = $1
		  AND o.transfer_id IS NULL
		  AND o.category_id = ANY($2::uuid[])
		  AND ($3::timestamptz IS NULL OR o.operated_at >= $3)
		  AND ($4::timestamptz IS NULL OR o.operated_at <  $4)`

	t.Run("одна категория за период", func(t *testing.T) {
		from, to := utcStart(2025, time.January, 1), utcStart(2025, time.March, 1)
		rep, err := svc.Report(ctx, model.ReportFilter{
			UserID:      fx.userID,
			CategoryIDs: []uuid.UUID{fx.expense},
			DateFrom:    from,
			DateTo:      to,
		})
		if err != nil {
			t.Fatalf("Report: %v", err)
		}
		want := scalarFromSQL(t, sumByCategories, fx.userID, []uuid.UUID{fx.expense}, from, to)
		if rep.Expense == nil {
			t.Fatalf("секция расходов пуста, ожидалась сумма %v", want)
		}
		if !approxEqual(rep.Expense.Total, want) {
			t.Errorf("Expense.Total = %v, прямой SQL даёт %v", rep.Expense.Total, want)
		}
		if rep.Income != nil {
			t.Errorf("Income = %+v, ожидался nil: выбрана только категория расходов", rep.Income)
		}
	})

	t.Run("несколько категорий — итог равен сумме категорий", func(t *testing.T) {
		ids := []uuid.UUID{fx.expense, fx.transport}
		rep, err := svc.Report(ctx, model.ReportFilter{UserID: fx.userID, CategoryIDs: ids})
		if err != nil {
			t.Fatalf("Report: %v", err)
		}
		want := scalarFromSQL(t, sumByCategories, fx.userID, ids, nil, nil)
		if rep.Expense == nil {
			t.Fatalf("секция расходов пуста, ожидалась сумма %v", want)
		}
		if !approxEqual(rep.Expense.Total, want) {
			t.Errorf("Expense.Total = %v, прямой SQL даёт %v", rep.Expense.Total, want)
		}

		var byCategories, byBuckets float64
		for _, cat := range rep.Expense.Categories {
			byCategories += cat.Total
		}
		for _, bucket := range rep.Expense.Buckets {
			byBuckets += bucket.Total
		}
		if !approxEqual(byCategories, want) {
			t.Errorf("сумма по категориям = %v, прямой SQL даёт %v", byCategories, want)
		}
		if !approxEqual(byBuckets, want) {
			t.Errorf("сумма по бакетам = %v, прямой SQL даёт %v", byBuckets, want)
		}
	})

	t.Run("без фильтров — отчёт за всё время с авто-гранулярностью", func(t *testing.T) {
		rep, err := svc.Report(ctx, model.ReportFilter{UserID: fx.userID})
		if err != nil {
			t.Fatalf("Report: %v", err)
		}
		if rep.Bounds.Count == 0 || rep.Expense == nil || rep.Income == nil {
			t.Fatalf("пустой отчёт при nil-слайсах фильтра: %+v", rep)
		}
		// январь–апрель 2025 — 95 дней, порог месяца по таблице авто-гранулярности
		if rep.Granularity != model.GranularityMonth {
			t.Errorf("Granularity = %q, ожидался month (авто)", rep.Granularity)
		}
		if rep.Notice != "" {
			t.Errorf("Notice = %q, ожидалась пустая строка", rep.Notice)
		}
		if len(rep.Labels) != 4 {
			t.Errorf("Labels = %v, ожидались январь–апрель", rep.Labels)
		}

		const sumByType = `
			SELECT COALESCE(SUM(o.amount), 0)::float8
			FROM operations o
			JOIN categories c ON c.id = o.category_id AND c.user_id = o.user_id
			WHERE o.user_id = $1 AND o.transfer_id IS NULL AND o.type = $2`
		wantExpense := scalarFromSQL(t, sumByType, fx.userID, model.TypeExpense)
		wantIncome := scalarFromSQL(t, sumByType, fx.userID, model.TypeIncome)
		if !approxEqual(rep.Expense.Total, wantExpense) {
			t.Errorf("Expense.Total = %v, прямой SQL даёт %v", rep.Expense.Total, wantExpense)
		}
		if !approxEqual(rep.Income.Total, wantIncome) {
			t.Errorf("Income.Total = %v, прямой SQL даёт %v", rep.Income.Total, wantIncome)
		}
		if !approxEqual(rep.TotalNet, wantIncome-wantExpense) {
			t.Errorf("TotalNet = %v, ожидалось %v", rep.TotalNet, wantIncome-wantExpense)
		}
	})

	t.Run("месяц без операций остаётся в сетке с нулём", func(t *testing.T) {
		rep, err := svc.Report(ctx, model.ReportFilter{UserID: fx.userID})
		if err != nil {
			t.Fatalf("Report: %v", err)
		}
		// бакеты выровнены по Labels, отдельной даты у бакета нет
		idx := slices.Index(rep.Labels, "Март")
		if idx < 0 {
			t.Fatalf("март отсутствует в сетке: %v", rep.Labels)
		}
		if rep.Expense.Buckets[idx].Total != 0 || rep.Net[idx] != 0 {
			t.Errorf("март: расход %v, net %v — ожидались нули",
				rep.Expense.Buckets[idx].Total, rep.Net[idx])
		}
		if len(rep.Expense.Buckets[idx].Amounts) != len(rep.Expense.Categories) {
			t.Errorf("март: %d сумм на %d категорий",
				len(rep.Expense.Buckets[idx].Amounts), len(rep.Expense.Categories))
		}
	})

	t.Run("переводы между кошельками не попадают в отчёт", func(t *testing.T) {
		rep, err := svc.Report(ctx, model.ReportFilter{UserID: fx.userID})
		if err != nil {
			t.Fatalf("Report: %v", err)
		}
		withTransfers := scalarFromSQL(t,
			`SELECT COALESCE(SUM(amount), 0)::float8
			 FROM operations WHERE user_id = $1 AND type = 'expense'`,
			fx.userID)
		want := scalarFromSQL(t,
			`SELECT COALESCE(SUM(amount), 0)::float8
			 FROM operations
			 WHERE user_id = $1 AND type = 'expense' AND transfer_id IS NULL`,
			fx.userID)
		if approxEqual(withTransfers, want) {
			t.Fatal("фикстура без переводных расходов — проверка бессмысленна")
		}
		if !approxEqual(rep.Expense.Total, want) {
			t.Errorf("Expense.Total = %v, прямой SQL без переводов даёт %v",
				rep.Expense.Total, want)
		}
	})

	t.Run("чужие операции не попадают в отчёт", func(t *testing.T) {
		rep, err := svc.Report(ctx, model.ReportFilter{UserID: fx.userID})
		if err != nil {
			t.Fatalf("Report: %v", err)
		}
		all := scalarFromSQL(t,
			`SELECT COALESCE(SUM(amount), 0)::float8
			 FROM operations WHERE type = 'expense' AND transfer_id IS NULL`)
		want := scalarFromSQL(t,
			`SELECT COALESCE(SUM(amount), 0)::float8
			 FROM operations
			 WHERE user_id = $1 AND type = 'expense' AND transfer_id IS NULL`,
			fx.userID)
		if approxEqual(all, want) {
			t.Fatal("в БД нет чужих операций — проверка бессмысленна")
		}
		if !approxEqual(rep.Expense.Total, want) {
			t.Errorf("Expense.Total = %v, прямой SQL по своему user_id даёт %v",
				rep.Expense.Total, want)
		}
	})

	t.Run("ручная гранулярность перекрывает авто", func(t *testing.T) {
		rep, err := svc.Report(ctx, model.ReportFilter{
			UserID:      fx.userID,
			Granularity: model.GranularityYear,
		})
		if err != nil {
			t.Fatalf("Report: %v", err)
		}
		if rep.Granularity != model.GranularityYear {
			t.Errorf("Granularity = %q, ожидался year", rep.Granularity)
		}
		if len(rep.Labels) != 1 || rep.Labels[0] != "2025" {
			t.Errorf("Labels = %v, ожидался один год", rep.Labels)
		}
	})
}

// TestReportE2E_CollapsesToOther — больше maxReportCategories категорий в выборке
// схлопываются в «Прочее» без потери суммы.
func TestReportE2E_CollapsesToOther(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	fx := newReportFixture(t, "e2e-collapse@test.com")
	svc := service.New(storage.NewFromPool(testPool))

	createOp(t, fx.userID, fx.wallet1, fx.expense, model.TypeExpense, 1000, utcDate(2025, time.March, 1))
	for i := 1; i <= 11; i++ {
		id := createCategory(t, fx.userID, "Категория "+string(rune('A'+i-1)), model.TypeExpense)
		createOp(t, fx.userID, fx.wallet1, id, model.TypeExpense, float64(i), utcDate(2025, time.March, 2))
	}

	rep, err := svc.Report(ctx, model.ReportFilter{UserID: fx.userID})
	if err != nil {
		t.Fatalf("Report: %v", err)
	}
	cats := rep.Expense.Categories
	if len(cats) != 9 {
		t.Fatalf("категорий = %d, ожидались 8 + «Прочее»: %+v", len(cats), cats)
	}
	if cats[len(cats)-1].Name != "Прочее" {
		t.Errorf("последняя категория = %q, ожидалось «Прочее»", cats[len(cats)-1].Name)
	}
	want := scalarFromSQL(t,
		`SELECT COALESCE(SUM(amount), 0)::float8
		 FROM operations WHERE user_id = $1 AND type = 'expense' AND transfer_id IS NULL`,
		fx.userID)
	var total float64
	for _, cat := range cats {
		total += cat.Total
	}
	if !approxEqual(total, want) {
		t.Errorf("сумма категорий после схлопывания = %v, прямой SQL даёт %v", total, want)
	}
	// 12 категорий: в топ-8 не попали четыре самые мелкие — 1 + 2 + 3 + 4
	if !approxEqual(cats[len(cats)-1].Total, 10) {
		t.Errorf("«Прочее» = %v, ожидалось 10", cats[len(cats)-1].Total)
	}
}
