package service

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/dbashirov/wimm/internal/model"
	"github.com/google/uuid"
)

// пороги авто-гранулярности в днях
const (
	maxDaysForDay   = 31
	maxDaysForWeek  = 92
	maxDaysForMonth = 1826
)

const secondsPerDay = 24 * 60 * 60

// намеренный дубль storage.ruMonths: лейблы отчёта — забота service,
// тащить ради 12 литералов зависимость service→storage не стоит
var reportMonthsFull = [12]string{
	"Январь", "Февраль", "Март", "Апрель", "Май", "Июнь",
	"Июль", "Август", "Сентябрь", "Октябрь", "Ноябрь", "Декабрь",
}

var reportMonthsShort = [12]string{
	"янв", "фев", "мар", "апр", "май", "июн",
	"июл", "авг", "сен", "окт", "ноя", "дек",
}

// pickGranularity выбирает шаг шкалы по длине периода [from, to).
func pickGranularity(from, to time.Time) model.Granularity {
	// не time.Duration: она переполняется на диапазонах в тысячи лет
	days := int((to.UTC().Unix() - from.UTC().Unix()) / secondsPerDay)
	switch {
	case days <= maxDaysForDay:
		return model.GranularityDay
	case days <= maxDaysForWeek:
		return model.GranularityWeek
	case days <= maxDaysForMonth:
		return model.GranularityMonth
	default:
		return model.GranularityYear
	}
}

// truncate — зеркало date_trunc из SQL-агрегации отчёта.
// Всегда считает по UTC: иначе Go-сетка разъедется с бакетами БД.
func truncate(ts time.Time, g model.Granularity) time.Time {
	u := ts.UTC()
	switch g {
	case model.GranularityYear:
		return time.Date(u.Year(), time.January, 1, 0, 0, 0, 0, time.UTC)
	case model.GranularityMonth:
		return time.Date(u.Year(), u.Month(), 1, 0, 0, 0, 0, time.UTC)
	case model.GranularityWeek:
		day := time.Date(u.Year(), u.Month(), u.Day(), 0, 0, 0, 0, time.UTC)
		// date_trunc('week') в PostgreSQL отсчитывает неделю с понедельника
		offset := (int(day.Weekday()) + 6) % 7
		return day.AddDate(0, 0, -offset)
	default:
		return time.Date(u.Year(), u.Month(), u.Day(), 0, 0, 0, 0, time.UTC)
	}
}

// nextBucket возвращает начало следующего бакета после ts.
func nextBucket(ts time.Time, g model.Granularity) time.Time {
	switch g {
	case model.GranularityYear:
		return ts.AddDate(1, 0, 0)
	case model.GranularityMonth:
		return ts.AddDate(0, 1, 0)
	case model.GranularityWeek:
		return ts.AddDate(0, 0, 7)
	default:
		return ts.AddDate(0, 0, 1)
	}
}

// bucketStarts строит полную сетку бакетов для [from, to),
// включая периоды без операций.
func bucketStarts(from, to time.Time, g model.Granularity) []time.Time {
	if !to.After(from) {
		return nil
	}
	end := to.UTC()
	var starts []time.Time
	for cur := truncate(from, g); cur.Before(end); cur = nextBucket(cur, g) {
		starts = append(starts, cur)
	}
	return starts
}

// bucketLabel форматирует подпись бакета; multiYear добавляет год там,
// где без него подписи разных лет неразличимы.
func bucketLabel(ts time.Time, g model.Granularity, multiYear bool) string {
	start := truncate(ts, g)
	switch g {
	case model.GranularityYear:
		return fmt.Sprintf("%d", start.Year())
	case model.GranularityMonth:
		if multiYear {
			return fmt.Sprintf(
				"%s %d",
				capitalize(reportMonthsShort[start.Month()-1]),
				start.Year(),
			)
		}
		return reportMonthsFull[start.Month()-1]
	case model.GranularityWeek:
		return weekLabel(start, multiYear)
	default:
		if multiYear {
			return fmt.Sprintf(
				"%02d.%02d.%02d",
				start.Day(), int(start.Month()), start.Year()%100,
			)
		}
		return fmt.Sprintf("%02d.%02d", start.Day(), int(start.Month()))
	}
}

