package service

import (
	"context"
	"errors"
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/dbashirov/wimm/internal/model"
	"github.com/dbashirov/wimm/internal/storage/mocks"
	"github.com/google/uuid"
	"go.uber.org/mock/gomock"
)

// ── Хелперы времени и сетки ─────────────────────────────────────────────────

func utc(year int, month time.Month, day int) time.Time {
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}

func TestPickGranularity(t *testing.T) {
	from := utc(2025, time.January, 1)

	tests := []struct {
		name string
		days int
		want model.Granularity
	}{
		{"нулевой период", 0, model.GranularityDay},
		{"31 день", 31, model.GranularityDay},
		{"32 дня", 32, model.GranularityWeek},
		{"92 дня", 92, model.GranularityWeek},
		{"93 дня", 93, model.GranularityMonth},
		{"1826 дней", 1826, model.GranularityMonth},
		{"1827 дней", 1827, model.GranularityYear},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			to := from.Add(time.Duration(tc.days) * 24 * time.Hour)
			if got := pickGranularity(from, to); got != tc.want {
				t.Errorf("pickGranularity(%d дней) = %q, want %q", tc.days, got, tc.want)
			}
		})
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		name string
		in   time.Time
		gran model.Granularity
		want time.Time
	}{
		{
			"день отбрасывает время",
			time.Date(2025, time.September, 18, 23, 59, 59, 0, time.UTC),
			model.GranularityDay, utc(2025, time.September, 18),
		},
		{
			"неделя от понедельника",
			time.Date(2025, time.September, 18, 10, 0, 0, 0, time.UTC),
			model.GranularityWeek, utc(2025, time.September, 15),
		},
		{
			"воскресенье относится к прошлой неделе",
			time.Date(2025, time.January, 5, 12, 0, 0, 0, time.UTC),
			model.GranularityWeek, utc(2024, time.December, 30),
		},
		{
			"понедельник остаётся собой",
			utc(2025, time.January, 6),
			model.GranularityWeek, utc(2025, time.January, 6),
		},
		{
			"месяц",
			time.Date(2025, time.February, 28, 18, 30, 0, 0, time.UTC),
			model.GranularityMonth, utc(2025, time.February, 1),
		},
		{
			"год",
			time.Date(2026, time.December, 31, 23, 0, 0, 0, time.UTC),
			model.GranularityYear, utc(2026, time.January, 1),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := truncate(tc.in, tc.gran); !got.Equal(tc.want) {
				t.Errorf("truncate = %s, want %s", got, tc.want)
			}
		})
	}
}

// TestTruncate_TimezoneIndependent — страховка сшивки Go-сетки с SQL-бакетами:
// date_trunc в БД считает по UTC, значит и Go обязан игнорировать зону входа.
func TestTruncate_TimezoneIndependent(t *testing.T) {
	base := time.Date(2025, time.September, 18, 23, 30, 0, 0, time.UTC)
	zones := []*time.Location{
		time.Local,
		time.FixedZone("MSK", 3*60*60),
		time.FixedZone("NY", -5*60*60),
	}
	grans := []model.Granularity{
		model.GranularityDay,
		model.GranularityWeek,
		model.GranularityMonth,
		model.GranularityYear,
	}

	for _, zone := range zones {
		shifted := base.In(zone)
		for _, gran := range grans {
			want := truncate(base, gran)
			got := truncate(shifted, gran)
			if !got.Equal(want) {
				t.Errorf("zone %s, gran %q: %s, want %s", zone, gran, got, want)
			}
			if got.Location() != time.UTC {
				t.Errorf("zone %s, gran %q: результат не в UTC: %s", zone, gran, got)
			}
		}
	}

	if want := utc(2025, time.September, 18); !truncate(base.In(time.Local), model.GranularityDay).Equal(want) {
		t.Errorf("день из time.Local не совпал с %s", want)
	}
}

func TestBucketStarts(t *testing.T) {
	tests := []struct {
		name string
		from time.Time
		to   time.Time
		gran model.Granularity
		want []time.Time
	}{
		{
			"один месячный бакет",
			utc(2025, time.March, 5), utc(2025, time.March, 20),
			model.GranularityMonth,
			[]time.Time{utc(2025, time.March, 1)},
		},
		{
			"дни через границу месяца",
			utc(2025, time.January, 30), utc(2025, time.February, 2),
			model.GranularityDay,
			[]time.Time{
				utc(2025, time.January, 30),
				utc(2025, time.January, 31),
				utc(2025, time.February, 1),
			},
		},
		{
			"недели через границу года",
			utc(2024, time.December, 28), utc(2025, time.January, 10),
			model.GranularityWeek,
			[]time.Time{
				utc(2024, time.December, 23),
				utc(2024, time.December, 30),
				utc(2025, time.January, 6),
			},
		},
		{
			"месяцы через границу года",
			utc(2024, time.November, 15), utc(2025, time.February, 1),
			model.GranularityMonth,
			[]time.Time{
				utc(2024, time.November, 1),
				utc(2024, time.December, 1),
				utc(2025, time.January, 1),
			},
		},
		{
			"годы",
			utc(2023, time.June, 1), utc(2025, time.March, 1),
			model.GranularityYear,
			[]time.Time{
				utc(2023, time.January, 1),
				utc(2024, time.January, 1),
				utc(2025, time.January, 1),
			},
		},
		{
			"пустой диапазон",
			utc(2025, time.March, 5), utc(2025, time.March, 5),
			model.GranularityDay, nil,
		},
		{
			"перевёрнутый диапазон",
			utc(2025, time.March, 5), utc(2025, time.March, 1),
			model.GranularityDay, nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := bucketStarts(tc.from, tc.to, tc.gran)
			if len(got) != len(tc.want) {
				t.Fatalf("len = %d, want %d: %v", len(got), len(tc.want), got)
			}
			for i, start := range got {
				if !start.Equal(tc.want[i]) {
					t.Errorf("bucket[%d] = %s, want %s", i, start, tc.want[i])
				}
			}
		})
	}
}

// TestBucketStarts_ExclusiveTo — граница DateTo эксклюзивна, как в SQL-фильтре.
func TestBucketStarts_ExclusiveTo(t *testing.T) {
	got := bucketStarts(utc(2025, time.January, 1), utc(2025, time.February, 1), model.GranularityMonth)
	if len(got) != 1 || !got[0].Equal(utc(2025, time.January, 1)) {
		t.Fatalf("got %v, want [2025-01-01]", got)
	}
}

