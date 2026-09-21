# Отчёт по категориям (страница /reports)

> Ревизия 2 — учтены замечания авто-ревью (см. раздел «История ревизий» в конце).

## Overview

Новая страница `/reports`: расходы и доходы по одной или нескольким выбранным
категориям за произвольный период (если период не задан — за всё время),
с разбивкой по временным интервалам, графиками и общим итогом.

**Проблема.** Сейчас аналитика жёстко зашита: `/dashboard` и `/wallets/{id}/stats`
показывают фиксированные окна (3 месяца по категориям, 12 месяцев по суммам) по
всем категориям сразу. Ответить на вопрос «сколько я потратил на еду и транспорт
с января по июнь» нельзя — только вручную листать `/operations` с фильтром по
одной категории без итогов.

**Решение.** Отдельная страница с фильтром (мультивыбор категорий, кошельки, тип,
период, гранулярность) и тремя уровнями представления данных:
итоговые числа → графики динамики и структуры → таблица по периодам.

**Ключевые решения (согласованы с пользователем):**

| Вопрос | Решение |
|---|---|
| Размещение | Новая страница `/reports` + пункт меню «Отчёты» |
| Мультивалютность | Не разделяем — суммируем `amount` как есть (как `TotalBalance` на дашборде) |
| Гранулярность | Авто: день / неделя / месяц / год по длине периода + ручное переключение |
| Графики | Stacked bar по категориям + линия среднего, пончик долей, таблица периодов |
| Тесты | Как в проекте: storage — testcontainers, service — gomock; плюс один табличный тест на парсинг фильтра |

## Context (from discovery)

**Файлы и компоненты:**
- `internal/model/model.go` — `OperationFilter`, `DashboardData`, `MonthlyCategoryData`, `MonthlyTotal`
- `internal/storage/operation.go:352-593` — существующие агрегаты: `fetchMonthlyCategories`,
  `DashboardData`, `WalletDashboardData`, хелпер `monthLabel` + `ruMonths`
- `internal/storage/iface.go` — `OperationRepo`, генерация моков через `go:generate`
- `internal/service/service.go:384-437` — `WalletStats`, `DashboardData`
- `internal/handler/handler.go:100-123` — `New` и `parseTemplates` со списком страниц
- `internal/handler/handler.go:146-200` — `Routes`, все защищённые роуты через `h.protected`
- `internal/handler/operation.go:25-78` — эталон парсинга фильтра из query
- `web/templates/wallet_stats.html` — эталон страницы с графиками (Chart.js + ECharts)
- `web/templates/operations.html:230-276` — эталон формы фильтра (`method="GET"`, flatpickr)
- `web/templates/base.html` — навигация (desktop `<nav>` + mobile `#mobile-menu`)

**Найденные паттерны:**
- Слои строго `handler → service → storage`, каждый SQL содержит `WHERE user_id = $N`
- Суммы: `NUMERIC(15,2)` в БД → явный `::float8` в запросах → `float64` в Go
- Шаблоны: `base.html + page.html + partials/*.html`, блоки `head_extra` и `scripts`
  для page-specific библиотек (Chart.js/ECharts/flatpickr грузятся только где нужны)
- Переводы (`transfer_id IS NOT NULL`) имеют `category_id = NULL` и исключаются
  из агрегатов дашборда — в отчёте по категориям исключаются автоматически
- Тесты storage: testcontainers + goose (`testmain_test.go`), хелперы `cleanDB`, `createUser`
- Ошибки: `fmt.Errorf("Тип.Метод: %w", err)`, `errors.Is(err, storage.ErrNotFound)`

**Проверенные факты, влияющие на реализацию:**
- `cmd/server/main.go:51-54` — ошибка `handler.New` → `os.Exit(1)`. Регистрация страницы
  в `parseTemplates` **до** создания шаблона уронит весь сервер, а не одну страницу.
- `migrations/001_init.sql:53` — `operated_at TIMESTAMPTZ` **без** `NOT NULL`;
  ни один существующий агрегат от NULL не защищается.
- `web/static/` содержит только `style.css` — общего JS-ассета нет, `makeRoseChart`
  из `wallet_stats.html` — локальная функция inline-скрипта, её придётся копировать.
- pgx `v5.8.0` — `nil`-слайс кодируется в SQL `NULL`, не в `'{}'`.
- `internal/service/service.go:252-274` — `CreateOperation` требует существующую категорию;
  `category_id IS NULL` бывает только у переводов. `JOIN categories` безопасен,
  ветка «Без категории» не нужна.

**Зависимости:** новых Go-зависимостей не требуется. Frontend — уже подключаемые
по CDN Chart.js, ECharts, flatpickr, htmx, DaisyUI.

