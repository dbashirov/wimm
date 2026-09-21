package handler

import (
	"context"
	"net/http/httptest"
	"slices"
	"testing"
	"time"

	"github.com/dbashirov/wimm/internal/model"
	"github.com/dbashirov/wimm/internal/service"
	"github.com/dbashirov/wimm/internal/storage/mocks"
	"github.com/google/uuid"
	"go.uber.org/mock/gomock"
)

func TestParseReportFilter(t *testing.T) {
	catA := uuid.Must(uuid.NewV7())
	catB := uuid.Must(uuid.NewV7())
	wallet := uuid.Must(uuid.NewV7())
	day := func(year int, month time.Month, num int) time.Time {
		return time.Date(year, month, num, 0, 0, 0, 0, time.UTC)
	}

	tests := []struct {
		name  string
		query string
		want  model.ReportFilter
	}{
		{
			name:  "пустой query",
			query: "",
			want:  model.ReportFilter{},
		},
		{
			name: "несколько категорий и кошелёк",
			query: "?category_id=" + catA.String() + "&category_id=" + catB.String() +
				"&wallet_id=" + wallet.String(),
			want: model.ReportFilter{
				CategoryIDs: []uuid.UUID{catA, catB},
				WalletIDs:   []uuid.UUID{wallet},
			},
		},
		{
			name:  "мусорные UUID отбрасываются",
			query: "?category_id=not-a-uuid&category_id=" + catA.String() + "&wallet_id=42",
			want:  model.ReportFilter{CategoryIDs: []uuid.UUID{catA}},
		},
		{
			name:  "валидный тип и гранулярность",
			query: "?type=income&granularity=week",
			want: model.ReportFilter{
				Type:        model.TypeIncome,
				Granularity: model.GranularityWeek,
			},
		},
		{
			name:  "невалидный тип и гранулярность сбрасываются",
			query: "?type=transfer&granularity=hour",
			want:  model.ReportFilter{},
		},
		{
			name:  "date_to сдвигается на день вперёд",
			query: "?date_from=2026-01-01&date_to=2026-01-31",
			want: model.ReportFilter{
				DateFrom: day(2026, time.January, 1),
				DateTo:   day(2026, time.February, 1),
			},
		},
		{
			name:  "один день — непустой период",
			query: "?date_from=2026-01-05&date_to=2026-01-05",
			want: model.ReportFilter{
				DateFrom: day(2026, time.January, 5),
				DateTo:   day(2026, time.January, 6),
			},
		},
		{
			name:  "перепутанные границы меняются местами",
			query: "?date_from=2026-03-10&date_to=2026-03-09",
			want: model.ReportFilter{
				DateFrom: day(2026, time.March, 9),
				DateTo:   day(2026, time.March, 11),
			},
		},
		{
			name:  "битые даты игнорируются",
			query: "?date_from=10.03.2026&date_to=",
			want:  model.ReportFilter{},
		},
		{
			name:  "только date_from",
			query: "?date_from=2026-03-10",
			want:  model.ReportFilter{DateFrom: day(2026, time.March, 10)},
		},
		{
			name:  "только date_to",
			query: "?date_to=2026-03-10",
			want:  model.ReportFilter{DateTo: day(2026, time.March, 11)},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/reports"+tc.query, nil)
			got := parseReportFilter(req)

			if !slices.Equal(got.CategoryIDs, tc.want.CategoryIDs) {
				t.Errorf("CategoryIDs = %v, want %v", got.CategoryIDs, tc.want.CategoryIDs)
			}
			if !slices.Equal(got.WalletIDs, tc.want.WalletIDs) {
				t.Errorf("WalletIDs = %v, want %v", got.WalletIDs, tc.want.WalletIDs)
			}
			if got.Type != tc.want.Type {
				t.Errorf("Type = %q, want %q", got.Type, tc.want.Type)
			}
			if got.Granularity != tc.want.Granularity {
				t.Errorf("Granularity = %q, want %q", got.Granularity, tc.want.Granularity)
			}
			if !got.DateFrom.Equal(tc.want.DateFrom) {
				t.Errorf("DateFrom = %v, want %v", got.DateFrom, tc.want.DateFrom)
			}
			if !got.DateTo.Equal(tc.want.DateTo) {
				t.Errorf("DateTo = %v, want %v", got.DateTo, tc.want.DateTo)
			}
			if got.UserID != uuid.Nil {
				t.Errorf("UserID = %v, want Nil — заполняет обработчик", got.UserID)
			}
		})
	}
}

