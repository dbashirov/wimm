package handler

import (
	"io"
	"testing"
	"time"

	"github.com/dbashirov/wimm/internal/model"
	"github.com/dbashirov/wimm/web"
	"github.com/google/uuid"
)

// TestParseTemplates — единственная защита от битого шаблона: parseTemplates
// вызывается из handler.New, а его ошибка роняет сервер при старте.
func TestParseTemplates(t *testing.T) {
	tmpls, err := parseTemplates(web.FS)
	if err != nil {
		t.Fatalf("parseTemplates: %v", err)
	}
	for _, page := range []string{"dashboard", "operations", "wallet_stats", "reports"} {
		if tmpls[page] == nil {
			t.Errorf("шаблон %q не собран", page)
		}
	}
}

// reportSection — секция отчёта для рендера шаблона.
func reportSection(labels []string, names ...string) *model.ReportSection {
	cats := make([]model.ReportCategory, 0, len(names))
	for i, name := range names {
		cats = append(cats, model.ReportCategory{
			ID:    uuid.Must(uuid.NewV7()),
			Name:  name,
			Total: float64(100 * (i + 1)),
			Share: 100 / float64(len(names)),
		})
	}
	buckets := make([]model.ReportBucket, len(labels))
	for i := range buckets {
		amounts := make([]float64, len(cats))
		var total float64
		for j := range amounts {
			amounts[j] = float64(10 * (i + j + 1))
			total += amounts[j]
		}
		buckets[i] = model.ReportBucket{Amounts: amounts, Total: total}
	}
	return &model.ReportSection{
		Categories: cats,
		Buckets:    buckets,
		Total:      600,
		Average:    300,
	}
}

// TestRenderReportsTemplate прогоняет reports.html на трёх наборах данных:
// битый шаблон или переименованное поле модели ломают тест, а не прод.
func TestRenderReportsTemplate(t *testing.T) {
	tmpls, err := parseTemplates(web.FS)
	if err != nil {
		t.Fatalf("parseTemplates: %v", err)
	}
	labels := []string{"01.01", "02.01", "03.01"}
	cats := []model.Category{
		{ID: uuid.Must(uuid.NewV7()), Name: "Еда", Type: model.TypeExpense},
		{ID: uuid.Must(uuid.NewV7()), Name: "Зарплата", Type: model.TypeIncome},
	}
	wallets := []model.Wallet{{ID: uuid.Must(uuid.NewV7()), Name: "Основной", Currency: "RUB"}}

	full := &model.Report{
		Granularity: model.GranularityDay,
		Labels:      labels,
		Expense:     reportSection(labels, "Еда", "Транспорт"),
		Income:      reportSection(labels, "Зарплата"),
		Net:         []float64{10, -20, 30},
		TotalNet:    20,
		Bounds: model.ReportBounds{
			From:  time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC),
			To:    time.Date(2026, time.January, 3, 0, 0, 0, 0, time.UTC),
			Count: 12,
		},
		Notice: "Слишком много периодов — шаг укрупнён до недель",
	}
	onlyExpense := &model.Report{
		Granularity: model.GranularityMonth,
		Labels:      labels,
		Expense:     reportSection(labels, "Еда"),
		Net:         []float64{-10, -20, -30},
		Bounds:      full.Bounds,
	}
	empty := &model.Report{Granularity: model.GranularityDay}

	tests := []struct {
		name   string
		report *model.Report
	}{
		{"полный отчёт", full},
		{"только расходы", onlyExpense},
		{"пустой отчёт", empty},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			data := reportData{
				Report:     tc.report,
				Wallets:    wallets,
				Categories: cats,
				Filter:     model.ReportFilter{Type: model.TypeExpense},
				Presets:    reportPresets(nil, "", "", time.Now()),
				HasFilter:  true,
			}
			if err := tmpls["reports"].ExecuteTemplate(io.Discard, "base", data); err != nil {
				t.Fatalf("ExecuteTemplate: %v", err)
			}
		})
	}
}