## Development Approach

- **testing approach**: Regular (код → тесты), как сложилось в проекте
- завершать каждую задачу полностью перед переходом к следующей
- маленькие сфокусированные изменения
- **CRITICAL: каждая задача с изменением кода ОБЯЗАНА включать новые/обновлённые тесты**
  - storage — интеграционные тесты на testcontainers (`internal/storage/*_test.go`)
  - service — юнит-тесты на gomock-моках (`internal/service/service_test.go` как эталон)
  - handler — один табличный тест на `parseReportFilter` (чистая функция, Handler не нужен)
  - покрывать и успешные, и граничные сценарии
- **CRITICAL: каждая задача заканчивается `go build ./... && go test ./...` — всё зелёное**
- **CRITICAL: обновлять этот файл при изменении объёма работ**
- обратная совместимость: существующие `/dashboard` и `/wallets/{id}/stats` не трогаем
- соблюдать правила проекта: лимит строки 120 символов, `uuid.Must(uuid.NewV7())`,
  `pgx.CollectRows` + `pgx.RowToStructByName` вместо ручного `rows.Scan`,
  комментарии внутри функций с маленькой буквы, без избыточных комментариев

## Testing Strategy

- **unit tests**: обязательны в каждой задаче (см. Development Approach)
- **integration tests**: `internal/storage/report_test.go` на реальном PostgreSQL
  через testcontainers — SQL-агрегация и есть основной риск
- **e2e tests**: в проекте отсутствуют (нет Playwright/Cypress) — не вводим
- **ручная проверка UI**: вынесена в Post-Completion

## Progress Tracking

- отмечать выполненное `[x]` сразу по факту
- новые обнаруженные задачи — с префиксом ➕
- проблемы и блокеры — с префиксом ⚠️
- держать план в синхроне с реальной работой

## Solution Overview

### Разделение по типу операции

Категория всегда принадлежит одному типу (`categories.type`). Смешивать доходы
и расходы в одном стеке нельзя — отчёт строится **двумя независимыми секциями**
(расходы и доходы) на **общей временной шкале**. Если выбраны категории только
одного типа — вторая секция не рендерится.

### Поток данных

```
GET /reports?category_id=..&category_id=..&wallet_id=..&type=..
            &date_from=..&date_to=..&granularity=..
   ↓ handler.getReport — парсинг query → model.ReportFilter
   ↓ service.Report
       1. storage.ReportBounds → MIN/MAX(operated_at), COUNT — фактический диапазон
       2. авто-гранулярность по длине диапазона (если granularity не задана явно)
       3. storage.ReportRows  → SUM по (bucket, category_id)
       4. сборка: полная сетка бакетов (включая пустые), top-N категорий + «Прочее»,
          лейблы, итоги, средние
   ↓ render reports.html
```

Два SQL-запроса вместо одного: границы нужны **до** выбора гранулярности, а без
них «за всё время» невозможно определить шаг сетки. Оба read-only и независимы —
транзакция не нужна.

### Диапазон сетки бакетов

- период задан явно → сетка строится от `DateFrom` до `DateTo`, пустые периоды
  по краям остаются нулями (пользователь просил конкретный период — он должен
  увидеть его целиком)
- период не задан → сетка строится от `ReportBounds.From` до `ReportBounds.To`
- данных нет вообще → пустой отчёт, рендерится пустое состояние

### Авто-гранулярность

| Длина периода | Гранулярность |
|---|---|
| ≤ 31 день | `day` |
| ≤ 92 дня | `week` |
| ≤ 1826 дней (~5 лет) | `month` |
| > 1826 дней | `year` |

Ручной выбор перекрывает авто. Защита от гребёнки двухуровневая:
в UI недопустимые для текущего периода варианты помечены `disabled`,
на сервере — если выбранный шаг даёт > 366 бакетов, поднимаемся на уровень
крупнее и заполняем `Report.Notice` (URL правится руками, серверу нельзя верить UI).

### Таймзона — UTC с обеих сторон

`date_trunc` в PostgreSQL считает в таймзоне сессии, а Go-код может оказаться
в `time.Local`. Если сетка бакетов из Go разъедется с бакетами из SQL, часть сумм
не найдёт своего бакета и **молча потеряется**. Поэтому:

- SQL: `date_trunc($2, o.operated_at AT TIME ZONE 'UTC')` — результат не зависит
  от настроек сессии
- Go: `truncate(t, g)` работает по `t.UTC()`

Это сильнее, чем в существующих агрегатах дашборда, но там нет сшивки Go-сетки
и SQL-бакетов, а здесь есть.

### Top-N категорий

