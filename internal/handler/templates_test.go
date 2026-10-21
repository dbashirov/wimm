package handler

import (
	"io"
	"io/fs"
	"regexp"
	"strings"
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
			Total: model.Money(10000 * (i + 1)),
			Share: 100 / float64(len(names)),
		})
	}
	buckets := make([]model.ReportBucket, len(labels))
	for i := range buckets {
		amounts := make([]model.Money, len(cats))
		var total model.Money
		for j := range amounts {
			amounts[j] = model.Money(1000 * (i + j + 1))
			total += amounts[j]
		}
		buckets[i] = model.ReportBucket{Amounts: amounts, Total: total}
	}
	return &model.ReportSection{
		Categories: cats,
		Buckets:    buckets,
		Total:      60000,
		Average:    30000,
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
		Net:         []model.Money{1000, -2000, 3000},
		TotalNet:    2000,
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
		Net:         []model.Money{-1000, -2000, -3000},
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

// TestTooltipNamesEscaped - подсказки ECharts вставляются как HTML, поэтому
// название категории в formatter обязано идти через escapeHTML: иначе
// категория "<img src=x onerror=...>" выполняет скрипт при наведении.
func TestTooltipNamesEscaped(t *testing.T) {
	rawName := regexp.MustCompile(`[^(]p\.name`)
	files, err := fs.Glob(web.FS, "templates/*.html")
	if err != nil {
		t.Fatalf("Glob: %v", err)
	}
	checked := 0
	for _, name := range files {
		data, err := fs.ReadFile(web.FS, name)
		if err != nil {
			t.Fatalf("ReadFile %s: %v", name, err)
		}
		src := string(data)
		if !strings.Contains(src, "formatter:") {
			continue
		}
		checked++
		for _, m := range rawName.FindAllString(src, -1) {
			t.Errorf("%s: p.name без escapeHTML: %q", name, m)
		}
		if !strings.Contains(src, "escapeHTML(p.name)") {
			t.Errorf("%s: в formatter нет escapeHTML(p.name)", name)
		}
	}
	if checked == 0 {
		t.Fatal("не найдено ни одного шаблона с formatter")
	}
}

// TestEscapeHTMLInBase - escapeHTML объявлен в base.html, его используют
// подсказки графиков и тосты с текстом ответа сервера.
func TestEscapeHTMLInBase(t *testing.T) {
	data, err := fs.ReadFile(web.FS, "templates/base.html")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	src := string(data)
	if !strings.Contains(src, "function escapeHTML(") {
		t.Error("в base.html нет функции escapeHTML")
	}
	if strings.Contains(src, "+ msg +") {
		t.Error("тост вставляет текст ответа сервера в innerHTML без экранирования")
	}
}
