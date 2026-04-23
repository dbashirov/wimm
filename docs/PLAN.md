# План: Сервис личных финансов "wimm"

## Context
Строим с нуля веб-сервис учёта личных финансов. Репозиторий пустой.
Требования: чистый Go + net/http, htmx-фронт, PostgreSQL, multi-валюта, дашборд с графиками, Docker-деплой, строгая изоляция данных между пользователями.

---

## Структура проекта

```
wimm/
├── cmd/server/main.go          # точка входа, сборка зависимостей
├── internal/
│   ├── model/                  # доменные типы (User, Wallet, Category, Operation, Session)
│   ├── storage/                # pgx-репозитории (один файл на сущность)
│   ├── service/                # бизнес-логика, вычисление balance
│   ├── handler/                # net/http хендлеры + middleware
│   └── session/                # cookie-сессии (хранение в БД)
├── migrations/                 # SQL-миграции (goose)
├── web/
│   ├── templates/              # html/template шаблоны
│   └── static/                 # htmx.min.js, chart.js, style.css
├── Dockerfile
├── docker-compose.yml
└── CLAUDE.md
```

---

## База данных (PostgreSQL)

```sql
-- users
id BIGSERIAL PK, email UNIQUE NOT NULL, password_hash TEXT, created_at TIMESTAMPTZ

-- sessions
id UUID PK, user_id → users(id) ON DELETE CASCADE, expires_at TIMESTAMPTZ
-- индекс на expires_at для очистки протухших сессий

-- wallets
id BIGSERIAL PK, user_id → users(id), name TEXT, currency CHAR(3), created_at TIMESTAMPTZ

-- categories
id BIGSERIAL PK, user_id → users(id), name TEXT, type VARCHAR(7) CHECK(type IN ('income','expense'))

-- operations
id BIGSERIAL PK, user_id → users(id), wallet_id → wallets(id),
category_id → categories(id), type VARCHAR(7), amount NUMERIC(15,2),
operated_at DATE NOT NULL, description TEXT, created_at TIMESTAMPTZ
```

**Важно:** `user_id` есть в каждой таблице. Все SELECT/UPDATE/DELETE обязательно включают `WHERE user_id = $N` — это главная гарантия изоляции.

---

## Аутентификация и сессии

- Пароли: `bcrypt` (cost=12)
- Сессия: UUID в cookie `session_id` (HttpOnly, SameSite=Strict), запись в таблице `sessions`
- Middleware `requireAuth`: читает cookie → ищет сессию в БД → кладёт `userID` в `context.Context`
- Истечение сессии: 30 дней, фоновая горутина чистит протухшие записи раз в час

---

## HTTP-маршруты (net/http ServeMux Go 1.25+)

```
GET/POST  /register
GET/POST  /login
POST      /logout

GET       /                     → редирект на /dashboard
GET       /dashboard            → сводка + графики (Chart.js через htmx)

GET       /wallets              → список
POST      /wallets              → создать (htmx → вставить строку в таблицу)
GET       /wallets/{id}/edit    → форма редактирования (htmx partial)
PUT       /wallets/{id}         → сохранить
DELETE    /wallets/{id}         → удалить (htmx → удалить строку)

/categories/*  — аналогично (с фильтром ?type=income|expense)
/operations/*  — аналогично + GET /operations с query-параметрами фильтрации
```

Go 1.25 поддерживает `{id}` в паттернах и method-routing (`GET /foo`, `POST /foo`) — фреймворк не нужен.

---

## Frontend

- **htmx**: inline-редактирование, удаление без перезагрузки, частичные обновления списков и форм
- **Chart.js**: на дашборде — pie/doughnut расходы по категориям, bar доходы vs расходы по месяцам. Данные отдаются через отдельный JSON-эндпоинт `/dashboard/data`
- **Шаблоны**: `html/template` с `base.html` (layout) + страничные шаблоны + partials для htmx-ответов
- **CSS**: Pico.css (classless, минималистичный, ~10 KB) — никакого Tailwind/Bootstrap, легко кастомизировать

---

## Слои и зависимости

```
handler → service → storage (pgx)
handler → session.Manager
```

- `storage` знает только про SQL и pgx, возвращает доменные модели и ошибки
- `service` содержит бизнес-правила (например, пересчёт баланса кошелька при удалении операции)
- `handler` переводит HTTP ↔ service, рендерит шаблоны

**Нет глобальных переменных.** Все зависимости инжектируются через конструкторы в `main.go`.

---

## Конфигурация

Через переменные окружения (без сторонних либ):
```
DB_URL, SERVER_ADDR, SESSION_SECRET, SESSION_TTL_DAYS
```

---

## Зависимости (go.mod)

| Пакет | Зачем |
|---|---|
| `github.com/jackc/pgx/v5` | PostgreSQL-драйвер |
| `github.com/pressly/goose/v3` | SQL-миграции |
| `golang.org/x/crypto` | bcrypt |

Всё остальное — стандартная библиотека Go.

Обязательно использовать Go 1.25!

---

## Docker

- `Dockerfile`: multi-stage build (golang:1.25-alpine → scratch/alpine)
- `docker-compose.yml`: сервисы `app` + `postgres`, volume для данных, `.env`-файл для секретов
- Миграции запускаются автоматически при старте `app` (goose up в main.go перед Listen)

---

## Порядок реализации

1. `go mod init` + структура папок
2. SQL-миграции (`migrations/`)
3. Доменные модели (`internal/model/`)
4. Слой хранилища (`internal/storage/`) — pgx-репозитории с `user_id` в каждом запросе
5. Сессии (`internal/session/`)
6. Сервисный слой (`internal/service/`)
7. Хендлеры + middleware (`internal/handler/`)
8. HTML-шаблоны + статика (`web/`)
9. `cmd/server/main.go` — сборка и запуск
10. `Dockerfile` + `docker-compose.yml`

---

## Проверка

```bash
docker-compose up --build
# Открыть http://localhost:8080
# 1. Зарегистрировать двух пользователей
# 2. Убедиться, что операции одного не видны другому
# 3. Проверить dashboard с графиками
# 4. go test ./...
```