Если в выборку попало больше 8 категорий (например, фильтр по категориям не задан,
а у пользователя их 30) — берём топ-8 по сумме, остальные схлопываем в «Прочее».
Иначе stacked bar и пончик превращаются в нечитаемую радугу. Порог — константа
`maxReportCategories = 8` в service.

### Ключевые design-решения и обоснование

1. **Отдельный файл `internal/storage/report.go`, но методы на существующем
   `OperationStore`** — отчёт работает с теми же `operations`, отдельный репозиторий
   создал бы дублирование подключения и лишний слой. Интерфейс `OperationRepo`
   расширяется двумя методами, моки регенерируются.
2. **Формирование лейблов и сетки бакетов — в service, не в SQL.** `generate_series`
   с интервалом-параметром усложняет запрос и требует отдельной ветки на каждую
   гранулярность; в Go это прозрачный цикл, который легко покрыть юнит-тестами.
3. **`date_trunc` c гранулярностью через параметр запроса**, значение — из
   белого списка констант `model.Granularity`, валидируется в service. SQL-инъекция
   невозможна: значение не конкатенируется в текст запроса.
4. **Все опциональные фильтры — через nullable-параметры, а не через пустые значения.**
   Пустая строка не равна SQL `NULL`: `""::operation_type` — ошибка enum,
   нулевой `time.Time` — это `0001-01-01`, `nil`-слайс в pgx — `NULL`.
   Нормализация делается в storage, на границе с БД (см. Technical Details).
5. **Мультивыбор категорий чекбоксами, не `<select multiple>`** — `<select multiple>`
   на мобильных практически неюзабелен, а чекбоксы дают группировку по типу
   и «выбрать все / снять всё».
6. **Форма фильтра — обычный `method="GET"`**, как на `/operations`. Состояние
   отчёта живёт в URL: ссылку можно сохранить и переслать. htmx здесь не нужен.
7. **Датасеты в JS отдаются через `json.Marshal`, а не ручной развёрткой шаблона.**
   Паттерн `[{{range}}"{{.Name}}",{{end}}]` из `wallet_stats.html:182-200` годится
   для одномерных списков; матрица «категории × периоды» на вложенных `range`
   с ручной расстановкой скобок — гарантированный источник битого JS.

## Technical Details

### Новые типы (`internal/model/model.go`)

```go
type Granularity string

const (
    GranularityDay   Granularity = "day"
    GranularityWeek  Granularity = "week"
    GranularityMonth Granularity = "month"
    GranularityYear  Granularity = "year"
)

// ReportFilter — параметры отчёта по категориям.
type ReportFilter struct {
    UserID      uuid.UUID
    CategoryIDs []uuid.UUID
    WalletIDs   []uuid.UUID
    Type        OperationType // "" — оба типа
    DateFrom    time.Time
    DateTo      time.Time   // эксклюзивная граница
    Granularity Granularity // "" — авто
}

// ReportRow — сырая строка агрегации из БД.
// Имена тегов совпадают с алиасами колонок для pgx.RowToStructByName.
type ReportRow struct {
    Bucket     time.Time     `db:"bucket"`
    CategoryID uuid.UUID     `db:"category_id"`
    Name       string        `db:"name"`
    Type       OperationType `db:"type"`
    Amount     float64       `db:"amount"`
}

// ReportBounds — фактические границы данных под фильтр.
type ReportBounds struct {
    From  time.Time
    To    time.Time
    Count int
}

// ReportCategory — итог по одной категории за весь период.
type ReportCategory struct {
    ID    uuid.UUID
    Name  string
    Total float64
    Share float64 // % от итога секции; 0 при нулевом итоге
}

// ReportBucket — один период на временной шкале.
type ReportBucket struct {
    Start   time.Time
    Label   string
    Amounts []float64 // выровнены по ReportSection.Categories
    Total   float64
}

// ReportSection — отчёт по одному типу операций.
type ReportSection struct {
    Type       OperationType
    Categories []ReportCategory
    Buckets    []ReportBucket
    Total      float64
    Average    float64 // Total / len(Buckets); 0 при пустой сетке
}

// Report — данные страницы отчёта.
type Report struct {
    Filter      ReportFilter
    Granularity Granularity
    Labels      []string
    Expense     *ReportSection
    Income      *ReportSection
    Net         []float64 // доход − расход по бакетам
    TotalNet    float64
    Bounds      ReportBounds
    Notice      string // «шаг укрупнён до месяцев»; пустая строка — сообщать нечего
}
```

### SQL-агрегация (`internal/storage/report.go`)