// weekLabel подписывает неделю, раскрывая месяц только при переходе через
// его границу; год добавляется при переходе через год и на многолетней
// шкале, где без него подписи соседних лет совпадают.
func weekLabel(start time.Time, multiYear bool) string {
	end := start.AddDate(0, 0, 6)
	startMonth := reportMonthsShort[start.Month()-1]
	endMonth := reportMonthsShort[end.Month()-1]
	switch {
	case start.Year() != end.Year():
		return fmt.Sprintf(
			"%d %s %02d – %d %s %02d",
			start.Day(), startMonth, start.Year()%100,
			end.Day(), endMonth, end.Year()%100,
		)
	case multiYear && start.Month() != end.Month():
		return fmt.Sprintf(
			"%d %s – %d %s %02d",
			start.Day(), startMonth, end.Day(), endMonth, end.Year()%100,
		)
	case multiYear:
		return fmt.Sprintf(
			"%d–%d %s %02d",
			start.Day(), end.Day(), startMonth, start.Year()%100,
		)
	case start.Month() != end.Month():
		return fmt.Sprintf(
			"%d %s – %d %s",
			start.Day(), startMonth, end.Day(), endMonth,
		)
	default:
		return fmt.Sprintf("%d–%d %s", start.Day(), end.Day(), startMonth)
	}
}

func capitalize(s string) string {
	runes := []rune(s)
	if len(runes) == 0 {
		return s
	}
	return strings.ToUpper(string(runes[0])) + string(runes[1:])
}

// maxReportCategories — сколько категорий показываем в секции отчёта:
// остальные схлопываются в «Прочее», иначе стек и пончик нечитаемы.
const maxReportCategories = 8

// otherCategoryName — подпись схлопнутого хвоста категорий.
const otherCategoryName = "Прочее"

// maxSelectedCategories — потолок для явно выбранных категорий. Кнопка
// «Выбрать все» отправляет их десятками, а в палитре графиков 15 цветов:
// 14 категорий плюс «Прочее» занимают её ровно, дальше цвета повторяются.
const maxSelectedCategories = 14

// collapseCategories сводит сырые строки агрегации к итогам по категориям:
// топ-limit по сумме, хвост — в «Прочее» (с uuid.Nil в ID).
// Второй результат — индекс категории в срезе по её ID, им caller
// раскладывает суммы бакетов по колонкам секции.
func collapseCategories(
	rows []model.ReportRow,
	limit int,
) ([]model.ReportCategory, map[uuid.UUID]int) {
	totals := make(map[uuid.UUID]*model.ReportCategory, len(rows))
	order := make([]uuid.UUID, 0, len(rows))
	var sum float64
	for _, row := range rows {
		cat, ok := totals[row.CategoryID]
		if !ok {
			cat = &model.ReportCategory{ID: row.CategoryID, Name: row.Name}
			totals[row.CategoryID] = cat
			order = append(order, row.CategoryID)
		}
		cat.Total += row.Amount
		sum += row.Amount
	}

	cats := make([]model.ReportCategory, 0, len(order))
	for _, id := range order {
		cats = append(cats, *totals[id])
	}
	sort.Slice(cats, func(i, j int) bool {
		if cats[i].Total != cats[j].Total {
			return cats[i].Total > cats[j].Total
		}
		return cats[i].Name < cats[j].Name
	})

	var collapsed []uuid.UUID
	if limit > 0 && len(cats) > limit {
		other := model.ReportCategory{ID: uuid.Nil, Name: otherCategoryName}
		for _, cat := range cats[limit:] {
			other.Total += cat.Total
			collapsed = append(collapsed, cat.ID)
		}
		cats = append(cats[:limit:limit], other)
	}

	index := make(map[uuid.UUID]int, len(totals)+1)
	for i, cat := range cats {
		index[cat.ID] = i
	}
	for _, id := range collapsed {
		index[id] = len(cats) - 1
	}

	for i := range cats {
		if sum != 0 {
			cats[i].Share = cats[i].Total / sum * 100
		}
	}
	return cats, index
}

// maxReportBuckets — предел колонок на шкале: сверх него шаг укрупняется,
// иначе график превращается в гребёнку.
const maxReportBuckets = 366

var granularityNames = map[model.Granularity]string{
	model.GranularityDay:   "дней",
	model.GranularityWeek:  "недель",
	model.GranularityMonth: "месяцев",
	model.GranularityYear:  "лет",
}

func validGranularity(g model.Granularity) bool {
	_, ok := granularityNames[g]
	return ok
}

// coarser возвращает следующий по крупности шаг шкалы.
func coarser(g model.Granularity) (model.Granularity, bool) {
	switch g {
	case model.GranularityDay:
		return model.GranularityWeek, true
	case model.GranularityWeek:
		return model.GranularityMonth, true
	case model.GranularityMonth:
		return model.GranularityYear, true
	default:
		return g, false
	}
}