func TestReportPresets(t *testing.T) {
	now := time.Date(2026, time.May, 18, 12, 0, 0, 0, time.UTC)
	query := httptest.NewRequest("GET", "/reports?type=expense&date_from=2026-05-01&date_to=2026-05-31", nil).
		URL.Query()

	presets := reportPresets(query, "2026-05-01", "2026-05-31", now)
	if len(presets) != 5 {
		t.Fatalf("len(presets) = %d, want 5", len(presets))
	}

	want := []struct {
		label  string
		url    string
		active bool
	}{
		{"Текущий месяц", "/reports?date_from=2026-05-01&date_to=2026-05-31&type=expense", true},
		{"Прошлый месяц", "/reports?date_from=2026-04-01&date_to=2026-04-30&type=expense", false},
		{"Квартал", "/reports?date_from=2026-04-01&date_to=2026-06-30&type=expense", false},
		{"Год", "/reports?date_from=2026-01-01&date_to=2026-12-31&type=expense", false},
		{"Всё время", "/reports?type=expense", false},
	}
	for i, item := range want {
		if presets[i].Label != item.label {
			t.Errorf("presets[%d].Label = %q, want %q", i, presets[i].Label, item.label)
		}
		if presets[i].URL != item.url {
			t.Errorf("presets[%d].URL = %q, want %q", i, presets[i].URL, item.url)
		}
		if presets[i].Active != item.active {
			t.Errorf("presets[%d].Active = %v, want %v", i, presets[i].Active, item.active)
		}
	}
}

// TestReportFilterFromURL_LongPeriodWithDayStep — ручная правка URL
// (granularity=day на пятилетнем периоде) не даёт гребёнку: сервис укрупняет
// шаг и объясняет это через Notice.
func TestReportFilterFromURL_LongPeriodWithDayStep(t *testing.T) {
	req := httptest.NewRequest("GET",
		"/reports?granularity=day&date_from=2020-01-01&date_to=2024-12-31", nil)
	filter := parseReportFilter(req)
	filter.UserID = uuid.Must(uuid.NewV7())
	if filter.Granularity != model.GranularityDay {
		t.Fatalf("Granularity = %q, want day — фильтр разобран не так", filter.Granularity)
	}

	ctrl := gomock.NewController(t)
	ops := mocks.NewMockOperationRepo(ctrl)
	ops.EXPECT().
		ReportBounds(gomock.Any(), filter).
		Return(model.ReportBounds{
			From:  time.Date(2020, time.March, 1, 0, 0, 0, 0, time.UTC),
			To:    time.Date(2024, time.November, 1, 0, 0, 0, 0, time.UTC),
			Count: 5,
		}, nil)
	ops.EXPECT().
		ReportRows(gomock.Any(), filter, gomock.Not(model.GranularityDay)).
		Return(nil, nil)

	svc := service.NewWithRepos(nil, nil, nil, ops)
	report, err := svc.Report(context.Background(), filter)
	if err != nil {
		t.Fatalf("Report: %v", err)
	}
	if report.Granularity == model.GranularityDay {
		t.Error("шаг остался дневным — защита от гребёнки не сработала")
	}
	// 366 — service.maxReportBuckets; константа неэкспортируемая,
	// кросс-пакетно сослаться на неё нельзя
	if len(report.Labels) > 366 {
		t.Errorf("len(Labels) = %d, want <= 366", len(report.Labels))
	}
	if report.Notice == "" {
		t.Error("Notice пуст — пользователь не узнает об укрупнении шага")
	}
}

func TestReportFormDates(t *testing.T) {
	from, to := reportFormDates(model.ReportFilter{})
	if from != "" || to != "" {
		t.Errorf("пустой фильтр дал %q–%q, ожидались пустые строки", from, to)
	}

	from, to = reportFormDates(model.ReportFilter{
		DateFrom: time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC),
		DateTo:   time.Date(2026, time.February, 1, 0, 0, 0, 0, time.UTC),
	})
	// DateTo эксклюзивна: в форме показывается последний включённый день
	if from != "2026-01-01" || to != "2026-01-31" {
		t.Errorf("got %q–%q, want 2026-01-01–2026-01-31", from, to)
	}
}

