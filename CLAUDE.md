# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Команды

```bash
# Сборка
go build ./...

# Запуск (нужны переменные окружения из .env.example)
go run ./cmd/server

# Тесты (internal/storage поднимает PostgreSQL через testcontainers — нужен Docker)
go test ./...

# Моки после правки internal/storage/iface.go
go generate ./internal/storage

# Docker (полный стек: приложение + PostgreSQL)
docker-compose up --build

# Миграции — только вручную
goose -dir migrations postgres "$DB_URL" up
goose -dir migrations postgres "$DB_URL" status
```

## Правила

- **Миграции никогда не запускаются автоматически при старте приложения.** Только вручную через `goose`. Не добавлять вызовы `goose` / `migrate` в `main()` или любой другой код инициализации.

## Архитектура

Слоёная архитектура без фреймворков:

```
handler → service → storage (pgxpool)
handler → session.Manager
```

- **`internal/storage/`** — pgx v5 репозитории. Каждый SQL-запрос обязательно содержит `WHERE user_id = $N` — это ключевая гарантия изоляции данных между пользователями.
- **`internal/service/`** — бизнес-логика: хеширование паролей (bcrypt), валидация (тип категории должен совпадать с типом операции). Ошибка ввода - `*service.ValidationError` с текстом для пользователя; хендлеры отдают её как 400 через `validationError(w, err)`.
- **`internal/handler/`** — net/http хендлеры. Роутинг через Go 1.22+ ServeMux с method-префиксами (`GET /wallets/{id}`).
- **`internal/session/`** — cookie-сессии (UUID в таблице `sessions`). Middleware `auth()` читает куку → ищет сессию в БД → кладёт `userID` в `context.Context`.
- **`web/`** — embed.FS с шаблонами и статикой. Каждая страница парсится отдельно: `base.html + page.html + partials/*.html`. Новая страница добавляется в список `pages` в `handler.parseTemplates`; шаблон создавать **до** регистрации — отсутствующий файл валит `handler.New` → `os.Exit(1)`, сервер не поднимется целиком.
- **`migrations/`** — SQL-файлы goose v3. Миграции **не запускаются автоматически** — только вручную через `goose`.

## Frontend

htmx 2.x + Chart.js + ECharts + DaisyUI (CDN). Шаблоны рендерятся сервером. Партиалы — именованные `{{define "name"}}` блоки, возвращаются на htmx-запросы (inline edit, удаление строк таблицы).

Данные для JS-графиков передаются хелпером шаблона `jsonData` (`{{jsonData .Report.Labels}}`): `json.Marshal` + `template.JS`. Ручную развёртку через `{{range}}` внутри JS-литерала не использовать. `template.JS` отключает контекстное экранирование, поэтому `jsonData` допустим только как значение внутри `<script>` — в атрибуте или `href` это XSS. `hasID .Filter.CategoryIDs .ID` — отметка выбранных чекбоксов в формах фильтров.

Строки, которые JS вставляет как HTML (formatter подсказок ECharts, тосты), идут через `escapeHTML` из `base.html`, иначе название категории становится XSS. Формы создания сбрасываются в `hx-on::after-request` только при `event.detail.successful`: ошибка 400 не должна стирать ввод.

Внешние скрипты и стили подключаются с точной версией и `integrity` + `crossorigin="anonymous"` (`TestExternalAssetsPinned`); хеш: `curl -s URL | openssl dgst -sha384 -binary | openssl base64 -A`. Исключения - `cdn.tailwindcss.com` и Google Fonts (динамический ответ).

Расширение базового шаблона: страницы могут определять `{{define "head_extra"}}` (добавляется в `<head>`, для CSS/шрифтов) и `{{define "scripts"}}` (добавляется перед `</body>`, для page-specific JS-библиотек). Тяжёлые скрипты (Chart.js, ECharts, Flatpickr) подключаются только в тех шаблонах, где они нужны.

## Тесты

