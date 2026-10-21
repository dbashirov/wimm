package handler

import (
	"strings"
	"testing"
	"time"

	"github.com/dbashirov/wimm/internal/model"
	"github.com/dbashirov/wimm/web"
	"github.com/google/uuid"
)

// TestOperationRowsRenderUTC - даты выводятся в UTC независимо от таймзоны
// процесса. parseDateTime разбирает ввод как UTC, поэтому рендер в местной
// зоне сдвигал бы операцию на смещение зоны при каждом сохранении формы.
func TestOperationRowsRenderUTC(t *testing.T) {
	tmpls, err := parseTemplates(web.FS)
	if err != nil {
		t.Fatalf("parseTemplates: %v", err)
	}

	msk := time.FixedZone("MSK", 3*60*60)
	op := &model.Operation{
		ID:           uuid.Must(uuid.NewV7()),
		WalletID:     uuid.Must(uuid.NewV7()),
		CategoryID:   uuid.Must(uuid.NewV7()),
		Type:         model.TypeExpense,
		Amount:       150075,
		OperatedAt:   time.Date(2026, time.March, 14, 12, 26, 0, 0, msk), // 09:26 UTC
		WalletName:   "Основной",
		CategoryName: "Продукты",
		Currency:     "RUB",
	}
	editData := map[string]any{
		"Op":         op,
		"Wallets":    []model.Wallet{{ID: op.WalletID, Name: "Основной", Currency: "RUB"}},
		"Categories": []model.Category{{ID: op.CategoryID, Name: "Продукты", Type: model.TypeExpense}},
	}
	transferID := uuid.Must(uuid.NewV7())
	transferData := map[string]any{
		"Expense": &model.Operation{
			ID: uuid.Must(uuid.NewV7()), WalletID: op.WalletID, Currency: "RUB",
			Amount: 150075, OperatedAt: op.OperatedAt, TransferID: &transferID,
		},
		"Income": &model.Operation{
			ID: uuid.Must(uuid.NewV7()), WalletID: op.WalletID, Currency: "RUB",
			Amount: 150075, OperatedAt: op.OperatedAt, TransferID: &transferID,
		},
		"Wallets": []model.Wallet{{ID: op.WalletID, Name: "Основной", Currency: "RUB"}},
	}

	tests := []struct {
		name     string
		partial  string
		data     any
		want     string
		unwanted string
	}{
		{
			name:     "строка списка",
			partial:  "operation-row",
			data:     op,
			want:     "09:26",
			unwanted: "12:26",
		},
		{
			name:     "форма редактирования",
			partial:  "operation-edit-row",
			data:     editData,
			want:     `value="2026-03-14T09:26"`,
			unwanted: "2026-03-14T12:26",
		},
		{
			name:     "форма редактирования перевода",
			partial:  "transfer-edit-row",
			data:     transferData,
			want:     `value="2026-03-14T09:26"`,
			unwanted: "2026-03-14T12:26",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var sb strings.Builder
			if err := tmpls["operations"].ExecuteTemplate(&sb, tc.partial, tc.data); err != nil {
				t.Fatalf("ExecuteTemplate: %v", err)
			}
			html := sb.String()
			if !strings.Contains(html, tc.want) {
				t.Errorf("нет %q - дата выведена не в UTC", tc.want)
			}
			if strings.Contains(html, tc.unwanted) {
				t.Errorf("найдено %q - дата выведена в местной зоне", tc.unwanted)
			}
		})
	}
}
