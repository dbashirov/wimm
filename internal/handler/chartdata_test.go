package handler

import (
	"strings"
	"testing"

	"github.com/dbashirov/wimm/internal/model"
	"github.com/dbashirov/wimm/web"
	"github.com/google/uuid"
)

// evilName - название категории, ломающее ручную сборку JS-литерала.
const evilName = `</script><script>alert(1)</script>"'`

// TestChartDataUsesJSON - данные графиков dashboard и статистики кошелька
// идут в JS через jsonData, а не ручной развёрткой {{range}}.
func TestChartDataUsesJSON(t *testing.T) {
	stats := statsWithCharts()
	stats.MonthlyExpenses[0].Items[0].Name = evilName

	pages := map[string]string{
		"wallet_stats": renderWalletStats(t, stats),
		"dashboard":    renderDashboard(t, stats),
	}
	for page, html := range pages {
		t.Run(page, func(t *testing.T) {
			if strings.Contains(html, "<script>alert(1)") {
				t.Error("название категории закрыло <script>")
			}
			for _, want := range []string{
				`makeMonthCharts('exp-', [{"Label":"Август","Total":20.00,` +
					`"Items":[{"Name":"` + "\\u003c/script\\u003e",
				`makeMonthCharts('inc-', null)`,
				`var TOTALS = [{"Month":"2026-08","Income":30.00,"Expense":20.00,"Balance":10.00}]`,
			} {
				if !strings.Contains(html, want) {
					t.Errorf("нет %s", want)
				}
			}
		})
	}
}

// TestOperationsCategoriesJSON - категории формы добавления операции
// передаются в JS через jsonData.
func TestOperationsCategoriesJSON(t *testing.T) {
	tmpls, err := parseTemplates(web.FS)
	if err != nil {
		t.Fatalf("parseTemplates: %v", err)
	}
	var sb strings.Builder
	err = tmpls["operations"].ExecuteTemplate(&sb, "base", operationsData{
		Page: 1,
		Categories: []model.Category{{
			ID: uuid.Must(uuid.NewV7()), Name: evilName, Type: model.TypeExpense,
		}},
	})
	if err != nil {
		t.Fatalf("ExecuteTemplate: %v", err)
	}
	html := sb.String()
	if strings.Contains(html, "<script>alert(1)") {
		t.Error("название категории закрыло <script>")
	}
	if !strings.Contains(html, `var OP_CATEGORIES = [{"ID":"`) {
		t.Error("категории не переданы через jsonData")
	}
}

func renderDashboard(t *testing.T, data *model.DashboardData) string {
	t.Helper()
	tmpls, err := parseTemplates(web.FS)
	if err != nil {
		t.Fatalf("parseTemplates: %v", err)
	}
	var sb strings.Builder
	err = tmpls["dashboard"].ExecuteTemplate(&sb, "base", dashboardPageData{DashboardData: data})
	if err != nil {
		t.Fatalf("ExecuteTemplate: %v", err)
	}
	return sb.String()
}
