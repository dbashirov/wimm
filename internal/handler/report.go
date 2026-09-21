package handler

import (
	"net/http"
	"net/url"
	"time"

	"github.com/dbashirov/wimm/internal/model"
	"github.com/google/uuid"
)

// reportPreset — готовая ссылка на период отчёта.
type reportPreset struct {
	Label  string
	URL    string
	Active bool
}

type reportData struct {
	basePage
	Report      *model.Report
	Wallets     []model.Wallet
	Categories  []model.Category
	Filter      model.ReportFilter
	DateFrom    string // значения для полей формы: date_to без эксклюзивного +1 дня
	DateTo      string
	Granularity model.Granularity // выбранное значение селекта «Шаг шкалы»
	Presets     []reportPreset
	HasFilter   bool
}

func (h *Handler) getReport(w http.ResponseWriter, r *http.Request) {
	userID := userIDFromCtx(r.Context())

	filter := parseReportFilter(r)
	filter.UserID = userID

	report, err := h.svc.Report(r.Context(), filter)
	if err != nil {
		h.serverError(w, r, "Report", err)
		return
	}
	wallets, err := h.svc.ListWallets(r.Context(), userID)
	if err != nil {
		h.serverError(w, r, "ListWallets", err)
		return
	}
	cats, err := h.svc.ListCategories(r.Context(), userID)
	if err != nil {
		h.serverError(w, r, "ListCategories", err)
		return
	}

	dateFrom, dateTo := reportFormDates(filter)
	h.render(w, "reports", reportData{
		Report:      report,
		Wallets:     wallets,
		Categories:  cats,
		Filter:      filter,
		DateFrom:    dateFrom,
		DateTo:      dateTo,
		Granularity: selectedGranularity(filter, report),
		Presets:     reportPresets(r.URL.Query(), dateFrom, dateTo, time.Now()),
		HasFilter:   hasReportFilter(filter),
	})
}

// selectedGranularity — значение селекта «Шаг шкалы»: «Авто» остаётся «Авто»,
// а выбранный вручную шаг показывается тем, который отчёт применил на самом
// деле — сервис мог укрупнить его под длину периода.
func selectedGranularity(f model.ReportFilter, report *model.Report) model.Granularity {
	if f.Granularity == "" {
		return ""
	}
	return report.Granularity
}

// reportFormDates возвращает границы периода для полей формы:
// DateTo эксклюзивна, пользователю показываем последний включённый день.
func reportFormDates(f model.ReportFilter) (string, string) {
	var from, to string
	if !f.DateFrom.IsZero() {
		from = f.DateFrom.Format(dateLayout)
	}
	if !f.DateTo.IsZero() {
		to = f.DateTo.AddDate(0, 0, -1).Format(dateLayout)
	}
	return from, to
}

// hasReportFilter — задан ли хоть один параметр фильтра; от этого зависит,
// развёрнута ли панель фильтра и показывать ли «Сбросить».
func hasReportFilter(f model.ReportFilter) bool {
	return len(f.CategoryIDs) > 0 ||
		len(f.WalletIDs) > 0 ||
		f.Type != "" ||
		f.Granularity != "" ||
		!f.DateFrom.IsZero() ||
		!f.DateTo.IsZero()
}

// parseReportFilter разбирает query-параметры отчёта. UserID заполняет вызывающий.
func parseReportFilter(r *http.Request) model.ReportFilter {
	q := r.URL.Query()
	filter := model.ReportFilter{
		CategoryIDs: parseUUIDs(q["category_id"]),
		WalletIDs:   parseUUIDs(q["wallet_id"]),
	}

	switch opType := model.OperationType(q.Get("type")); opType {
	case model.TypeIncome, model.TypeExpense:
		filter.Type = opType
	}

	switch g := model.Granularity(q.Get("granularity")); g {
	case model.GranularityDay, model.GranularityWeek, model.GranularityMonth, model.GranularityYear:
		filter.Granularity = g
	}

	from := parseReportDate(q.Get("date_from"))
	to := parseReportDate(q.Get("date_to"))
	if !from.IsZero() && !to.IsZero() && to.Before(from) {
		from, to = to, from // границы перепутаны местами — это не повод терять период
	}
	filter.DateFrom = from
	if !to.IsZero() {
		filter.DateTo = to.AddDate(0, 0, 1) // эксклюзивная граница: SQL-фильтр < DateTo
	}
	return filter
}

// parseReportDate разбирает дату фильтра, молча игнорируя мусор.
func parseReportDate(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	parsed, err := time.Parse(dateLayout, s)
	if err != nil {
		return time.Time{}
	}
	return parsed
}

// maxReportFilterIDs — потолок на число id в одном query-параметре:
// повторить его можно тысячи раз, и весь массив уходит в = ANY(...).
const maxReportFilterIDs = 200

// parseUUIDs разбирает повторяющийся query-параметр, молча пропуская невалидные значения.
func parseUUIDs(values []string) []uuid.UUID {
	if len(values) > maxReportFilterIDs {
		values = values[:maxReportFilterIDs]
	}
	ids := make([]uuid.UUID, 0, len(values))
	for _, value := range values {
		if id, err := uuid.Parse(value); err == nil {
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		return nil
	}
	return ids
}

// reportPresets строит ссылки-пресеты периода, сохраняя остальные параметры фильтра.
// Границы считаются в UTC, как и вся сетка отчёта: в ссылку уходят только
// «год-месяц-день», и зона сервера не должна сдвигать их на сутки.
func reportPresets(
	q url.Values,
	dateFrom string,
	dateTo string,
	now time.Time,
) []reportPreset {
	year, month, _ := now.UTC().Date()
	monthStart := time.Date(year, month, 1, 0, 0, 0, 0, time.UTC)
	quarterStart := time.Date(year, month-(month-1)%3, 1, 0, 0, 0, 0, time.UTC)
	yearStart := time.Date(year, time.January, 1, 0, 0, 0, 0, time.UTC)

	ranges := []struct {
		label string
		from  time.Time
		to    time.Time
	}{
		{"Текущий месяц", monthStart, monthStart.AddDate(0, 1, -1)},
		{"Прошлый месяц", monthStart.AddDate(0, -1, 0), monthStart.AddDate(0, 0, -1)},
		{"Квартал", quarterStart, quarterStart.AddDate(0, 3, -1)},
		{"Год", yearStart, yearStart.AddDate(1, 0, -1)},
		{"Всё время", time.Time{}, time.Time{}},
	}

	presets := make([]reportPreset, 0, len(ranges))
	for _, item := range ranges {
		var from, to string
		if !item.from.IsZero() {
			from = item.from.Format(dateLayout)
			to = item.to.Format(dateLayout)
		}
		presets = append(presets, reportPreset{
			Label:  item.label,
			URL:    reportURL(q, from, to),
			Active: dateFrom == from && dateTo == to,
		})
	}
	return presets
}

// reportURL подменяет в текущем query только границы периода.
func reportURL(q url.Values, from, to string) string {
	next := url.Values{}
	for key, values := range q {
		if key == "date_from" || key == "date_to" {
			continue
		}
		next[key] = values
	}
	if from != "" {
		next.Set("date_from", from)
		next.Set("date_to", to)
	}
	if len(next) == 0 {
		return "/reports"
	}
	return "/reports?" + next.Encode()
}