```sql
SELECT date_trunc($2, o.operated_at AT TIME ZONE 'UTC') AS bucket,
       o.category_id                                    AS category_id,
       c.name                                           AS name,
       c.type                                           AS type,
       SUM(o.amount)::float8                            AS amount
FROM operations o
JOIN categories c ON c.id = o.category_id AND c.user_id = o.user_id
WHERE o.user_id = $1
  AND o.transfer_id IS NULL
  AND o.operated_at IS NOT NULL
  AND (cardinality($3::uuid[]) = 0 OR o.category_id = ANY($3::uuid[]))
  AND (cardinality($4::uuid[]) = 0 OR o.wallet_id   = ANY($4::uuid[]))
  AND ($5::operation_type IS NULL OR o.type = $5)
  AND ($6::timestamptz    IS NULL OR o.operated_at >= $6)
  AND ($7::timestamptz    IS NULL OR o.operated_at <  $7)
GROUP BY 1, 2, 3, 4
ORDER BY bucket, amount DESC, name
```

Фиксированный набор параметров вместо динамической сборки строки: запрос один,
план кешируется, нумерация плейсхолдеров не плывёт.

**Нормализация параметров перед вызовом — обязательна, иначе ломается базовый
сценарий «за всё время без фильтров»:**

| Поле фильтра | Пустое значение в Go | Что передавать в запрос |
|---|---|---|
| `CategoryIDs` / `WalletIDs` | `nil` | `[]uuid.UUID{}` — иначе pgx пишет SQL `NULL`, `cardinality(NULL)` → `NULL`, весь `OR` → `NULL`, строка отсеивается |
| `Type` | `""` | `nil` (`*model.OperationType`) — `""::operation_type` даёт `invalid input value for enum` |
| `DateFrom` / `DateTo` | `time.Time{}` | `nil` (`*time.Time`) — иначе `operated_at < '0001-01-01'`, отчёт всегда пуст |

Алиасы у всех колонок обязательны: `pgx.RowToStructByName` матчит поля по именам
колонок, а `SUM(o.amount)::float8` без алиаса придёт как `sum`/`float8`.

`AND o.operated_at IS NOT NULL` — защита от NULLable колонки (`migrations/001_init.sql:53`):
`date_trunc(NULL)` вернёт NULL, скан в `time.Time` упадёт.

Запрос границ (`ReportBounds`) — те же условия `WHERE`, но
`SELECT MIN(operated_at), MAX(operated_at), COUNT(*)`; границы nullable,
сканируются через `pgtype.Timestamptz`.

### Сборка отчёта (`internal/service/report.go`)

- `pickGranularity(from, to time.Time) Granularity` — таблица порогов
- `truncate(t time.Time, g Granularity) time.Time` — зеркало `date_trunc`,
  всегда по `t.UTC()`; неделя — понедельник (как `date_trunc('week')` в PG)
- `bucketStarts(from, to time.Time, g Granularity) []time.Time` — полная сетка,
  включая периоды без операций (иначе график «съедает» пустые месяцы и врёт
  о динамике)
- `bucketLabel(t time.Time, g Granularity, multiYear bool) string`:
  - `day` → `18.09`, при `multiYear` → `18.09.25`
  - `week` → `18–24 авг`; если неделя пересекает месяц — `28 июл – 3 авг`;
    если год — `30 дек 24 – 5 янв 25`
  - `month` → `Сентябрь`, при `multiYear` → `Сен 2025`
  - `year` → `2026`
- `collapseCategories(rows []model.ReportRow, limit int)` — top-N + «Прочее»
- деления на ноль: `Share` при `Total == 0` → `0`; `Average` при пустой сетке → `0`.
  Ни `NaN`, ни `±Inf` не должны попасть в шаблон — `money`/`num` их отрендерят мусором

### Handler (`internal/handler/report.go`)

Query-параметры:

| Параметр | Формат | Примечание |
|---|---|---|
| `category_id` | повторяющийся UUID | `r.URL.Query()["category_id"]`, невалидные молча пропускаются |
| `wallet_id` | повторяющийся UUID | то же |
| `type` | `income` / `expense` / пусто | пусто = оба; прочие значения → пусто |
| `date_from`, `date_to` | `2006-01-02` | `date_to` → `+1 день` (SQL-фильтр `< DateTo`) |
| `granularity` | `day`/`week`/`month`/`year` | пусто = авто; прочие значения → пусто |

`DateFrom > DateTo` → фильтр очищается, отчёт строится за всё время.

Пресеты периода (`Текущий месяц`, `Прошлый месяц`, `Квартал`, `Год`, `Всё время`) —
**готовые ссылки**, которые handler кладёт в данные страницы: каждая уже содержит
посчитанные `date_from`/`date_to`. Отдельной ветки парсинга и приоритета над
датами нет — иначе ручная правка URL даёт неочевидное поведение.

