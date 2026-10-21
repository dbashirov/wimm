# План подготовки к продакшену

Статус задач: ✅ — сделано, ❌ — открыто, ⏸ — отложено.
Приоритеты: `🔴 Критично` → `🟠 Важно` → `🟡 Желательно` → `🟢 Nice to have`.

---

## 🔴 Критично (блокирует деплой)

### ✅ 1. HTTP-сервер таймауты + graceful shutdown
`http.Server` с `ReadHeaderTimeout/ReadTimeout/WriteTimeout/IdleTimeout/MaxHeaderBytes`.
`signal.NotifyContext(SIGINT, SIGTERM)` → `srv.Shutdown` → `store.Close`.
`time.Tick` заменён на `time.NewTicker` + `<-ctx.Done()`.

### ✅ 2. CSRF-защита
Stateless double-submit cookie: `token = HMAC-SHA256(sessionID, CSRF_SECRET)`,
кладётся в cookie `csrf_token`. htmx через `htmx:configRequest` шлёт его
в заголовке `X-CSRF-Token` на каждом мутирующем запросе.

### ✅ 3. Rate-limiting на /login и /register
Token bucket per-IP (`golang.org/x/time/rate`): 5 burst + 1/мин refill.
Очистка устаревших записей раз в 10 минут.

### ✅ 4. Timing-safe Login
Если пользователя нет — всё равно вызывается `bcrypt.CompareHashAndPassword`
с фиктивным хешем (генерируется в `init()`).

### ✅ 5. Логирование ошибок в хендлерах
Хелпер `h.serverError(w, r, op, err)` пишет лог + 500.
Replaced все `http.Error(500)` без логов.

### ✅ 6. Recovery middleware
`recoverPanic` ловит панику, логирует stack trace в slog, возвращает 500.

---

## 🟠 Важно

### ✅ 7. Security headers
CSP с whitelist'ом CDN (jsdelivr/unpkg/npmcdn/cdn.tailwindcss.com/fonts.googleapis.com),
`X-Frame-Options: DENY`, `X-Content-Type-Options: nosniff`,
`Referrer-Policy`, `Permissions-Policy`. HSTS опционально через `HSTS=true`.

### ✅ 8. Body size limit
`limitBody(1 << 20)` middleware → `http.MaxBytesReader` 1 MB.

### ✅ 9. Static FileServer (fs.Sub)
`fs.Sub(web.FS, "static")` — шаблоны больше не торчат через `/static/templates/...`

### ✅ 10. time.Tick → time.NewTicker
Сделано в составе пункта 1.

### ✅ 11. Email-валидация
`net/mail.ParseAddress` в `service.Register`, нормализация (lowercase + trim)
в `Register` и `Login`. Ошибка `ErrInvalidEmail`.

### ✅ 12. pgxpool config
`MaxConns=25`, `MinConns=2`, `MaxConnLifetime=1h`, `MaxConnIdleTime=30m`,
`HealthCheckPeriod=1m`.

### ✅ 13. DB query timeouts
Middleware `requestTimeout(15s)` ставит общий дедлайн на запрос —
наследуется во все вызовы pgx через `r.Context()`.

### ✅ 14. Healthcheck endpoints
- `GET /health` — liveness, всегда 200 OK
- `GET /ready` — readiness, пингует БД с таймаутом 2s

### ✅ 15. Удалён dead code `getDashboardJSON`

### ✅ 16. slog — структурированное логирование
- `LOG_LEVEL` (debug/info/warn/error) и `LOG_FORMAT` (text/json) из env
- Все хендлеры и middleware пишут структурированно: `method`, `path`,
  `op`, `err`, `duration_ms`, `ip`, `status`
- `http.Server.ErrorLog` перенаправлен в slog

### ❌ 17. Account enumeration на регистрации
Ошибка "Email уже зарегистрирован" позволяет перечислять пользователей.
Спорный пункт UX vs security. Для финансового сервиса в идеале —
"Если email не зарегистрирован, мы пришлём письмо для подтверждения"
(требует email-flow). **Пока решено оставить как есть.**

---

## 🟡 Желательно

### ✅ 18. Session ID на crypto/rand
Теперь 32 байта в base64 RawURL (43 символа). Миграция 005 меняет
`sessions.id` с UUID на TEXT.

### ✅ 19. Тайм-зоны → UTC
`parseDateTime` парсит в UTC. `Dockerfile` задаёт `TZ=UTC` для
консистентности `time.Now()` и пр. **Известное ограничение:** клиент
не передаёт TZ, поэтому "12:00" воспринимается как 12:00 UTC. Для
мультизоны нужен явный TZ от клиента.

### ✅ 20. JS template injection (потенциально)
В `dashboard.html` и `wallet_stats.html` данные разворачиваются вручную:
```js
[{{range .CategoryExpenses}}"{{.Name}}",{{end}}]
```
Go's `html/template` делает контекстный экранинг для `<script>`, поэтому
дыры здесь нет, но конструкция хрупкая. Принятая в кодовой базе замена —
хелпер шаблона `jsonData` (`json.Marshal` + `template.JS`, см. `reports.html`),
а не `data-*` + `JSON.parse`, как предлагала первая редакция аудита.
`dashboard.html`, `wallet_stats.html` и категории формы в `operations.html`
переведены на `jsonData`.

### ✅ 21. Pagination upper bound
`parsePage` ограничивает номер страницы 10000: `page=MaxInt` переполнял
расчёт OFFSET и давал 500. Заодно ссылки пагинации сохраняют фильтр.