func TestHasReportFilter(t *testing.T) {
	tests := []struct {
		name   string
		filter model.ReportFilter
		want   bool
	}{
		{"пустой", model.ReportFilter{}, false},
		{"только UserID", model.ReportFilter{UserID: uuid.Must(uuid.NewV7())}, false},
		{"категории", model.ReportFilter{CategoryIDs: []uuid.UUID{uuid.Nil}}, true},
		{"кошельки", model.ReportFilter{WalletIDs: []uuid.UUID{uuid.Nil}}, true},
		{"тип", model.ReportFilter{Type: model.TypeIncome}, true},
		{"гранулярность", model.ReportFilter{Granularity: model.GranularityWeek}, true},
		{"дата от", model.ReportFilter{DateFrom: time.Now()}, true},
		{"дата до", model.ReportFilter{DateTo: time.Now()}, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := hasReportFilter(tc.filter); got != tc.want {
				t.Errorf("hasReportFilter = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestReportPresets_NoQuery — «Всё время» без прочих параметров даёт голый
// /reports и отмечается активным.
func TestReportPresets_NoQuery(t *testing.T) {
	now := time.Date(2026, time.May, 18, 12, 0, 0, 0, time.UTC)
	presets := reportPresets(nil, "", "", now)

	last := presets[len(presets)-1]
	if last.Label != "Всё время" {
		t.Fatalf("последний пресет = %q, want «Всё время»", last.Label)
	}
	if last.URL != "/reports" {
		t.Errorf("URL = %q, want /reports", last.URL)
	}
	if !last.Active {
		t.Error("«Всё время» не активно при пустом периоде")
	}
}

// TestParseUUIDs_Limit — повтор category_id тысячи раз не уезжает целиком
// в = ANY(...).
func TestParseUUIDs_Limit(t *testing.T) {
	values := make([]string, maxReportFilterIDs+50)
	for i := range values {
		values[i] = uuid.Must(uuid.NewV7()).String()
	}
	if got := len(parseUUIDs(values)); got != maxReportFilterIDs {
		t.Errorf("len = %d, want %d", got, maxReportFilterIDs)
	}
}

// TestSelectedGranularity — селект показывает применённый шаг, но «Авто»
// остаётся «Авто»: иначе первый же повторный запрос закрепил бы шаг.
func TestSelectedGranularity(t *testing.T) {
	tests := []struct {
		name      string
		requested model.Granularity
		applied   model.Granularity
		want      model.Granularity
	}{
		{"авто остаётся авто", "", model.GranularityWeek, ""},
		{"шаг применён как есть", model.GranularityMonth, model.GranularityMonth, model.GranularityMonth},
		{"шаг укрупнён сервисом", model.GranularityDay, model.GranularityWeek, model.GranularityWeek},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			filter := model.ReportFilter{Granularity: tc.requested}
			report := &model.Report{Granularity: tc.applied}
			if got := selectedGranularity(filter, report); got != tc.want {
				t.Errorf("selectedGranularity = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestSelectedGranularity_RoundTrip — значение селекта, отправленное обратно
// в query, должно разбираться в тот же шаг и не меняться дальше.
func TestSelectedGranularity_RoundTrip(t *testing.T) {
	first := selectedGranularity(
		model.ReportFilter{Granularity: model.GranularityDay},
		&model.Report{Granularity: model.GranularityWeek},
	)

	req := httptest.NewRequest("GET", "/reports?granularity="+string(first), nil)
	next := parseReportFilter(req)
	if next.Granularity != model.GranularityWeek {
		t.Fatalf("granularity после round-trip = %q, want week", next.Granularity)
	}
	second := selectedGranularity(next, &model.Report{Granularity: model.GranularityWeek})
	if second != first {
		t.Errorf("второй проход дал %q, want %q", second, first)
	}

	auto := httptest.NewRequest("GET", "/reports?granularity=", nil)
	if got := selectedGranularity(parseReportFilter(auto), &model.Report{Granularity: model.GranularityMonth}); got != "" {
		t.Errorf("«Авто» превратилось в %q", got)
	}
}
