# План подготовки к продакшену

Аудит проведён по текущему состоянию `main`. Приоритеты: `🔴 Критично` →
`🟠 Важно` → `🟡 Желательно` → `🟢 Nice to have`.

---

## 🔴 Критично (блокирует деплой)

### 1. HTTP-сервер без таймаутов и graceful shutdown
**Где:** `cmd/server/main.go`

Используется `http.ListenAndServe` без настройки. Уязвимо к slowloris,
при SIGTERM запросы обрываются, пул БД не закрывается корректно.

**Что делать:**
```go
srv := &http.Server{
    Addr:              addr,
    Handler:           mux,
    ReadHeaderTimeout: 5 * time.Second,
    ReadTimeout:       30 * time.Second,
    WriteTimeout:      30 * time.Second,
    IdleTimeout:       120 * time.Second,
    MaxHeaderBytes:    1 << 20,
}

ctx, stop := signal.NotifyContext(context.Background(),
    syscall.SIGINT, syscall.SIGTERM)
defer stop()

go func() {
    if err := srv.ListenAndServe(); err != nil &&
        !errors.Is(err, http.ErrServerClosed) {
        log.Fatalf("ListenAndServe: %v", err)
    }
}()

<-ctx.Done()
shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()
if err := srv.Shutdown(shutdownCtx); err != nil {
    log.Printf("Shutdown: %v", err)
}
store.Close()
```

### 2. CSRF-защита отсутствует
**Где:** все POST/PUT/DELETE-эндпоинты

Все мутирующие эндпоинты уязвимы. `SameSite=Strict` помогает частично,
но не во всех браузерах/контекстах.

**Что делать:** добавить middleware с double-submit cookie или
подключить `gorilla/csrf`. Токен встроить в шаблоны через `{{.CSRFToken}}`.

### 3. Brute force на `/login`
**Где:** `internal/handler/auth.go`

Нет rate-limiting → можно перебирать пароли неограниченно.

**Что делать:** middleware с in-memory rate limiter (`golang.org/x/time/rate`)
или Redis для распред. деплоя. Минимум: 5 попыток на 15 минут на пару IP+email.

### 4. Timing attack на `/login`
**Где:** `internal/service/service.go: Login()`

Если пользователя нет — `bcrypt.CompareHashAndPassword` не вызывается →
быстрый ответ. Можно перечислять зарегистрированных юзеров по времени.

**Что делать:**
```go
func (s *Service) Login(ctx context.Context, email, password string) (*model.User, error) {
    user, err := s.users.GetByEmail(ctx, email)
    if err != nil {
        // Сравниваем пароль с фиктивным хешем чтобы выровнять время
        bcrypt.CompareHashAndPassword(dummyHash, []byte(password))
        return nil, ErrInvalidCredentials
    }
    if err := bcrypt.CompareHashAndPassword(
        []byte(user.PasswordHash), []byte(password),
    ); err != nil {
        return nil, ErrInvalidCredentials
    }
    return user, nil
}
```

### 5. Логи ошибок не пишутся в хендлерах
**Где:** все файлы в `internal/handler/`

Все хендлеры делают `http.Error(w, "internal error", 500)` без
`log.Printf`. В продакшене будет невозможно дебажить.

**Что делать:** middleware-логгер + перед каждым `http.Error(500)` писать
`log.Printf("handler X: %v", err)`. Лучше — структурированный slog
с request_id.

### 6. Нет recovery middleware
**Где:** `internal/handler/middleware.go`

Паника в любом хендлере роняет весь сервер.

**Что делать:**
```go
func recover(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        defer func() {
            if v := recover(); v != nil {
                log.Printf("PANIC %s %s: %v\n%s",
                    r.Method, r.URL.Path, v, debug.Stack())
                http.Error(w, "internal error", 500)
            }
        }()
        next.ServeHTTP(w, r)
    })
}
```

---

## 🟠 Важно (исправить до прода)

### 7. Security headers отсутствуют
Нужны:
- `Content-Security-Policy` (с поправкой на CDN htmx/Chart.js/daisyui)
- `X-Content-Type-Options: nosniff`
- `Strict-Transport-Security` (если HTTPS)
- `Referrer-Policy: strict-origin-when-cross-origin`
- `X-Frame-Options: DENY` (или `frame-ancestors 'none'` в CSP)

### 8. Body size не лимитирован
`r.FormValue` читает всё в память. DoS через большой POST.

**Что делать:** middleware с `r.Body = http.MaxBytesReader(w, r.Body, 1<<20)`.

### 9. Static FileServer слишком широкий
**Где:** `cmd/server/main.go`

```go
mux.Handle("GET /static/", http.FileServer(http.FS(web.FS)))
```

Отдаёт весь `web/`, включая шаблоны (если кто-то знает путь).

**Что делать:**
```go
staticFS, _ := fs.Sub(web.FS, "static")
mux.Handle("GET /static/",
    http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))
```

### 10. Утечка горутины с `time.Tick`
**Где:** `cmd/server/main.go`

```go
go func() {
    for range time.Tick(time.Hour) { ... }
}()
```

`time.Tick` нельзя остановить, ticker не GC-ится.