### ✅ 22. Редактирование типа категории
Тип меняется в форме редактирования, пока по категории нет операций
(иначе 409). Инвариант держит и SQL (`NOT EXISTS`), и FK из миграции 009.

### ✅ 23. Индексы по user_id
Миграция 004: `wallets_user_id_idx`, `categories_user_id_idx`.

### ✅ 24. migrations.go проверка
Только embed SQL-файлов (для тестов через goose). Никаких автовызовов.
Противоречий с CLAUDE.md нет.

### ✅ 25. .env.example задокументирован
`DB_URL`, `SERVER_ADDR`, `CSRF_SECRET`, `COOKIE_INSECURE`, `HSTS`,
`LOG_LEVEL`, `LOG_FORMAT`.

### ✅ 26. Описания operations в шаблонах
Обрезанное описание целиком в `title`, на узком экране выводится строкой
под суммой с переносом.

---

## 🟢 Nice to have

- ❌ Метрики Prometheus (`/metrics`)
- ✅ Request ID middleware: `X-Request-ID` в ответе, `request_id` в логах
- ❌ Audit log (кто что когда)
- ❌ Password reset flow
- ❌ Email-верификация при регистрации
- ❌ 2FA
- ❌ Backup-стратегия для PostgreSQL
- 🟡 CI: `go vet` и `go test -race` перед деплоем добавлены; `golangci-lint`,
  `gosec`, `govulncheck` - нет (сначала прогнать на текущем коде, иначе
  незнакомые находки заблокируют автодеплой)

---

## 🔐 Аудит безопасности 2026-10-21

Независимый аудит (Claude + Codex), исправления в ветке `fix/overnight-audit`.

| Серьёзность | Находка | Статус |
|---|---|---|
| high | Stored XSS: название категории попадало в HTML подсказок ECharts (dashboard, статистика кошелька, отчёт) | ✅ `escapeHTML` |
| medium | Login/register CSRF: CSRF-токен привязан к сессии, у входа её нет | ✅ `http.CrossOriginProtection` |
| medium | `/wallets/reorder`: UPDATE на каждый id без лимита (~26 тыс. запросов на 1 MB тела) | ✅ один UPDATE, лимит 1000, без повторов |
| low | Тост вставлял текст ответа сервера через `innerHTML` | ✅ экранирование |
| low | Выход при сбое БД оставлял cookie и делал вид, что всё прошло | ✅ cookie чистится всегда, ошибка 500 |
| low | `/ready` отдавал текст ошибки драйвера БД | ✅ только в лог |
| low | Переполнение OFFSET при огромном `page` | ✅ см. пункт 21 |
| low | Невалидный ввод (валюта, тип, пустое имя, пароль > 72 байт) давал 500, длины не ограничены | ✅ `service.ValidationError` -> 400 |
| low | JOIN-ы и подзапросы операций искали кошелёк/категорию только по id | ✅ условие владельца + FK миграции 009 |
| low | CDN-скрипты без точных версий и SRI | ✅ кроме Tailwind CDN и Google Fonts |
| low | Перечисление email на регистрации | ❌ принятый риск, см. пункт 17 |

Попутно найдены и исправлены не связанные с безопасностью баги: пагинация
сбрасывала фильтр, сохранение категории затирало описание, `putWallet`/
`putCategory` отвечали 500 на чужой id, формы очищались при ошибке 400.

---

## 📈 Известные риски производительности

### Отчёт по категориям (`/reports`)
`ReportBounds`/`ReportRows` фильтруют категории и кошельки формой
`cardinality($n::uuid[]) = 0 OR col = ANY($n::uuid[])` — при такой записи
планировщик не видит селективного предиката и берёт
`operations_user_id_operated_at_idx` с последующей фильтрацией.
Отдельный индекс по `category_id` намеренно не добавлялся: на реальных
объёмах одного пользователя это дешевле, чем лишний индекс на запись.
Если отчёт начнёт тормозить — снимать `EXPLAIN ANALYZE` на живых данных,
а не гадать.

---

## 🎯 Готовность к продакшену

**MVP-prod готов** — всё критическое и почти всё важное закрыто.
Можно деплоить за reverse-proxy с HTTPS (Caddy/nginx).

### Чек-лист перед деплоем:
- [ ] Миграция 009: сначала три проверочных запроса из её шапки (должны
      вернуть 0), затем `goose up` в тихое время (блокирует запись в
      operations/wallets/categories на время проверки FK). Код работает и без неё
- [ ] Сгенерировать стабильный `CSRF_SECRET`: `openssl rand -base64 32`
- [ ] `COOKIE_INSECURE=` (пусто = Secure)
- [ ] `HSTS=true`
- [ ] `LOG_FORMAT=json`, `LOG_LEVEL=info`
- [ ] Запустить миграции: `goose -dir migrations postgres "$DB_URL" up`
- [ ] Настроить healthcheck: `/health` для liveness, `/ready` для readiness
- [ ] Настроить TLS на reverse-proxy
- [ ] Backup-стратегия для PostgreSQL

### Что доделать после первого деплоя (по убыванию полезности):
1. Сборка Tailwind CSS вместо `cdn.tailwindcss.com` (`TODO_TAILWIND_BUILD.md`):
   уберёт последний скрипт без SRI и `'unsafe-eval'` из CSP
2. **17** — account enumeration mitigation (опционально)
3. Метрики Prometheus
4. CI с linters (`golangci-lint`, `gosec`, `govulncheck`)