// bucketCount считает размер сетки [from, to) арифметически, не материализуя
// её: даты из query ничем не ограничены, и bucketStarts на диапазоне в тысячи
// лет разворачивается в миллионы элементов ещё до проверки maxReportBuckets.
func bucketCount(from, to time.Time, g model.Granularity) int {
	if !to.After(from) {
		return 0
	}
	start, end := truncate(from, g), to.UTC()
	// tail — бакет, начатый end и не дотянувший до своего конца
	tail := 0
	if truncate(end, g).Before(end) {
		tail = 1
	}
	switch g {
	case model.GranularityYear:
		return end.Year() - start.Year() + tail
	case model.GranularityMonth:
		return (end.Year()-start.Year())*12 + int(end.Month()) - int(start.Month()) + tail
	default:
		// секунды, а не time.Duration: она переполняется на тысячах лет
		step := int64(secondsPerDay)
		if g == model.GranularityWeek {
			step = 7 * secondsPerDay
		}
		// хвост считаем по времени, а не по Unix(): секунды режут доли,
		// а сетка сравнивает границу с наносекундной точностью
		n := int((end.Unix() - start.Unix()) / step)
		if shiftBuckets(start, g, n).Before(end) {
			n++
		}
		return n
	}
}

// shiftBuckets сдвигает начало бакета на n шагов (n может быть отрицательным).
func shiftBuckets(ts time.Time, g model.Granularity, n int) time.Time {
	switch g {
	case model.GranularityYear:
		return ts.AddDate(n, 0, 0)
	case model.GranularityMonth:
		return ts.AddDate(0, n, 0)
	case model.GranularityWeek:
		return ts.AddDate(0, 0, 7*n)
	default:
		return ts.AddDate(0, 0, n)
	}
}

// lastBucketStart возвращает начало последнего бакета сетки, кончающейся в to.
func lastBucketStart(to time.Time, g model.Granularity) time.Time {
	end := to.UTC()
	if start := truncate(end, g); start.Before(end) {
		return start
	}
	// граница эксклюзивна: бакет, начинающийся ровно в to, в сетку не входит
	return truncate(end.Add(-time.Nanosecond), g)
}

// reportGrid подбирает шаг шкалы и строит сетку так, что len(grid) никогда
// не превышает maxReportBuckets: сначала укрупняется шаг, а когда укрупнять
// уже некуда (year) — обрезается начало диапазона.
func reportGrid(
	from, to time.Time,
	g model.Granularity,
) ([]time.Time, model.Granularity, string) {
	var notice string
	for bucketCount(from, to, g) > maxReportBuckets {
		next, ok := coarser(g)
		if !ok {
			break
		}
		g = next
		notice = "Слишком много периодов — шаг укрупнён до " + granularityNames[g]
	}
	if bucketCount(from, to, g) > maxReportBuckets {
		from = shiftBuckets(lastBucketStart(to, g), g, -(maxReportBuckets - 1))
		notice = fmt.Sprintf(
			"Слишком длинный период — показаны последние %d %s",
			maxReportBuckets, granularityNames[g],
		)
	}
	return bucketStarts(from, to, g), g, notice
}

// reportRange выбирает диапазон сетки: явно заданный период приоритетнее
// фактических границ данных.
func reportRange(f model.ReportFilter, b model.ReportBounds) (time.Time, time.Time) {
	from, to := f.DateFrom, f.DateTo
	if from.IsZero() {
		from = b.From
	}
	if to.IsZero() && !b.To.IsZero() {
		// Bounds.To — момент последней операции, а граница сетки эксклюзивна
		to = b.To.UTC().Add(time.Nanosecond)
	}
	return from, to
}

// clampToBounds сужает диапазон до фактических границ данных.
// Вызывается только когда сетка не влезает в maxReportBuckets: иначе явно
// заданный период показывается целиком, с нулями по краям.
func clampToBounds(from, to time.Time, b model.ReportBounds) (time.Time, time.Time) {
	if b.From.IsZero() || b.To.IsZero() {
		return from, to
	}
	if b.From.After(from) {
		from = b.From
	}
	// граница сетки эксклюзивна, последняя операция обязана в неё попасть
	if end := b.To.UTC().Add(time.Nanosecond); end.Before(to) {
		to = end
	}
	return from, to
}

// reportGranularity — шаг шкалы: ручной выбор приоритетнее авто-подбора.
func reportGranularity(f model.ReportFilter, from, to time.Time) model.Granularity {
	if validGranularity(f.Granularity) {
		return f.Granularity
	}
	return pickGranularity(from, to)
}