`reportData` **обязана встраивать `basePage`** (`internal/handler/handler.go:36-38`),
иначе `base.html` упадёт на `.NoNav`.

Роут: `mux.Handle("GET /reports", h.protected(h.getReport))`.

Новые хелперы в `tmplFuncs`:
- `jsonData(v any) template.JS` — `json.Marshal` для датасетов графиков
- `hasID(ids []uuid.UUID, id uuid.UUID) bool` — отметка выбранных чекбоксов

### Шаблон (`web/templates/reports.html`)

1. **Шапка**: заголовок + карточки итогов (Расходы / Доходы / Итог, количество
   операций, фактический диапазон дат) + `Notice`, если он не пуст
2. **Фильтр** (`<details class="collapse">`, как на `/operations`):
   - пресеты периода ссылками, поля дат (flatpickr, класс `fp-date`)
   - переключатель гранулярности; варианты, не подходящие под текущий период,
     помечены `disabled`
   - чекбоксы категорий, сгруппированные по типу, с «Выбрать все / Снять всё»
   - мультивыбор кошельков
   - кнопки «Применить» / «Сбросить»
3. **График динамики**: stacked bar по категориям (Chart.js, `stack: 'x'`)
   + пунктирная линия среднего уровня — отдельно для расходов и доходов
4. **Пончики**: доли категорий за весь период, отдельно расходы и доходы (ECharts)
5. **Таблица**: строки — периоды, столбцы — категории + «Итого» (+ «Net», когда
   в отчёте обе секции), футер — общий итог, `overflow-x-auto` для мобильных
6. **Пустое состояние**: когда под фильтр не попало ни одной операции

Блоки `head_extra` (flatpickr CSS) и `scripts` (flatpickr + Chart.js + ECharts) —
как в `operations.html` и `wallet_stats.html`. Реакция на смену темы —
`MutationObserver` по `data-theme`, как в `wallet_stats.html`.

`makeRoseChart` и `fmtMoney` **копируются** из `wallet_stats.html` в inline-скрипт
`reports.html`: общего JS-ассета в проекте нет, а вынос в `web/static/charts.js`
потребовал бы править и `wallet_stats.html` — это отдельная задача вне объёма
(записана в Post-Completion).

### Известные ограничения (осознанные)

- **Валюты не разделяются** — суммы разных кошельков складываются как есть.
  Согласовано; консистентно с текущим `TotalBalance` на дашборде.
- **`makeRoseChart` дублируется** между `wallet_stats.html` и `reports.html`.
  Осознанная плата за то, чтобы не трогать работающую страницу.

## What Goes Where

- **Implementation Steps** (`[ ]`): код, тесты, документация в этом репозитории
- **Post-Completion** (без чекбоксов): ручная проверка UI, индекс по результатам
  профилирования на реальных данных, рефакторинг общего JS

## Implementation Steps

### Task 1: Типы отчёта в модели

**Files:**
- Modify: `internal/model/model.go`

- [x] добавить тип `Granularity` и константы `GranularityDay/Week/Month/Year`
- [x] добавить `ReportFilter`, `ReportRow` (с `db`-тегами), `ReportBounds`
- [x] добавить `ReportCategory`, `ReportBucket`, `ReportSection`, `Report`
- [x] `go build ./... && go test ./...` — зелёное перед Task 2

### Task 2: ReportBounds — границы данных под фильтр

**Files:**
- Create: `internal/storage/report.go`
- Create: `internal/storage/report_test.go`

- [x] создать `internal/storage/report.go` с `OperationStore.ReportBounds`
- [x] реализовать нормализацию параметров фильтра: `nil`-слайсы → пустые,
      `Type ""` → `nil`, нулевые `time.Time` → `nil` (общий хелпер для обоих методов)
- [x] реализовать запрос `MIN/MAX(operated_at)`, `COUNT(*)` с nullable-границами
      через `pgtype.Timestamptz`
- [x] написать тест: непустая выборка возвращает корректные границы и счётчик
- [x] написать тест: пустая выборка — нулевые границы, `Count == 0`, без ошибки
- [x] написать тест: `nil`-слайсы категорий и кошельков = фильтр не применяется
      (страховка от кодирования `nil` в SQL `NULL`)
- [x] написать тест: пустой `Type` не роняет запрос на enum
- [x] написать тест: нулевые даты не обрезают выборку
- [x] `go build ./... && go test ./...` — зелёное перед Task 3

### Task 3: ReportRows — SQL-агрегация по бакетам

**Files:**
- Modify: `internal/storage/report.go`
- Modify: `internal/storage/report_test.go`

- [x] реализовать `OperationStore.ReportRows` с алиасами всех колонок
      и `date_trunc($2, o.operated_at AT TIME ZONE 'UTC')`