- **`internal/storage/`** — интеграционные на testcontainers (`postgres:16-alpine` + goose-миграции в `TestMain`), каждый тест начинается с `cleanDB(t)`.
- **`internal/service/`** — gomock поверх интерфейсов из `internal/storage/iface.go`. Пакет теста выбирается по файлу целиком: `service_test.go` — внешний `package service_test`, `report_test.go` — внутренний `package service`, потому что там же лежат тесты неэкспортируемых хелперов (`truncate`, `bucketCount`, `reportGrid`) и дробить файл ради этого не стали.
- **`internal/handler/`** — чистые функции (парсинг фильтров, пресеты) и рендер шаблонов через `parseTemplates(web.FS)`; `Handler` конструируется для проверки стека middleware (`New(nil, nil, web.FS, Config{})`, до сервиса такие запросы не доходят) и для маппинга ошибок сервиса в HTTP-статусы: `service.NewWithRepos` поверх моков, хендлер вызывается напрямую с `userIDKey` в контексте (`validation_test.go`).
- `internal/storage/report_e2e_test.go` тестирует **service** поверх живой БД, но лежит в `package storage_test` — только там поднят PostgreSQL. Слои это не нарушает.

## База данных

Таблицы: `users`, `sessions`, `wallets`, `categories`, `operations`. Поле `user_id` есть в каждой таблице кроме `users`. С миграции 009 составные FK `operations (wallet_id | transfer_wallet_id, user_id)` и `(category_id, user_id, type)` не дают сослаться на чужой кошелёк или категорию и разойтись типу операции с типом категории; смена типа категории с операциями ловится и в SQL (`NOT EXISTS`), и этим FK (23503 → `ErrCategoryInUse`). Тест, которому нужна заведомо битая строка, вставляет её в обход FK (`insertBypassingFK`). Суммы — целые копейки: колонка `operations.amount_minor BIGINT`, в Go тип `model.Money` (int64). `float64` для денег не использовать. `SUM(bigint)` в PostgreSQL возвращает `numeric`, поэтому в агрегатах явный `::int8`. Пользовательский ввод разбирается `model.ParseMoney` (запятая или точка, пробелы между тысячами), в шаблонах — хелперы `money` (с разделителями) и `num` (для `value` и JS); в JSON `Money` сериализуется в рублях (`1234.56`), так что графики получают привычные числа.

Перевод — две операции с общим `transfer_id`, у каждой сумма в валюте своего кошелька. Между разными валютами обязательна сумма зачисления (`to_amount`), в одной валюте сервис приравнивает её к списанию. Итоговый баланс на дашборде (`TotalBalances`) считается отдельно по каждой валюте.

## Отчёт по категориям (`/reports`)

Страница расходов/доходов по выбранным категориям за период. Логика разнесена по слоям: `internal/storage/report.go` (SQL-агрегация `ReportBounds`/`ReportRows`), `internal/service/report.go` (сетка бакетов, лейблы, top-8 категорий + «Прочее»), `internal/handler/report.go` (парсинг фильтра, пресеты периода), шаблон `web/templates/reports.html`.

- **Авто-гранулярность** по длине периода: ≤ 31 дня — `day`, ≤ 92 — `week`, ≤ 1826 (~5 лет) — `month`, дальше — `year`. Ручной выбор перекрывает авто; если выбранный шаг даёт > 366 бакетов, сервер укрупняет шаг и заполняет `Report.Notice`. Число бакетов считается арифметически (`bucketCount`) **до** построения сетки — иначе диапазон из сочинённого URL разворачивается в миллионы элементов; на `year` укрупнять уже некуда, поэтому диапазон сначала сужается до фактических границ данных (`clampToBounds`, только в этом случае — иначе явно заданный период показывается целиком, с нулями по краям), а если и это не помогло — обрезается до последних 366 бакетов. Если на сетку не легла ни одна строка, `Report.Labels` остаётся пустым: шаблон рисует пустое состояние, а не каркас без графиков.
- **UTC с обеих сторон при сшивке Go-сетки и SQL-бакетов.** `date_trunc` считает в таймзоне сессии, а Go может оказаться в `time.Local`; при расхождении часть сумм молча попадёт не в свой бакет. Поэтому в SQL всегда `date_trunc($n, operated_at AT TIME ZONE 'UTC')`, а в Go `truncate` работает по `t.UTC()`.
- **Top-8 категорий + «Прочее»** — защита от радуги в легенде, когда фильтр по категориям не задан. Если пользователь выбрал категории явно, показываются все выбранные, но не больше 14 (плюс «Прочее» — ровно палитра графиков в 15 цветов): «Выбрать все» иначе даёт нечитаемый график.
- **Суммы не разделяются по валютам** — `amount_minor` складывается как есть. Осознанное решение для графиков отчёта; итоговый баланс на дашборде, в отличие от отчёта, по валютам разделён.