func TestBucketLabel(t *testing.T) {
	tests := []struct {
		name      string
		in        time.Time
		gran      model.Granularity
		multiYear bool
		want      string
	}{
		{"день", utc(2025, time.September, 18), model.GranularityDay, false, "18.09"},
		{"день с годом", utc(2025, time.September, 18), model.GranularityDay, true, "18.09.25"},
		{"день с ведущим нулём", utc(2025, time.March, 7), model.GranularityDay, false, "07.03"},
		{"неделя внутри месяца", utc(2025, time.August, 18), model.GranularityWeek, false, "18–24 авг"},
		{"неделя через месяц", utc(2025, time.July, 28), model.GranularityWeek, false, "28 июл – 3 авг"},
		{
			"неделя через год", utc(2024, time.December, 30),
			model.GranularityWeek, false, "30 дек 24 – 5 янв 25",
		},
		{
			"неделя от середины недели", time.Date(2025, time.August, 20, 15, 0, 0, 0, time.UTC),
			model.GranularityWeek, false, "18–24 авг",
		},
		{"неделя с годом", utc(2025, time.August, 18), model.GranularityWeek, true, "18–24 авг 25"},
		{
			"неделя через месяц с годом", utc(2025, time.July, 28),
			model.GranularityWeek, true, "28 июл – 3 авг 25",
		},
		{"месяц", utc(2025, time.September, 1), model.GranularityMonth, false, "Сентябрь"},
		{"месяц с годом", utc(2025, time.September, 1), model.GranularityMonth, true, "Сен 2025"},
		{"май с годом", utc(2024, time.May, 14), model.GranularityMonth, true, "Май 2024"},
		{"год", utc(2026, time.May, 1), model.GranularityYear, false, "2026"},
		{"год при multiYear", utc(2026, time.May, 1), model.GranularityYear, true, "2026"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := bucketLabel(tc.in, tc.gran, tc.multiYear); got != tc.want {
				t.Errorf("bucketLabel = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestBucketLabel_WeeksUniqueAcrossYears — на многолетней недельной шкале
// (granularity=week задаётся вручную) подписи не должны повторяться.
func TestBucketLabel_WeeksUniqueAcrossYears(t *testing.T) {
	// 6 лет — 314 недель, это влезает в maxReportBuckets; календарь за такой
	// срок повторяется, и без года «4–10 мар» 2019-го совпало бы с 2024-м
	grid := bucketStarts(utc(2019, time.January, 1), utc(2025, time.January, 1), model.GranularityWeek)
	if len(grid) < 300 {
		t.Fatalf("сетка из %d недель — мало для проверки", len(grid))
	}

	seen := make(map[string]time.Time, len(grid))
	for _, start := range grid {
		label := bucketLabel(start, model.GranularityWeek, true)
		if prev, dup := seen[label]; dup {
			t.Fatalf("подпись %q повторяется: %s и %s", label, prev.Format(time.DateOnly),
				start.Format(time.DateOnly))
		}
		seen[label] = start
	}
}

// ── collapseCategories ──────────────────────────────────────────────────────

func reportCatID(n int) uuid.UUID {
	return uuid.MustParse(fmt.Sprintf("40000000-0000-0000-0000-%012d", n))
}

// reportRows раскладывает суммы категорий по двум бакетам, чтобы на вход
// collapseCategories попадали сырые строки, а не готовые итоги.
func reportRows(totals ...model.Money) []model.ReportRow {
	rows := make([]model.ReportRow, 0, len(totals)*2)
	for i, total := range totals {
		id := reportCatID(i + 1)
		name := fmt.Sprintf("Категория %02d", i+1)
		rows = append(rows,
			model.ReportRow{
				Bucket:     utc(2025, time.January, 1),
				CategoryID: id, Name: name,
				Type: model.TypeExpense, Amount: total / 2,
			},
			model.ReportRow{
				Bucket:     utc(2025, time.February, 1),
				CategoryID: id, Name: name,
				Type: model.TypeExpense, Amount: total - total/2,
			},
		)
	}
	return rows
}

func TestCollapseCategories(t *testing.T) {
	tests := []struct {
		name      string
		totals    []model.Money
		limit     int
		wantNames []string
		wantSums  []model.Money
	}{
		{
			"меньше лимита",
			[]model.Money{100, 300, 200}, 8,
			[]string{"Категория 02", "Категория 03", "Категория 01"},
			[]model.Money{300, 200, 100},
		},
		{
			"ровно лимит",
			[]model.Money{8, 7, 6, 5, 4, 3, 2, 1}, 8,
			[]string{
				"Категория 01", "Категория 02", "Категория 03", "Категория 04",
				"Категория 05", "Категория 06", "Категория 07", "Категория 08",
			},
			[]model.Money{8, 7, 6, 5, 4, 3, 2, 1},
		},
		{
			"больше лимита — хвост в «Прочее»",
			[]model.Money{10, 9, 8, 7, 6, 5, 4, 3, 2, 1}, 8,
			[]string{
				"Категория 01", "Категория 02", "Категория 03", "Категория 04",
				"Категория 05", "Категория 06", "Категория 07", "Категория 08",
				otherCategoryName,
			},
			[]model.Money{10, 9, 8, 7, 6, 5, 4, 3, 3},
		},
		{
			"лимит 1",
			[]model.Money{5, 3, 2}, 1,
			[]string{"Категория 01", otherCategoryName},
			[]model.Money{5, 5},
		},
		{
			"пустой вход",
			nil, 8,
			nil, nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cats, index := collapseCategories(reportRows(tc.totals...), tc.limit)
			if len(cats) != len(tc.wantNames) {
				t.Fatalf("len(cats) = %d, want %d: %+v", len(cats), len(tc.wantNames), cats)
			}
			for i, cat := range cats {
				if cat.Name != tc.wantNames[i] {
					t.Errorf("cats[%d].Name = %q, want %q", i, cat.Name, tc.wantNames[i])
				}
				if cat.Total != tc.wantSums[i] {
					t.Errorf("cats[%d].Total = %v, want %v", i, cat.Total, tc.wantSums[i])
				}
			}
			// каждая исходная категория обязана иметь колонку в секции
			for i := range tc.totals {
				if _, ok := index[reportCatID(i+1)]; !ok {
					t.Errorf("категория %d не попала в индекс", i+1)
				}
			}
		})
	}
}

// TestCollapseCategories_OrderDesc — порядок строго по убыванию суммы,
// «Прочее» всегда последнее, даже когда его сумма больше топовой категории.
func TestCollapseCategories_OrderDesc(t *testing.T) {
	cats, _ := collapseCategories(reportRows(9, 8, 7, 6, 5, 4, 3, 2, 20, 20), 8)
	for i := 1; i < len(cats)-1; i++ {
		if cats[i-1].Total < cats[i].Total {
			t.Errorf("порядок нарушен на %d: %v < %v", i, cats[i-1].Total, cats[i].Total)
		}
	}
	last := cats[len(cats)-1]
	if last.Name != otherCategoryName || last.ID != uuid.Nil {
		t.Fatalf("последняя категория = %+v, want «Прочее» с uuid.Nil", last)
	}
	// в топ-8 попали 20, 20, 9, 8, 7, 6, 5, 4 — схлопнулись 3 и 2
	if last.Total != 5 {
		t.Errorf("«Прочее».Total = %v, want 5", last.Total)
	}
}

// TestCollapseCategories_OtherSum — сумма «Прочего» равна сумме схлопнутых
// категорий, а общий итог не меняется от схлопывания.
func TestCollapseCategories_OtherSum(t *testing.T) {
	totals := []model.Money{100, 90, 80, 70, 60, 50, 40, 30, 20, 10, 5}
	rows := reportRows(totals...)

	full, _ := collapseCategories(rows, 0)
	collapsed, index := collapseCategories(rows, 8)

	var wantOther model.Money
	for _, total := range totals[8:] {
		wantOther += total
	}
	other := collapsed[len(collapsed)-1]
	if other.Total != wantOther {
		t.Errorf("«Прочее».Total = %v, want %v", other.Total, wantOther)
	}

	sumOf := func(cats []model.ReportCategory) model.Money {
		var sum model.Money
		for _, cat := range cats {
			sum += cat.Total
		}
		return sum
	}
	if sumOf(full) != sumOf(collapsed) {
		t.Errorf("итог после схлопывания = %v, want %v", sumOf(collapsed), sumOf(full))
	}
	if len(full) != len(totals) {
		t.Errorf("limit 0 схлопнул категории: %d, want %d", len(full), len(totals))
	}

	otherIdx := len(collapsed) - 1
	for i := 8; i < len(totals); i++ {
		if got := index[reportCatID(i+1)]; got != otherIdx {
			t.Errorf("категория %d ведёт в колонку %d, want %d", i+1, got, otherIdx)
		}
	}
}

// TestCollapseCategories_Share — доли считаются от итога секции и не дают NaN
// при нулевых суммах.
func TestCollapseCategories_Share(t *testing.T) {
	cats, _ := collapseCategories(reportRows(75, 25), 8)
	if cats[0].Share != 75 || cats[1].Share != 25 {
		t.Errorf("доли = %v / %v, want 75 / 25", cats[0].Share, cats[1].Share)
	}

	zero, _ := collapseCategories(reportRows(0, 0), 8)
	for i, cat := range zero {
		if cat.Share != 0 {
			t.Errorf("zero[%d].Share = %v, want 0", i, cat.Share)
		}
	}
}

// ── Подбор шага и размер сетки ──────────────────────────────────────────────

func TestCoarser(t *testing.T) {
	tests := []struct {
		in     model.Granularity
		want   model.Granularity
		wantOK bool
	}{
		{model.GranularityDay, model.GranularityWeek, true},
		{model.GranularityWeek, model.GranularityMonth, true},
		{model.GranularityMonth, model.GranularityYear, true},
		{model.GranularityYear, model.GranularityYear, false},
	}
	for _, tc := range tests {
		t.Run(string(tc.in), func(t *testing.T) {
			got, ok := coarser(tc.in)
			if got != tc.want || ok != tc.wantOK {
				t.Errorf("coarser(%q) = %q, %v; want %q, %v", tc.in, got, ok, tc.want, tc.wantOK)
			}
		})
	}
}

// TestBucketCount_MatchesGridProperty — счётчик обязан совпадать с длиной
// сетки на любой границе, включая «начало бакета + наносекунда»: ровно такую
// границу строит reportRange для операции, записанной без времени.
func TestBucketCount_MatchesGridProperty(t *testing.T) {
	grans := []model.Granularity{
		model.GranularityDay, model.GranularityWeek,
		model.GranularityMonth, model.GranularityYear,
	}
	froms := []time.Time{
		utc(2025, time.January, 1),
		utc(2025, time.March, 5),
		utc(2024, time.February, 29),
		utc(2023, time.December, 31),
		utc(2025, time.March, 1).Add(13 * time.Hour),
	}
	offsets := []time.Duration{
		0, time.Nanosecond, -time.Nanosecond, time.Hour, 12 * time.Hour,
	}
	for _, g := range grans {
		for _, from := range froms {
			for steps := 0; steps <= 40; steps++ {
				base := shiftBuckets(truncate(from, g), g, steps)
				for _, off := range offsets {
					to := base.Add(off)
					want := len(bucketStarts(from, to, g))
					if got := bucketCount(from, to, g); got != want {
						t.Fatalf(
							"g=%s from=%v to=%v: bucketCount=%d, len(bucketStarts)=%d",
							g, from, to, got, want,
						)
					}
				}
			}
		}
	}
}

// TestReportGrid_Boundary — ровно maxReportBuckets шаг не меняет,
// а один лишний бакет уже укрупняет.
func TestReportGrid_Boundary(t *testing.T) {
	from := utc(2025, time.January, 1)

	grid, g, notice := reportGrid(from, from.AddDate(0, 0, maxReportBuckets), model.GranularityDay)
	if g != model.GranularityDay || len(grid) != maxReportBuckets || notice != "" {
		t.Errorf("%d бакетов: got %q, %d, notice %q", maxReportBuckets, g, len(grid), notice)
	}

	grid, g, notice = reportGrid(from, from.AddDate(0, 0, maxReportBuckets+1), model.GranularityDay)
	if g != model.GranularityWeek {
		t.Errorf("%d бакетов: got %q, want week", maxReportBuckets+1, g)
	}
	if len(grid) > maxReportBuckets || notice == "" {
		t.Errorf("грид %d, notice %q", len(grid), notice)
	}
}

// TestReportGrid_DoubleCoarsening — 40 лет с дневным шагом укрупняются
// через неделю и месяц до года.
func TestReportGrid_DoubleCoarsening(t *testing.T) {
	from, to := utc(1985, time.January, 1), utc(2025, time.January, 1)
	grid, g, notice := reportGrid(from, to, model.GranularityDay)
	if g != model.GranularityYear {
		t.Errorf("granularity = %q, want year", g)
	}
	if len(grid) != 40 {
		t.Errorf("len(grid) = %d, want 40", len(grid))
	}
	if notice == "" {
		t.Error("notice пуст — пользователь не узнает об укрупнении")
	}
}

// TestReportGrid_HugeRangeAtYear — на year укрупнять некуда, поэтому сетка
// обрезается: инвариант len(grid) <= maxReportBuckets держится безусловно.
func TestReportGrid_HugeRangeAtYear(t *testing.T) {
	from, to := utc(2, time.January, 1), utc(9999, time.December, 31)
	grid, g, notice := reportGrid(from, to, model.GranularityYear)
	if g != model.GranularityYear {
		t.Errorf("granularity = %q, want year", g)
	}
	if len(grid) != maxReportBuckets {
		t.Fatalf("len(grid) = %d, want %d", len(grid), maxReportBuckets)
	}
	if notice == "" {
		t.Error("notice пуст — обрезанный диапазон надо объяснить")
	}
	if last := grid[len(grid)-1]; last.Year() != 9999 {
		t.Errorf("последний бакет = %v, ожидался 9999 год", last)
	}
}

// ── Service.Report ──────────────────────────────────────────────────────────

func newReportSvc(t *testing.T) (*Service, *mocks.MockOperationRepo) {
	t.Helper()
	ops := mocks.NewMockOperationRepo(gomock.NewController(t))
	return NewWithRepos(nil, nil, nil, ops), ops
}

func reportRow(
	bucket time.Time,
	id uuid.UUID,
	name string,
	opType model.OperationType,
	amount model.Money,
) model.ReportRow {
	return model.ReportRow{
		Bucket:     bucket,
		CategoryID: id,
		Name:       name,
		Type:       opType,
		Amount:     amount,
	}
}

func approxEqual[T model.Money | float64](got, want T) bool {
	return math.Abs(float64(got-want)) < 1e-9
}

func TestReport_Sections(t *testing.T) {
	svc, ops := newReportSvc(t)

	filter := model.ReportFilter{
		UserID:      uuid.Must(uuid.NewV7()),
		DateFrom:    utc(2025, time.January, 1),
		DateTo:      utc(2025, time.April, 1),
		Granularity: model.GranularityMonth,
	}
	bounds := model.ReportBounds{
		From:  utc(2025, time.January, 5),
		To:    utc(2025, time.March, 20),
		Count: 4,
	}
	jan, mar := utc(2025, time.January, 1), utc(2025, time.March, 1)
	food, taxi, salary := reportCatID(1), reportCatID(2), reportCatID(3)
	rows := []model.ReportRow{
		reportRow(jan, food, "Еда", model.TypeExpense, 100),
		reportRow(jan, taxi, "Такси", model.TypeExpense, 50),
		reportRow(mar, food, "Еда", model.TypeExpense, 30),
		reportRow(mar, salary, "Зарплата", model.TypeIncome, 500),
	}

	ops.EXPECT().ReportBounds(gomock.Any(), filter).Return(bounds, nil)
	ops.EXPECT().
		ReportRows(gomock.Any(), filter, model.GranularityMonth).
		Return(rows, nil)

	rep, err := svc.Report(context.Background(), filter)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rep.Granularity != model.GranularityMonth {
		t.Errorf("granularity: got %q, want month", rep.Granularity)
	}
	if rep.Notice != "" {
		t.Errorf("notice: got %q, want empty", rep.Notice)
	}
	wantLabels := []string{"Январь", "Февраль", "Март"}
	if len(rep.Labels) != len(wantLabels) {
		t.Fatalf("labels: got %v, want %v", rep.Labels, wantLabels)
	}
	for i, label := range wantLabels {
		if rep.Labels[i] != label {
			t.Errorf("label %d: got %q, want %q", i, rep.Labels[i], label)
		}
	}

	exp := rep.Expense
	if exp == nil {
		t.Fatal("expense section is nil")
	}
	if len(exp.Categories) != 2 {
		t.Fatalf("categories: got %d, want 2", len(exp.Categories))
	}
	if exp.Categories[0].Name != "Еда" || !approxEqual(exp.Categories[0].Total, 130) {
		t.Errorf("top category: got %+v", exp.Categories[0])
	}
	if !approxEqual(exp.Categories[0].Share, 130.0/180*100) {
		t.Errorf("share: got %v, want %v", exp.Categories[0].Share, 130.0/180*100)
	}
	if !approxEqual(exp.Total, 180) || !approxEqual(exp.Average, 60) {
		t.Errorf("expense totals: total %v, average %v", exp.Total, exp.Average)
	}
	if len(exp.Buckets) != 3 {
		t.Fatalf("buckets: got %d, want 3", len(exp.Buckets))
	}
	empty := exp.Buckets[1]
	if !approxEqual(empty.Total, 0) || len(empty.Amounts) != 2 {
		t.Fatalf("empty bucket: %+v", empty)
	}
	for i, amount := range empty.Amounts {
		if !approxEqual(amount, 0) {
			t.Errorf("empty bucket amount %d: got %v, want 0", i, amount)
		}
	}
	if !approxEqual(exp.Buckets[0].Amounts[0], 100) || !approxEqual(exp.Buckets[0].Amounts[1], 50) {
		t.Errorf("january amounts: %v", exp.Buckets[0].Amounts)
	}

	inc := rep.Income
	if inc == nil {
		t.Fatal("income section is nil")
	}
	if !approxEqual(inc.Total, 500) || !approxEqual(inc.Buckets[0].Total, 0) {
		t.Errorf("income: total %v, january %v", inc.Total, inc.Buckets[0].Total)
	}

	wantNet := []model.Money{-150, 0, 470}
	for i, want := range wantNet {
		if !approxEqual(rep.Net[i], want) {
			t.Errorf("net %d: got %v, want %v", i, rep.Net[i], want)
		}
	}
	if !approxEqual(rep.TotalNet, 320) {
		t.Errorf("total net: got %v, want 320", rep.TotalNet)
	}
}

func TestReport_ManualGranularityOverridesAuto(t *testing.T) {
	svc, ops := newReportSvc(t)

	filter := model.ReportFilter{
		UserID:      uuid.Must(uuid.NewV7()),
		DateFrom:    utc(2025, time.January, 1),
		DateTo:      utc(2025, time.January, 31),
		Granularity: model.GranularityMonth,
	}
	bounds := model.ReportBounds{
		From:  utc(2025, time.January, 2),
		To:    utc(2025, time.January, 30),
		Count: 1,
	}
	row := reportRow(utc(2025, time.January, 1), reportCatID(1), "Еда", model.TypeExpense, 10)

	ops.EXPECT().ReportBounds(gomock.Any(), filter).Return(bounds, nil)
	ops.EXPECT().
		ReportRows(gomock.Any(), filter, model.GranularityMonth).
		Return([]model.ReportRow{row}, nil)

	rep, err := svc.Report(context.Background(), filter)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rep.Granularity != model.GranularityMonth {
		t.Errorf("granularity: got %q, want month", rep.Granularity)
	}
	if len(rep.Expense.Buckets) != 1 {
		t.Errorf("buckets: got %d, want 1", len(rep.Expense.Buckets))
	}
}

func TestReport_TooManyBucketsCoarsened(t *testing.T) {
	svc, ops := newReportSvc(t)

	filter := model.ReportFilter{
		UserID:      uuid.Must(uuid.NewV7()),
		DateFrom:    utc(2020, time.January, 1),
		DateTo:      utc(2025, time.January, 1),
		Granularity: model.GranularityDay,
	}
	bounds := model.ReportBounds{
		From:  utc(2020, time.February, 1),
		To:    utc(2024, time.December, 1),
		Count: 10,
	}

	// понедельники внутри bounds: сетка недель строится от понедельника
	week1, week2 := utc(2020, time.February, 3), utc(2023, time.June, 5)
	food := reportCatID(1)
	rows := []model.ReportRow{
		reportRow(week1, food, "Еда", model.TypeExpense, 100),
		reportRow(week2, food, "Еда", model.TypeExpense, 40),
	}

	ops.EXPECT().ReportBounds(gomock.Any(), filter).Return(bounds, nil)
	ops.EXPECT().
		ReportRows(gomock.Any(), filter, model.GranularityWeek).
		Return(rows, nil)

	rep, err := svc.Report(context.Background(), filter)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rep.Granularity != model.GranularityWeek {
		t.Errorf("granularity: got %q, want week", rep.Granularity)
	}
	if rep.Notice == "" {
		t.Error("notice: got empty, want message about coarser step")
	}
	if len(rep.Labels) > maxReportBuckets {
		t.Errorf("labels: got %d, want <= %d", len(rep.Labels), maxReportBuckets)
	}
	if rep.Expense == nil {
		t.Fatal("expense section is nil — строки не легли на укрупнённую сетку")
	}
	if !approxEqual(rep.Expense.Total, 140) {
		t.Errorf("expense total: got %v, want 140", rep.Expense.Total)
	}
	var byBuckets model.Money
	for _, bucket := range rep.Expense.Buckets {
		byBuckets += bucket.Total
	}
	if !approxEqual(byBuckets, 140) {
		t.Errorf("сумма по бакетам: got %v, want 140", byBuckets)
	}
}

func TestReport_EmptySelection(t *testing.T) {
	svc, ops := newReportSvc(t)

	filter := model.ReportFilter{UserID: uuid.Must(uuid.NewV7())}
	ops.EXPECT().
		ReportBounds(gomock.Any(), filter).
		Return(model.ReportBounds{}, nil)

	rep, err := svc.Report(context.Background(), filter)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rep.Expense != nil || rep.Income != nil {
		t.Errorf("sections: got %+v / %+v, want nil", rep.Expense, rep.Income)
	}
	if len(rep.Labels) != 0 || len(rep.Net) != 0 {
		t.Errorf("labels %v, net %v — want empty", rep.Labels, rep.Net)
	}
	if rep.TotalNet != 0 {
		t.Errorf("total net: got %v, want 0", rep.TotalNet)
	}
	if !validGranularity(rep.Granularity) {
		t.Errorf("granularity: got %q, want one of the known steps", rep.Granularity)
	}
}

func TestReport_SingleTypeLeavesOtherSectionNil(t *testing.T) {
	svc, ops := newReportSvc(t)

	filter := model.ReportFilter{
		UserID:      uuid.Must(uuid.NewV7()),
		Type:        model.TypeExpense,
		DateFrom:    utc(2025, time.January, 1),
		DateTo:      utc(2025, time.February, 1),
		Granularity: model.GranularityMonth,
	}
	bounds := model.ReportBounds{
		From:  utc(2025, time.January, 3),
		To:    utc(2025, time.January, 25),
		Count: 1,
	}
	row := reportRow(utc(2025, time.January, 1), reportCatID(1), "Еда", model.TypeExpense, 40)

	ops.EXPECT().ReportBounds(gomock.Any(), filter).Return(bounds, nil)
	ops.EXPECT().
		ReportRows(gomock.Any(), filter, model.GranularityMonth).
		Return([]model.ReportRow{row}, nil)

	rep, err := svc.Report(context.Background(), filter)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rep.Income != nil {
		t.Errorf("income section: got %+v, want nil", rep.Income)
	}
	if rep.Expense == nil || !approxEqual(rep.Expense.Total, 40) {
		t.Fatalf("expense section: %+v", rep.Expense)
	}
	if !approxEqual(rep.Net[0], -40) {
		t.Errorf("net: got %v, want -40", rep.Net[0])
	}
}

func TestReport_AutoRangeFromBounds(t *testing.T) {
	svc, ops := newReportSvc(t)

	filter := model.ReportFilter{UserID: uuid.Must(uuid.NewV7())}
	bounds := model.ReportBounds{
		From:  utc(2025, time.January, 10),
		To:    utc(2025, time.January, 20),
		Count: 2,
	}
	rows := []model.ReportRow{
		reportRow(utc(2025, time.January, 10), reportCatID(1), "Еда", model.TypeExpense, 10),
		reportRow(utc(2025, time.January, 20), reportCatID(1), "Еда", model.TypeExpense, 20),
	}

	ops.EXPECT().ReportBounds(gomock.Any(), filter).Return(bounds, nil)
	ops.EXPECT().
		ReportRows(gomock.Any(), filter, model.GranularityDay).
		Return(rows, nil)

	rep, err := svc.Report(context.Background(), filter)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rep.Granularity != model.GranularityDay {
		t.Errorf("granularity: got %q, want day", rep.Granularity)
	}
	if len(rep.Labels) != 11 {
		t.Fatalf("labels: got %d (%v), want 11", len(rep.Labels), rep.Labels)
	}
	if rep.Labels[10] != "20.01" {
		t.Errorf("last label: got %q, want 20.01", rep.Labels[10])
	}
	last := rep.Expense.Buckets[10]
	if !approxEqual(last.Total, 20) {
		t.Errorf("last bucket: got %v, want 20", last.Total)
	}
}

func TestReport_ZeroAmountsNoNaN(t *testing.T) {
	svc, ops := newReportSvc(t)

	filter := model.ReportFilter{
		UserID:      uuid.Must(uuid.NewV7()),
		DateFrom:    utc(2025, time.January, 1),
		DateTo:      utc(2025, time.February, 1),
		Granularity: model.GranularityMonth,
	}
	bounds := model.ReportBounds{
		From:  utc(2025, time.January, 5),
		To:    utc(2025, time.January, 6),
		Count: 1,
	}
	row := reportRow(utc(2025, time.January, 1), reportCatID(1), "Еда", model.TypeExpense, 0)

	ops.EXPECT().ReportBounds(gomock.Any(), filter).Return(bounds, nil)
	ops.EXPECT().
		ReportRows(gomock.Any(), filter, model.GranularityMonth).
		Return([]model.ReportRow{row}, nil)

	rep, err := svc.Report(context.Background(), filter)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	exp := rep.Expense
	if exp == nil {
		t.Fatal("expense section is nil")
	}
	if math.IsNaN(exp.Categories[0].Share) || exp.Categories[0].Share != 0 {
		t.Errorf("share: got %v, want 0", exp.Categories[0].Share)
	}
	if exp.Average != 0 {
		t.Errorf("average: got %v, want 0", exp.Average)
	}
}

// TestReport_HugeRangeStaysBounded — сочинённый вручную URL с диапазоном
// в тысячи лет не разворачивает сетку в миллионы элементов и при этом
// показывает данные: диапазон сужается до фактических границ.
func TestReport_HugeRangeStaysBounded(t *testing.T) {
	svc, ops := newReportSvc(t)

	filter := model.ReportFilter{
		UserID:      uuid.Must(uuid.NewV7()),
		DateFrom:    utc(2, time.January, 1),
		DateTo:      utc(9999, time.December, 31),
		Granularity: model.GranularityDay,
	}
	bounds := model.ReportBounds{From: utc(2025, time.March, 1), To: utc(2025, time.March, 2), Count: 1}

	ops.EXPECT().ReportBounds(gomock.Any(), filter).Return(bounds, nil)
	ops.EXPECT().
		ReportRows(gomock.Any(), filter, model.GranularityDay).
		Return([]model.ReportRow{
			reportRow(utc(2025, time.March, 1), reportCatID(1), "Еда", model.TypeExpense, 10),
		}, nil)

	rep, err := svc.Report(context.Background(), filter)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rep.Labels) != 2 {
		t.Fatalf("len(Labels) = %d, want 2 — сетка сужается до данных", len(rep.Labels))
	}
	if rep.Granularity != model.GranularityDay {
		t.Errorf("granularity = %q, want day — шаг задан явно", rep.Granularity)
	}
	if rep.Expense == nil || !approxEqual(rep.Expense.Total, 10) {
		t.Fatalf("expense section: %+v, want total 10", rep.Expense)
	}
}

// TestReport_ExplicitCategoriesNotCollapsed — пользователь отметил галочками
// 11 категорий, значит все 11 и должны быть в отчёте, без «Прочего».
func TestReport_ExplicitCategoriesNotCollapsed(t *testing.T) {
	svc, ops := newReportSvc(t)

	const count = maxReportCategories + 3
	ids := make([]uuid.UUID, 0, count)
	rows := make([]model.ReportRow, 0, count)
	bucket := utc(2025, time.March, 1)
	for i := 1; i <= count; i++ {
		id := reportCatID(i)
		ids = append(ids, id)
		rows = append(rows, reportRow(bucket, id, fmt.Sprintf("Кат %d", i), model.TypeExpense, model.Money(i)))
	}

	filter := model.ReportFilter{
		UserID:      uuid.Must(uuid.NewV7()),
		CategoryIDs: ids,
		DateFrom:    utc(2025, time.March, 1),
		DateTo:      utc(2025, time.April, 1),
	}
	bounds := model.ReportBounds{From: bucket, To: bucket, Count: count}

	ops.EXPECT().ReportBounds(gomock.Any(), filter).Return(bounds, nil)
	ops.EXPECT().ReportRows(gomock.Any(), filter, gomock.Any()).Return(rows, nil)

	rep, err := svc.Report(context.Background(), filter)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rep.Expense == nil {
		t.Fatal("expense section is nil")
	}
	if len(rep.Expense.Categories) != count {
		t.Fatalf("категорий = %d, want %d", len(rep.Expense.Categories), count)
	}
	for _, cat := range rep.Expense.Categories {
		if cat.Name == otherCategoryName {
			t.Fatalf("явно выбранные категории схлопнуты в %q", otherCategoryName)
		}
	}
}

// TestReport_ExplicitCategoriesCeiling — «Выбрать все» отправляет десятки
// категорий: до потолка показываем все, за потолком хвост уходит в «Прочее».
func TestReport_ExplicitCategoriesCeiling(t *testing.T) {
	tests := []struct {
		name     string
		selected int
		wantCats int
		wantMore bool
	}{
		{"ровно потолок", maxSelectedCategories, maxSelectedCategories, false},
		{"потолок + 1", maxSelectedCategories + 1, maxSelectedCategories + 1, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc, ops := newReportSvc(t)

			ids := make([]uuid.UUID, 0, tc.selected)
			rows := make([]model.ReportRow, 0, tc.selected)
			bucket := utc(2025, time.March, 1)
			for i := 1; i <= tc.selected; i++ {
				id := reportCatID(i)
				ids = append(ids, id)
				rows = append(rows,
					reportRow(bucket, id, fmt.Sprintf("Кат %02d", i), model.TypeExpense, model.Money(i)))
			}

			filter := model.ReportFilter{
				UserID:      uuid.Must(uuid.NewV7()),
				CategoryIDs: ids,
				DateFrom:    utc(2025, time.March, 1),
				DateTo:      utc(2025, time.April, 1),
			}
			bounds := model.ReportBounds{From: bucket, To: bucket, Count: tc.selected}

			ops.EXPECT().ReportBounds(gomock.Any(), filter).Return(bounds, nil)
			ops.EXPECT().ReportRows(gomock.Any(), filter, gomock.Any()).Return(rows, nil)

			rep, err := svc.Report(context.Background(), filter)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if rep.Expense == nil {
				t.Fatal("expense section is nil")
			}
			cats := rep.Expense.Categories
			if len(cats) != tc.wantCats {
				t.Fatalf("категорий = %d, want %d", len(cats), tc.wantCats)
			}
			hasOther := cats[len(cats)-1].Name == otherCategoryName
			if hasOther != tc.wantMore {
				t.Fatalf("«Прочее» = %v, want %v", hasOther, tc.wantMore)
			}
			if !approxEqual(rep.Expense.Total, model.Money(tc.selected*(tc.selected+1)/2)) {
				t.Errorf("итог секции = %v — схлопывание не должно терять суммы", rep.Expense.Total)
			}
		})
	}
}

// TestReport_NoFilterStillCollapses — без явного фильтра защита от радуги
// в легенде остаётся.
func TestReport_NoFilterStillCollapses(t *testing.T) {
	svc, ops := newReportSvc(t)

	const count = maxReportCategories + 3
	rows := make([]model.ReportRow, 0, count)
	bucket := utc(2025, time.March, 1)
	for i := 1; i <= count; i++ {
		rows = append(rows,
			reportRow(bucket, reportCatID(i), fmt.Sprintf("Кат %d", i), model.TypeExpense, model.Money(i)))
	}

	filter := model.ReportFilter{
		UserID:   uuid.Must(uuid.NewV7()),
		DateFrom: utc(2025, time.March, 1),
		DateTo:   utc(2025, time.April, 1),
	}
	bounds := model.ReportBounds{From: bucket, To: bucket, Count: count}

	ops.EXPECT().ReportBounds(gomock.Any(), filter).Return(bounds, nil)
	ops.EXPECT().ReportRows(gomock.Any(), filter, gomock.Any()).Return(rows, nil)

	rep, err := svc.Report(context.Background(), filter)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	cats := rep.Expense.Categories
	if len(cats) != maxReportCategories+1 || cats[len(cats)-1].Name != otherCategoryName {
		t.Fatalf("категории = %+v, ожидались %d + «Прочее»", cats, maxReportCategories)
	}
}

// TestReport_RowOutsideGridIgnored — строка вне сетки не должна ломать
// раскладку: диапазон мог быть обрезан в reportGrid.
func TestReport_RowOutsideGridIgnored(t *testing.T) {
	svc, ops := newReportSvc(t)

	filter := model.ReportFilter{
		UserID:      uuid.Must(uuid.NewV7()),
		DateFrom:    utc(2025, time.March, 1),
		DateTo:      utc(2025, time.April, 1),
		Granularity: model.GranularityMonth,
	}
	bounds := model.ReportBounds{From: utc(2025, time.March, 5), To: utc(2025, time.March, 20), Count: 2}
	food := reportCatID(1)
	rows := []model.ReportRow{
		reportRow(utc(2025, time.March, 1), food, "Еда", model.TypeExpense, 100),
		reportRow(utc(2030, time.March, 1), food, "Еда", model.TypeExpense, 999),
	}

	ops.EXPECT().ReportBounds(gomock.Any(), filter).Return(bounds, nil)
	ops.EXPECT().ReportRows(gomock.Any(), filter, model.GranularityMonth).Return(rows, nil)

	rep, err := svc.Report(context.Background(), filter)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !approxEqual(rep.Expense.Total, 100) {
		t.Errorf("Expense.Total = %v, want 100 — строка вне сетки учтена", rep.Expense.Total)
	}
}

func TestReport_BoundsError(t *testing.T) {
	svc, ops := newReportSvc(t)

	wantErr := errors.New("connection refused")
	ops.EXPECT().ReportBounds(gomock.Any(), gomock.Any()).
		Return(model.ReportBounds{}, wantErr)

	rep, err := svc.Report(context.Background(), model.ReportFilter{})
	if !errors.Is(err, wantErr) {
		t.Errorf("err = %v, want %v", err, wantErr)
	}
	if rep != nil {
		t.Errorf("report = %+v, want nil", rep)
	}
}

func TestReport_RowsError(t *testing.T) {
	svc, ops := newReportSvc(t)

	filter := model.ReportFilter{
		UserID:   uuid.Must(uuid.NewV7()),
		DateFrom: utc(2025, time.March, 1),
		DateTo:   utc(2025, time.April, 1),
	}
	wantErr := errors.New("query failed")
	ops.EXPECT().ReportBounds(gomock.Any(), filter).
		Return(model.ReportBounds{From: utc(2025, time.March, 5), To: utc(2025, time.March, 6), Count: 1}, nil)
	ops.EXPECT().ReportRows(gomock.Any(), filter, gomock.Any()).Return(nil, wantErr)

	rep, err := svc.Report(context.Background(), filter)
	if !errors.Is(err, wantErr) {
		t.Errorf("err = %v, want %v — ошибка агрегации не должна давать отчёт из нулей", err, wantErr)
	}
	if rep != nil {
		t.Errorf("report = %+v, want nil", rep)
	}
}

// TestReport_TypoInDateTo — опечатка в «Дата до» (2999 вместо 2029) не должна
// давать страницу без графиков и без пустого состояния: диапазон сужается
// до фактических границ данных.
func TestReport_TypoInDateTo(t *testing.T) {
	svc, ops := newReportSvc(t)

	filter := model.ReportFilter{
		UserID:   uuid.Must(uuid.NewV7()),
		DateFrom: utc(2024, time.January, 1),
		DateTo:   utc(2999, time.December, 31),
	}
	bounds := model.ReportBounds{
		From:  utc(2024, time.February, 10),
		To:    utc(2024, time.November, 20),
		Count: 3,
	}
	food := reportCatID(1)
	rows := []model.ReportRow{
		reportRow(utc(2024, time.February, 1), food, "Еда", model.TypeExpense, 100),
		reportRow(utc(2024, time.November, 1), food, "Еда", model.TypeExpense, 50),
	}

	ops.EXPECT().ReportBounds(gomock.Any(), filter).Return(bounds, nil)
	ops.EXPECT().
		ReportRows(gomock.Any(), filter, model.GranularityMonth).
		Return(rows, nil)

	rep, err := svc.Report(context.Background(), filter)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rep.Labels) == 0 || len(rep.Labels) > maxReportBuckets {
		t.Fatalf("len(Labels) = %d, want 1..%d", len(rep.Labels), maxReportBuckets)
	}
	if rep.Expense == nil {
		t.Fatal("expense section nil при непустых Labels — страница без графиков и без пустого состояния")
	}
	if !approxEqual(rep.Expense.Total, 150) {
		t.Errorf("Expense.Total = %v, want 150", rep.Expense.Total)
	}
	if len(rep.Labels) != 10 {
		t.Errorf("len(Labels) = %d, want 10 — февраль..ноябрь 2024", len(rep.Labels))
	}
}

// TestReport_CategoryTotalsMatchBuckets — итог категории обязан совпадать
// с суммой её колонки по бакетам: иначе пончик и tfoot разойдутся с таблицей.
func TestReport_CategoryTotalsMatchBuckets(t *testing.T) {
	svc, ops := newReportSvc(t)

	filter := model.ReportFilter{
		UserID:      uuid.Must(uuid.NewV7()),
		DateFrom:    utc(2, time.January, 1),
		DateTo:      utc(9999, time.December, 31),
		Granularity: model.GranularityYear,
	}
	// данные сами растянуты на тысячи лет: сетка будет обрезана в reportGrid
	bounds := model.ReportBounds{
		From:  utc(2, time.January, 1),
		To:    utc(9999, time.December, 31),
		Count: 4,
	}
	food, rent := reportCatID(1), reportCatID(2)
	rows := []model.ReportRow{
		reportRow(utc(9700, time.January, 1), food, "Еда", model.TypeExpense, 100),
		reportRow(utc(9800, time.January, 1), food, "Еда", model.TypeExpense, 30),
		reportRow(utc(9900, time.January, 1), rent, "Аренда", model.TypeExpense, 70),
		reportRow(utc(2025, time.January, 1), food, "Еда", model.TypeExpense, 999),
	}

	ops.EXPECT().ReportBounds(gomock.Any(), filter).Return(bounds, nil)
	ops.EXPECT().
		ReportRows(gomock.Any(), filter, model.GranularityYear).
		Return(rows, nil)

	rep, err := svc.Report(context.Background(), filter)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rep.Expense == nil {
		t.Fatal("expense section nil")
	}
	byCategory := make([]model.Money, len(rep.Expense.Categories))
	var byBuckets model.Money
	for _, bucket := range rep.Expense.Buckets {
		for i, amount := range bucket.Amounts {
			byCategory[i] += amount
		}
		byBuckets += bucket.Total
	}
	for i, cat := range rep.Expense.Categories {
		if !approxEqual(cat.Total, byCategory[i]) {
			t.Errorf("%q: Total = %v, сумма по бакетам = %v", cat.Name, cat.Total, byCategory[i])
		}
	}
	if !approxEqual(rep.Expense.Total, byBuckets) || !approxEqual(rep.Expense.Total, 200) {
		t.Errorf("Expense.Total = %v, по бакетам %v, want 200", rep.Expense.Total, byBuckets)
	}
	var share float64
	for _, cat := range rep.Expense.Categories {
		share += cat.Share
	}
	if !approxEqual(share, 100) {
		t.Errorf("сумма долей = %v%%, want 100%%", share)
	}
}

// TestReport_AllRowsOutsideGrid — если на обрезанную сетку не легла ни одна
// строка, страница показывает пустое состояние, а не пустой каркас.
func TestReport_AllRowsOutsideGrid(t *testing.T) {
	svc, ops := newReportSvc(t)

	filter := model.ReportFilter{
		UserID:      uuid.Must(uuid.NewV7()),
		DateFrom:    utc(2, time.January, 1),
		DateTo:      utc(9999, time.December, 31),
		Granularity: model.GranularityYear,
	}
	bounds := model.ReportBounds{
		From:  utc(2, time.January, 1),
		To:    utc(9999, time.December, 31),
		Count: 1,
	}
	rows := []model.ReportRow{
		reportRow(utc(2025, time.January, 1), reportCatID(1), "Еда", model.TypeExpense, 10),
	}

	ops.EXPECT().ReportBounds(gomock.Any(), filter).Return(bounds, nil)
	ops.EXPECT().
		ReportRows(gomock.Any(), filter, model.GranularityYear).
		Return(rows, nil)

	rep, err := svc.Report(context.Background(), filter)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rep.Labels) != 0 {
		t.Errorf("Labels = %d штук при пустых секциях — шаблон нарисует каркас без графиков", len(rep.Labels))
	}
	if rep.Expense != nil || rep.Income != nil {
		t.Errorf("sections: %+v / %+v, want nil", rep.Expense, rep.Income)
	}
	if rep.TotalNet != 0 || len(rep.Net) != 0 {
		t.Errorf("net: %v, total %v — want пусто", rep.Net, rep.TotalNet)
	}
}

