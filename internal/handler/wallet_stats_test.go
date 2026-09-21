package handler

import (
	"io"
	"strings"
	"testing"

	"github.com/dbashirov/wimm/internal/model"
	"github.com/dbashirov/wimm/web"
)

// renderWalletStats рендерит страницу статистики кошелька и возвращает HTML.
func renderWalletStats(t *testing.T, data *model.DashboardData) string {
	t.Helper()
	tmpls, err := parseTemplates(web.FS)
	if err != nil {
		t.Fatalf("parseTemplates: %v", err)
	}
	var sb strings.Builder
	err = tmpls["wallet_stats"].ExecuteTemplate(&sb, "base", walletStatsData{
		DashboardData:  data,
		WalletName:     "Основной",
		WalletCurrency: "RUB",
		BackURL:        "/wallets",
	})
	if err != nil {
		t.Fatalf("ExecuteTemplate: %v", err)
	}
	return sb.String()
}

// statsWithCharts — данные, при которых страница рисует графики.
func statsWithCharts() *model.DashboardData {
	return &model.DashboardData{
		WalletBalances: []model.WalletBalance{{
			Wallet:  model.Wallet{Name: "Основной", Currency: "RUB"},
			Balance: 1000,
		}},
		MonthlyTotals: []model.MonthlyTotal{
			{Month: "2026-08", Income: 3000, Expense: 2000, Balance: 1000},
		},
		MonthlyExpenses: []model.MonthlyCategoryData{{
			Label: "Август",
			Total: 2000,
			Items: []model.CategoryAmount{{Name: "Еда", Amount: 2000}},
		}},
	}
}

// TestWalletStatsLoadsChartLibs — защита от регрессии: Chart.js и ECharts должны
// подключаться на странице статистики кошелька, иначе инлайновый скрипт падает
// на "echarts is not defined" и графики не рисуются.
func TestWalletStatsLoadsChartLibs(t *testing.T) {
	html := renderWalletStats(t, statsWithCharts())

	for _, lib := range []string{"chart.js", "echarts"} {
		if !strings.Contains(html, lib) {
			t.Errorf("библиотека %q не подключена на странице", lib)
		}
	}
	if !strings.Contains(html, "new Chart(") {
		t.Fatal("код графиков не попал в страницу")
	}
}

// TestWalletStatsChartScriptAfterLibs — инлайновый скрипт обязан идти после
// тегов библиотек: обычный <script src> выполняется синхронно, поэтому порядок
// в документе и определяет, будут ли Chart и echarts определены к его запуску.
func TestWalletStatsChartScriptAfterLibs(t *testing.T) {
	html := renderWalletStats(t, statsWithCharts())

	libs := strings.LastIndex(html, "echarts.min.js")
	code := strings.Index(html, "Chart.defaults.color")
	if libs == -1 || code == -1 {
		t.Fatalf("не найдено: libs=%d code=%d", libs, code)
	}
	if code < libs {
		t.Errorf("код графиков (%d) идёт раньше библиотек (%d)", code, libs)
	}
}

// TestWalletStatsNoChartsWithoutData — без операций библиотеки не тянутся.
func TestWalletStatsNoChartsWithoutData(t *testing.T) {
	html := renderWalletStats(t, &model.DashboardData{
		WalletBalances: []model.WalletBalance{{
			Wallet: model.Wallet{Name: "Основной", Currency: "RUB"},
		}},
	})

	if strings.Contains(html, "echarts.min.js") {
		t.Error("библиотеки подключены, хотя операций нет")
	}
	if !strings.Contains(html, "По этому кошельку пока нет операций") {
		t.Error("не отрендерилось пустое состояние")
	}
}

// TestParseTemplatesWalletStats — ошибка parseTemplates роняет handler.New,
// а он делает os.Exit(1) в main, то есть сервер не поднимется целиком.
func TestParseTemplatesWalletStats(t *testing.T) {
	tmpls, err := parseTemplates(web.FS)
	if err != nil {
		t.Fatalf("parseTemplates: %v", err)
	}
	if tmpls["wallet_stats"] == nil {
		t.Fatal("шаблон wallet_stats не собран")
	}
	if err := tmpls["wallet_stats"].ExecuteTemplate(io.Discard, "base", walletStatsData{
		DashboardData: statsWithCharts(),
	}); err != nil {
		t.Fatalf("рендер: %v", err)
	}
}
