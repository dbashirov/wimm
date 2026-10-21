package handler

import (
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/dbashirov/wimm/internal/model"
	"github.com/dbashirov/wimm/web"
)

func TestParsePage(t *testing.T) {
	tests := []struct {
		in   string
		want int
	}{
		{"", 1},
		{"abc", 1},
		{"0", 1},
		{"-5", 1},
		{"3", 3},
		{strconv.Itoa(maxOperationsPage), maxOperationsPage},
		{strconv.Itoa(maxOperationsPage + 1), maxOperationsPage},
		{"9223372036854775807", maxOperationsPage},
		{"99999999999999999999", 1}, // не влезает в int - Atoi ошибка
	}
	for _, tc := range tests {
		if got := parsePage(tc.in); got != tc.want {
			t.Errorf("parsePage(%q) = %d, want %d", tc.in, got, tc.want)
		}
	}
}

func TestPageURL(t *testing.T) {
	filter := url.Values{"wallet_id": {"w1"}, "type": {"expense"}, "page": {"2"}}

	if got, want := pageURL(filter, 3), "?page=3&type=expense&wallet_id=w1"; got != want {
		t.Errorf("pageURL = %q, want %q", got, want)
	}
	if filter.Get("page") != "2" {
		t.Error("pageURL изменил исходные параметры запроса")
	}
	for _, page := range []int{0, maxOperationsPage + 1} {
		if got := pageURL(filter, page); got != "" {
			t.Errorf("pageURL(%d) = %q, want пусто", page, got)
		}
	}

	full := model.OperationsPageSize
	if nextPageURL(filter, 1, full-1) != "" {
		t.Error("неполная страница: ссылки «Вперёд» быть не должно")
	}
	if nextPageURL(filter, 1, full) == "" {
		t.Error("полная страница: нужна ссылка «Вперёд»")
	}
	if nextPageURL(filter, maxOperationsPage, full) != "" {
		t.Error("последняя допустимая страница: ссылки «Вперёд» быть не должно")
	}
}

// TestRenderOperationsPagination - ссылка на следующую страницу сохраняет
// фильтр: раньше она вела на ?page=N и сбрасывала его.
func TestRenderOperationsPagination(t *testing.T) {
	tmpls, err := parseTemplates(web.FS)
	if err != nil {
		t.Fatalf("parseTemplates: %v", err)
	}
	q := url.Values{"wallet_id": {"0190a000-0000-7000-8000-000000000001"}}
	var sb strings.Builder
	err = tmpls["operations"].ExecuteTemplate(&sb, "base", operationsData{
		Page:    1,
		NextURL: pageURL(q, 2),
	})
	if err != nil {
		t.Fatalf("ExecuteTemplate: %v", err)
	}
	want := `href="?page=2&amp;wallet_id=0190a000-0000-7000-8000-000000000001"`
	if !strings.Contains(sb.String(), want) {
		t.Errorf("нет ссылки %s", want)
	}
	if strings.Contains(sb.String(), "← Назад") {
		t.Error("на первой странице не должно быть ссылки «Назад»")
	}
}