- [x] добавить в `WHERE` условия `transfer_id IS NULL` и `operated_at IS NOT NULL`
- [x] сканировать через `pgx.CollectRows` + `pgx.RowToStructByName`
- [x] написать тесты агрегации по `day`, `week`, `month`, `year` на фикстуре
      с операциями в разных месяцах
- [x] написать тест: `c.type` корректно скан ится в `model.OperationType`
- [x] написать тесты фильтров: несколько категорий, несколько кошельков, тип,
      границы периода (операция ровно на границе месяца и недели)
- [x] написать тест изоляции: операции другого пользователя не попадают в отчёт
- [x] написать тест: переводы (`transfer_id IS NOT NULL`) исключены
- [x] написать тест: операция с `operated_at IS NULL` не роняет запрос
- [x] `go build ./... && go test ./...` — зелёное перед Task 4

### Task 4: Расширение интерфейса OperationRepo и моков

**Files:**
- Modify: `internal/storage/iface.go`
- Modify: `internal/storage/mocks/mock_repos.go` (регенерация)

- [x] добавить `ReportBounds` и `ReportRows` в интерфейс `OperationRepo`
- [x] перенести сигнатуры через несколько строк (правило: 4+ параметра — по строке)
- [x] регенерировать моки: `go generate ./internal/storage/...`
- [x] `go build ./... && go test ./...` — существующие тесты service не сломались

### Task 5: Временные хелперы отчёта

**Files:**
- Create: `internal/service/report.go`
- Create: `internal/service/report_test.go`

- [x] реализовать `pickGranularity` — пороги 31 / 92 / 1826 дней
- [x] реализовать `truncate(t, g)` — зеркало `date_trunc`, всегда по `t.UTC()`,
      неделя с понедельника
- [x] реализовать `bucketStarts` — полная сетка, включая пустые периоды
- [x] реализовать `bucketLabel` — форматы day/week/month/year, короткие русские
      месяцы, год в лейбле только при пересечении годов
- [x] написать табличные тесты `pickGranularity` (границы 31/32, 92/93, 1826/1827)
- [x] написать тесты `truncate` для всех гранулярностей, включая вход в `time.Local`
      (результат обязан совпадать с UTC-вариантом)
- [x] написать тесты `bucketStarts`: один бакет, пустой диапазон, переход через год
- [x] написать тесты `bucketLabel`: неделя через месяц, неделя через год, `multiYear`
- [x] `go build ./... && go test ./...` — зелёное перед Task 6

### Task 6: Схлопывание категорий в top-N

**Files:**
- Modify: `internal/service/report.go`
- Modify: `internal/service/report_test.go`

- [x] добавить константу `maxReportCategories = 8`
- [x] реализовать `collapseCategories` — top-N по сумме + «Прочее»
- [x] написать тесты: категорий меньше лимита, ровно лимит, больше лимита
- [x] написать тест: суммы «Прочего» равны сумме схлопнутых категорий
- [x] написать тест: порядок категорий — по убыванию суммы
- [x] `go build ./... && go test ./...` — зелёное перед Task 7

### Task 7: Сборка Service.Report

**Files:**
- Modify: `internal/service/report.go`
- Modify: `internal/service/report_test.go`

- [x] реализовать `Service.Report(ctx, filter)`: bounds → гранулярность → rows → сборка
- [x] выбрать диапазон сетки: явный период приоритетнее `Bounds`
- [x] реализовать защиту от гребёнки: > 366 бакетов → шаг крупнее + `Notice`
- [x] собрать секции по типу, общую шкалу `Labels`, `Net`, `TotalNet`, итоги, средние
- [x] защитить от деления на ноль: `Share` при `Total == 0`, `Average` при пустой сетке
- [x] написать тест на gomock: пустые бакеты заполнены нулями, `Share` считается
      от итога секции, категории отсортированы по убыванию
- [x] написать тест: ручная гранулярность перекрывает авто
- [x] написать тест: > 366 бакетов → шаг укрупнён, `Notice` не пуст
- [x] написать тест: пустая выборка → пустой отчёт без `NaN`/`Inf`
- [x] написать тест: категории только одного типа → вторая секция `nil`
- [x] `go build ./... && go test ./...` — зелёное перед Task 8

### Task 8: Handler, парсинг фильтра и маршрут

**Files:**
- Create: `internal/handler/report.go`
- Create: `internal/handler/report_test.go`
- Modify: `internal/handler/handler.go`

- [x] создать `reportData` со встроенным `basePage` и обработчик `getReport`
- [x] реализовать `parseReportFilter(r)` — мультизначные `category_id`/`wallet_id`,
      `type` и `granularity` по белому списку, даты (`date_to` + 1 день),
      сброс фильтра при `DateFrom > DateTo`