**Что делать:**
```go
ticker := time.NewTicker(time.Hour)
defer ticker.Stop()
for {
    select {
    case <-ctx.Done(): return
    case <-ticker.C: sm.CleanExpired(ctx)
    }
}
```

### 11. Email-валидация отсутствует
**Где:** `internal/service/service.go: Register()`

Любая строка проходит как email.

**Что делать:** `net/mail.ParseAddress(email)` в Register.

### 12. Connection pool не настроен
**Где:** `internal/storage/storage.go`

`pgxpool.New(ctx, dsn)` берёт дефолты. Под нагрузкой нужно явно
указать `MaxConns`, `MinConns`, `MaxConnLifetime`.

```go
cfg, err := pgxpool.ParseConfig(dsn)
cfg.MaxConns = 25
cfg.MinConns = 2
cfg.MaxConnLifetime = time.Hour
cfg.MaxConnIdleTime = 30 * time.Minute
pool, err := pgxpool.NewWithConfig(ctx, cfg)
```

### 13. DB запросы без таймаутов
Контекст реквеста наследуется, но отдельных deadline для запросов нет.
Долгие запросы могут забить пул.

**Что делать:** `context.WithTimeout(ctx, 5*time.Second)` в storage-методах,
особенно в `DashboardData` и `List`.

### 14. Нет healthcheck-эндпоинта
Для k8s/load-balancer-а нужен:
- `GET /health` — живёт ли процесс
- `GET /ready` — готов БД пинговать

### 15. `getDashboardJSON` — dead code
**Где:** `internal/handler/dashboard.go:26`

Объявлен, но не зарегистрирован в роутере. Либо удалить, либо подключить.

### 16. Логирование примитивное
`log.Printf` без уровней, без структуры, без request_id.

**Что делать:** перейти на `log/slog` (std lib с 1.21) с JSON-handler-ом
для прода.

### 17. Account enumeration на регистрации
**Где:** `internal/handler/auth.go: postRegister`

Ошибка "Email уже зарегистрирован" → можно перечислять. Спорно
(UX vs security), но в банковских/финансовых сервисах лучше скрыть и
писать что-то вроде "Если email не зарегистрирован, мы пришлём
письмо для подтверждения".

---

## 🟡 Желательно

### 18. Session ID — UUIDv4
122 бита энтропии достаточно, но best practice — `crypto/rand` 32 байта
в base64.

### 19. Тайм-зоны
**Где:** `internal/handler/operation.go: parseDateTime`

Используется `time.Local` → зависит от TZ контейнера. Лучше форсить
UTC или передавать TZ от клиента.

### 20. JS template injection (потенциально)
**Где:** `web/templates/dashboard.html`

```js
[{{range .CategoryExpenses}}"{{.Name}}",{{end}}]
```

Go's `html/template` делает контекстный экранинг для `<script>`, но
желательно явно использовать функцию `js` или передавать данные через
`data-*` атрибуты + `JSON.parse`.

### 21. Pagination без верхней границы
`Page=999999` пройдёт через `OFFSET 49999950` — медленный запрос.
Ограничить, скажем, 10000.

### 22. Категории нельзя редактировать тип
`Update` принимает только name/description. Возможно намеренно, но если
пользователь ошибся — придётся удалить и пересоздать.

### 23. Индексы по `user_id`
**Где:** `migrations/001_init.sql`

`wallets.user_id`, `categories.user_id` не имеют индексов. Все запросы
фильтруют по `user_id` — нужны индексы.

```sql
CREATE INDEX wallets_user_id_idx ON wallets(user_id);
CREATE INDEX categories_user_id_idx ON categories(user_id);
```

### 24. `migrations/migrations.go` противоречит CLAUDE.md?
По правилам миграции запускаются только вручную, но в папке есть
`migrations.go`. Проверить и убрать вызовы из main, если есть.

### 25. `.env.example` неполный
Нет `COOKIE_INSECURE`, `MAX_DB_CONNS`, и т.д. Документировать все
переменные.

### 26. Описания operations не показываются в выдаче
Запрос делает JOIN, но в шаблонах `operation_row` не везде
используется. Проверить.

---

## 🟢 Nice to have

- Метрики Prometheus (`/metrics`)
- Request ID middleware (для трейсинга)
- Audit log (кто что когда)
- Password reset flow
- Email-верификация при регистрации
- 2FA
- Backup-стратегия для PostgreSQL
- CI с `go test -race`, `golangci-lint`, `gosec`

---

## 🎯 Минимальный набор для безопасного прод-деплоя

Порядок реализации для MVP-prod:

1. **HTTP server timeouts + graceful shutdown** (#1)
2. **Recovery middleware** (#6)
3. **Логирование ошибок в хендлерах** (#5)
4. **Rate-limiting на /login и /register** (#3)
5. **Timing-safe login** (#4)
6. **CSRF-токены** (#2)
7. **Security headers** (#7)
8. **MaxBytesReader** (#8)
9. **pgxpool config + DB query timeouts** (#12, #13)
10. **Healthcheck endpoint** (#14)

С этим списком уже можно ставить в прод за reverse-proxy с HTTPS
(Caddy/nginx).
