package handler

import (
	"strings"
	"testing"

	"github.com/dbashirov/wimm/internal/model"
	"github.com/dbashirov/wimm/web"
	"github.com/google/uuid"
)

func TestFormatMoney(t *testing.T) {
	tests := []struct {
		in   model.Money
		want string
	}{
		{0, "0.00"},
		{5, "0.05"},
		{99, "0.99"},
		{100, "1.00"},
		{123456, "1 234.56"},
		{123456789, "1 234 567.89"},
		{-123456, "-1 234.56"},
		{-5, "-0.05"},
	}
	for _, tc := range tests {
		if got := formatMoney(tc.in); got != tc.want {
			t.Errorf("formatMoney(%d) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// TestRenderDashboardTotalsByCurrency — итог дашборда выводится отдельно
// по каждой валюте, а не суммой рублей с долларами.
func TestRenderDashboardTotalsByCurrency(t *testing.T) {
	tmpls, err := parseTemplates(web.FS)
	if err != nil {
		t.Fatalf("parseTemplates: %v", err)
	}
	data := dashboardPageData{DashboardData: &model.DashboardData{
		WalletBalances: []model.WalletBalance{
			{Wallet: model.Wallet{Name: "Карта", Currency: "RUB"}, Balance: 1500000},
			{Wallet: model.Wallet{Name: "Долг", Currency: "USD"}, Balance: -2550},
		},
		TotalBalances: []model.CurrencyAmount{
			{Currency: "RUB", Amount: 1500000},
			{Currency: "USD", Amount: -2550},
		},
		MonthlyTotals: []model.MonthlyTotal{
			{Month: "2026-09", Income: 1500000, Expense: 2550, Balance: 1497450},
		},
	}}
	var sb strings.Builder
	if err := tmpls["dashboard"].ExecuteTemplate(&sb, "base", data); err != nil {
		t.Fatalf("ExecuteTemplate: %v", err)
	}
	html := sb.String()
	for _, want := range []string{"15 000.00", "-25.50"} {
		if !strings.Contains(html, want) {
			t.Errorf("на странице нет суммы %q", want)
		}
	}
}

func TestRenderTransferEditRow(t *testing.T) {
	tmpls, err := parseTemplates(web.FS)
	if err != nil {
		t.Fatalf("parseTemplates: %v", err)
	}
	transferID := uuid.Must(uuid.NewV7())
	rub := model.Wallet{ID: uuid.Must(uuid.NewV7()), Name: "Карта", Currency: "RUB"}
	usd := model.Wallet{ID: uuid.Must(uuid.NewV7()), Name: "Доллары", Currency: "USD"}
	cash := model.Wallet{ID: uuid.Must(uuid.NewV7()), Name: "Наличные", Currency: "RUB"}

	render := func(to model.Wallet, toAmount model.Money) string {
		t.Helper()
		var sb strings.Builder
		err := tmpls["operations"].ExecuteTemplate(&sb, "transfer-edit-row", map[string]any{
			"Expense": &model.Operation{
				ID: uuid.Must(uuid.NewV7()), WalletID: rub.ID, Currency: rub.Currency,
				Amount: 900000, TransferID: &transferID,
			},
			"Income": &model.Operation{
				ID: uuid.Must(uuid.NewV7()), WalletID: to.ID, Currency: to.Currency,
				Amount: toAmount, TransferID: &transferID,
			},
			"Wallets": []model.Wallet{rub, usd, cash},
		})
		if err != nil {
			t.Fatalf("ExecuteTemplate: %v", err)
		}
		return sb.String()
	}

	cross := render(usd, 10000)
	if !strings.Contains(cross, `name="to_amount"`) || !strings.Contains(cross, `value="100.00" required`) {
		t.Errorf("перевод между валютами: нет обязательного поля зачисления с суммой 100.00")
	}
	same := render(cash, 900000)
	if !strings.Contains(same, `value="9000.00" disabled`) {
		t.Errorf("перевод в одной валюте: поле зачисления должно быть выключено")
	}
}
