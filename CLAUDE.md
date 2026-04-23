# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Команды

```bash
# Сборка
go build ./...

# Запуск (нужны переменные окружения из .env.example)
go run ./cmd/server

# Тесты
go test ./...

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
- **`internal/service/`** — бизнес-логика: хеширование паролей (bcrypt), валидация (тип категории должен совпадать с типом операции).
- **`internal/handler/`** — net/http хендлеры. Роутинг через Go 1.22+ ServeMux с method-префиксами (`GET /wallets/{id}`).
- **`internal/session/`** — cookie-сессии (UUID в таблице `sessions`). Middleware `auth()` читает куку → ищет сессию в БД → кладёт `userID` в `context.Context`.
- **`web/`** — embed.FS с шаблонами и статикой. Каждая страница парсится отдельно: `base.html + page.html + partials/*.html`.
- **`migrations/`** — SQL-файлы goose v3. Миграции **не запускаются автоматически** — только вручную через `goose`.

## Frontend

htmx 2.x + Chart.js + Pico.css (CDN). Шаблоны рендерятся сервером. Партиалы — именованные `{{define "name"}}` блоки, возвращаются на htmx-запросы (inline edit, удаление строк таблицы).

## База данных

Таблицы: `users`, `sessions`, `wallets`, `categories`, `operations`. Поле `user_id` есть в каждой таблице кроме `users`. Суммы хранятся как `NUMERIC(15,2)`, в Go сканируются в `float64` (pgx v5 поддерживает это напрямую). В запросах с агрегатами используется явный `::float8`.