// resolveGrid выбирает диапазон и шаг шкалы и строит по ним сетку бакетов.
func resolveGrid(
	f model.ReportFilter,
	bounds model.ReportBounds,
) ([]time.Time, model.Granularity, string) {
	from, to := reportRange(f, bounds)
	g := reportGranularity(f, from, to)
	if bucketCount(from, to, g) > maxReportBuckets {
		// запрошенный период шире данных настолько, что сетка не влезает:
		// показываем данные, а не пустые бакеты по краям
		from, to = clampToBounds(from, to, bounds)
		g = reportGranularity(f, from, to)
	}
	return reportGrid(from, to, g)
}

// splitRowsByType разносит строки агрегации по секциям расходов и доходов.
func splitRowsByType(rows []model.ReportRow) (expense, income []model.ReportRow) {
	for _, row := range rows {
		if row.Type == model.TypeIncome {
			income = append(income, row)
			continue
		}
		expense = append(expense, row)
	}
	return expense, income
}

// Report собирает отчёт по категориям: границы данных → шаг шкалы →
// агрегация по бакетам → секции расходов и доходов на общей шкале.
func (s *Service) Report(ctx context.Context, f model.ReportFilter) (*model.Report, error) {
	bounds, err := s.operations.ReportBounds(ctx, f)
	if err != nil {
		return nil, err
	}

	from, to := reportRange(f, bounds)
	report := &model.Report{Granularity: reportGranularity(f, from, to), Bounds: bounds}
	if bounds.Count == 0 {
		return report, nil
	}

	grid, g, notice := resolveGrid(f, bounds)
	report.Granularity, report.Notice = g, notice
	if len(grid) == 0 {
		return report, nil
	}

	rows, err := s.operations.ReportRows(ctx, f, g)
	if err != nil {
		return nil, err
	}

	pos := make(map[time.Time]int, len(grid))
	for i, start := range grid {
		pos[start] = i
	}

	expenseRows, incomeRows := splitRowsByType(rows)

	limit := maxReportCategories
	if n := len(f.CategoryIDs); n > 0 {
		// категории выбраны явно — показываем все, пока их не столько,
		// что график всё равно нечитаем
		limit = min(n, maxSelectedCategories)
	}
	report.Expense = buildSection(expenseRows, grid, pos, g, limit)
	report.Income = buildSection(incomeRows, grid, pos, g, limit)
	if report.Expense == nil && report.Income == nil {
		// ни одна строка не легла на сетку: без Labels страница покажет
		// пустое состояние, а не голый каркас из нулевых карточек
		return report, nil
	}

	multiYear := grid[0].Year() != grid[len(grid)-1].Year()
	report.Labels = make([]string, len(grid))
	for i, start := range grid {
		report.Labels[i] = bucketLabel(start, g, multiYear)
	}

	report.Net = make([]float64, len(grid))
	for i := range grid {
		report.Net[i] = bucketTotal(report.Income, i) - bucketTotal(report.Expense, i)
		report.TotalNet += report.Net[i]
	}
	return report, nil
}

// buildSection раскладывает строки одного типа по сетке бакетов;
// nil означает, что операций такого типа в выборке нет.
// limit — сколько категорий показать до схлопывания хвоста в «Прочее».
func buildSection(
	rows []model.ReportRow,
	grid []time.Time,
	pos map[time.Time]int,
	g model.Granularity,
	limit int,
) *model.ReportSection {
	if len(rows) == 0 {
		return nil
	}
	// строки вне сетки отбрасываем до сведения категорий: иначе итоги
	// по категориям посчитались бы за более широкий период, чем шкала
	inGrid := make([]model.ReportRow, 0, len(rows))
	for _, row := range rows {
		if _, ok := pos[truncate(row.Bucket, g)]; ok {
			inGrid = append(inGrid, row)
		}
	}
	if len(inGrid) == 0 {
		return nil
	}
	rows = inGrid
	cats, index := collapseCategories(rows, limit)
	buckets := make([]model.ReportBucket, len(grid))
	for i := range grid {
		buckets[i] = model.ReportBucket{Amounts: make([]float64, len(cats))}
	}

	section := &model.ReportSection{Categories: cats, Buckets: buckets}
	for _, row := range rows {
		i := pos[truncate(row.Bucket, g)]
		col := index[row.CategoryID]
		buckets[i].Amounts[col] += row.Amount
		buckets[i].Total += row.Amount
		section.Total += row.Amount
	}
	section.Average = section.Total / float64(len(buckets))
	return section
}

func bucketTotal(section *model.ReportSection, i int) float64 {
	if section == nil {
		return 0
	}
	return section.Buckets[i].Total
}