- [x] сформировать ссылки-пресеты периода в данных страницы
- [x] загрузить списки категорий и кошельков для формы фильтра
- [x] зарегистрировать `GET /reports` через `h.protected` в `Routes`
- [x] добавить в `tmplFuncs` хелперы `jsonData` и `hasID`
- [x] написать табличный тест `parseReportFilter`: пустой query, несколько категорий,
      мусорные UUID, невалидный `type`/`granularity`, `date_to` + 1 день,
      `DateFrom > DateTo`
- [x] `go build ./... && go test ./...` — зелёное перед Task 9

### Task 9: Шаблон отчёта — фильтр, итоги, навигация

**Files:**
- Create: `web/templates/reports.html`
- Modify: `web/templates/base.html`
- Modify: `internal/handler/handler.go`

- [x] создать `reports.html` с блоком `content`: заголовок, карточки итогов,
      вывод `Notice`
- [x] сверстать форму фильтра `method="GET" action="/reports"` в `<details class="collapse">`
- [x] добавить ссылки-пресеты периода и поля дат с классом `fp-date`
- [x] добавить переключатель гранулярности с `disabled` для неподходящих вариантов
- [x] добавить чекбоксы категорий по типам, с «Выбрать все / Снять всё»
- [x] добавить мультивыбор кошельков и кнопки «Применить» / «Сбросить»
- [x] добавить блоки `head_extra` (flatpickr CSS) и `scripts` (flatpickr + локаль ru)
- [x] добавить пустое состояние, когда под фильтр не попало операций
- [x] **только теперь** добавить `"reports"` в список страниц `parseTemplates`
      (раньше — ошибка `parseTemplates` → `handler.New` → `os.Exit(1)`, сервер не стартует)
- [x] добавить пункт «Отчёты» в desktop-навигацию и в мобильное меню `base.html`
- [x] `go build ./... && go test ./...` — зелёное; рендер страницы проверен
      временным тестом на `parseTemplates` + `ExecuteTemplate` (полные и пустые
      данные); живой клик по UI — ручная проверка, вынесена в Post-Completion

### Task 10: Шаблон отчёта — графики и таблица

**Files:**
- Modify: `web/templates/reports.html`

- [x] отдать датасеты в JS одним литералом через `jsonData`, без вложенных `range`
- [x] добавить stacked bar по категориям (Chart.js, `stack: 'x'`) с пунктирной
      линией среднего — отдельно для расходов и доходов
- [x] скопировать `makeRoseChart` и `fmtMoney` из `wallet_stats.html`, построить
      пончики долей категорий за период
- [x] добавить таблицу: строки — периоды, столбцы — категории + «Итого» (+ «Net»
      при обеих секциях), футер — общий итог
- [x] обернуть таблицу в `overflow-x-auto`, скрыть второстепенные столбцы на мобильных
- [x] подключить `MutationObserver` по `data-theme` и скрипты Chart.js + ECharts
- [x] проверить отображение на узком экране (≤ 375px) и в обеих темах
      (проверено статически: `overflow-x-auto`, `hidden lg:table-cell`, адаптивные
      grid-классы, обе темы в `MutationObserver`; визуальная проверка — в Post-Completion)
- [x] `go build ./... && go test ./...` — зелёное перед Task 11

### Task 11: Verify acceptance criteria

Проверка сквозная: `internal/storage/report_e2e_test.go` поднимает реальный
PostgreSQL (testcontainers), гоняет `service.Report` поверх `storage.NewFromPool`
и сверяет итоги с прямыми SQL-запросами к той же выборке.

- [x] отчёт по одной категории за период — суммы совпадают с прямым SQL-запросом
      `SELECT SUM(amount) FROM operations WHERE ...` к той же БД
      (`TestReportE2E_AgainstDirectSQL/одна категория за период`)
- [x] отчёт по нескольким категориям — итог равен сумме категорий
      (`TestReportE2E_AgainstDirectSQL/несколько категорий…`: секция = сумма
      категорий = сумма бакетов = прямой SQL)
- [x] без указания периода — отчёт за всё время, гранулярность выбрана авто
      (`TestReportE2E_AgainstDirectSQL/без фильтров…`: 95 дней → `month`,
      `Notice` пуст; на моках — `TestReport_AutoRangeFromBounds`)
- [x] без указания категорий и кошельков — отчёт не пустой (проверка нормализации
      `nil`-слайсов на живом запросе) — тот же подтест, фильтр с `nil`-слайсами
- [x] периоды без операций присутствуют в графике и таблице с нулём
      (`TestReportE2E_AgainstDirectSQL/месяц без операций…`: март в сетке,
      `Total == 0`, длина `Amounts` совпадает с числом категорий; шаблон рендерит
      те же бакеты — визуальная проверка в Post-Completion)