// TestReport_MidnightBoundaryGridFits — операция ровно в полночь UTC даёт
// границу «полночь + наносекунда»: сетка обязана влезать в maxReportBuckets.
func TestReport_MidnightBoundaryGridFits(t *testing.T) {
	svc, ops := newReportSvc(t)

	filter := model.ReportFilter{
		UserID:      uuid.Must(uuid.NewV7()),
		DateFrom:    utc(2025, time.January, 1),
		Granularity: model.GranularityDay,
	}
	bounds := model.ReportBounds{
		From:  utc(2025, time.January, 1),
		To:    utc(2026, time.January, 2),
		Count: 2,
	}
	// понедельники: на недельной шкале БД вернёт начала недель
	rows := []model.ReportRow{
		reportRow(utc(2024, time.December, 30), reportCatID(1), "Еда", model.TypeExpense, 10),
		reportRow(utc(2025, time.December, 29), reportCatID(1), "Еда", model.TypeExpense, 20),
	}

	ops.EXPECT().ReportBounds(gomock.Any(), filter).Return(bounds, nil)
	ops.EXPECT().
		ReportRows(gomock.Any(), filter, model.GranularityWeek).
		Return(rows, nil)

	rep, err := svc.Report(context.Background(), filter)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rep.Labels) > maxReportBuckets {
		t.Fatalf("len(Labels) = %d, want <= %d", len(rep.Labels), maxReportBuckets)
	}
	if rep.Granularity != model.GranularityWeek {
		t.Errorf("granularity = %q, want week — 367 дней в шкалу не влезают", rep.Granularity)
	}
	if rep.Expense == nil || !approxEqual(rep.Expense.Total, 30) {
		t.Fatalf("expense section: %+v, want total 30", rep.Expense)
	}
}