- [x] переводы между кошельками не попадают в отчёт
      (`TestReportE2E_AgainstDirectSQL/переводы…` + `TestOperationStore_ReportRows_ExcludesTransfers`)
- [x] ручное переключение гранулярности перекрывает авто
      (`TestReportE2E_AgainstDirectSQL/ручная гранулярность…` + `TestReport_ManualGranularityOverridesAuto`)
- [x] > 8 категорий схлопываются в «Прочее» (`TestReportE2E_CollapsesToOther`:
      12 категорий → 8 + «Прочее», сумма сохранена; плюс `TestCollapseCategories`)
- [x] ручная правка URL (`granularity=day` на периоде в 5 лет) не вешает страницу
      (`TestReportFilterFromURL_LongPeriodWithDayStep`: `parseReportFilter` →
      `Service.Report` → шаг укрупнён до недели, `Labels` ≤ 366, `Notice` заполнен)
- [x] `go build ./...`
- [x] `go test ./...` — весь набор зелёный
- [x] e2e: в проекте отсутствуют — не применимо

### Task 12: [Final] Обновить документацию

- [x] дополнить `CLAUDE.md`: страница `/reports`, паттерн авто-гранулярности,
      где лежит логика отчёта
- [x] зафиксировать в `CLAUDE.md` правило «UTC с обеих сторон при сшивке
      Go-сетки и SQL-бакетов»
- [x] отметить, что суммы в отчёте не разделяются по валютам (осознанно)
- [x] переместить этот план в `docs/plans/completed/`

## Post-Completion

*Требует ручного вмешательства или внешних систем — без чекбоксов, информационно*

**Ручная проверка:**
- прогнать отчёт на реальных данных: «за всё время», «текущий месяц», «год»
- проверить читаемость stacked bar при 8 категориях и 12+ периодах
- проверить обе темы (светлая/тёмная) и мобильную вёрстку (Safari iOS — там уже
  ловили баг с dropdown, см. коммит 1443b5b)

**Индекс под фильтр по категориям (по результатам профилирования):**
Изначально планировался как задача, но при форме запроса
`cardinality($3) = 0 OR category_id = ANY($3)` планировщик в generic plan
не видит селективного предиката по `category_id`, а существующий
`operations_user_id_operated_at_idx` уже покрывает доступ «по пользователю за период».
Если на реальных данных отчёт начнёт тормозить — снять `EXPLAIN ANALYZE`
и решать по факту: либо индекс `(user_id, category_id, operated_at)`,
либо смена формы запроса на безусловный `= ANY($3)` с подстановкой всех
категорий пользователя, когда фильтр не задан (тогда план детерминирован).
`CREATE INDEX` без `CONCURRENTLY` блокирует запись в `operations`;
для CONCURRENTLY в goose нужен `-- +goose NO TRANSACTION`.

**Возможные следующие шаги (вне объёма):**
- вынести общий JS графиков в `web/static/charts.js` и убрать дублирование
  `makeRoseChart` между `wallet_stats.html` и `reports.html`
- экспорт отчёта в CSV
- сравнение с предыдущим периодом (дельта в процентах)
- разделение по валютам, если появятся мультивалютные сценарии
- сохранённые пресеты отчётов

## История ревизий

**Ревизия 2** — по результатам авто-ревью:
- исправлены три параметра SQL, ломавшие базовый сценарий «за всё время»:
  `nil`-слайсы → пустые, `Type ""` → `nil`, нулевые `time.Time` → `nil`
- добавлены алиасы колонок (без `AS amount` не собирается `pgx.RowToStructByName`)
- добавлена защита от NULLable `operated_at`
- регистрация страницы в `parseTemplates` перенесена из Task 5 в Task 9 —
  иначе между задачами сервер не стартует (`os.Exit(1)` в `main.go:53`)
- зафиксирован UTC с обеих сторон при сшивке Go-сетки и SQL-бакетов
- Task 2 разбита на две, Task 4 — на три; во все задачи добавлен гейт
  `go build ./... && go test ./...`
- добавлен табличный тест на `parseReportFilter`
- датасеты графиков отдаются через `jsonData`, а не ручной развёрткой шаблона
- задача с индексом перенесена в Post-Completion
- убраны `AutoChosen`, `MaxLabel`, `MaxTotal`, отдельный график Доходы/Расходы/Net
- `ReportBuckets` переименован в `ReportRows` (метод возвращает сырые строки)
- порог `month → year` поднят с 1096 до 1826 дней
- пресеты периода стали готовыми ссылками, без приоритета над `date_from`/`date_to`
