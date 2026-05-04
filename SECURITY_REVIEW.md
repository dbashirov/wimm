# Ревью проекта wimm: уязвимости и ошибки

Архитектура и базовая гигиена — на хорошем уровне (CSRF, bcrypt cost=12, timing-equalization при логине, body-limit, security headers, изоляция `WHERE user_id`, UUID для session_id из crypto/rand). Но есть конкретные проблемы.

## 🔴 Высокая важность

**1. `clientIP` слепо доверяет `X-Forwarded-For` / `X-Real-IP` — обход rate-limit на логине/регистрации**
`internal/handler/middleware.go:218-233`. Сервер слушает `:8080` напрямую, без обязательного reverse-proxy. Любой клиент может в каждом запросе подставлять случайный `X-Forwarded-For` и получать свежий token-bucket → лимит «5 попыток в минуту» (`handler.go:96`) полностью обходится → bruteforce паролей.

Фикс: либо принимать XFF только если `r.RemoteAddr` принадлежит белому списку доверенных прокси (env `TRUSTED_PROXIES`), либо при отсутствии доверенного прокси использовать только `r.RemoteAddr`.

**2. `/static/*` отдаётся в обход всего middleware — нет security headers**
`cmd/server/main.go:62-65` — `staticFS` повешен на внешний `mux` напрямую, а `securityHeaders`, `recoverPanic`, `logRequests` применяются только внутри `h.Routes()`. На статику не уезжают `X-Content-Type-Options: nosniff`, `X-Frame-Options`, CSP, HSTS. CSP «default-src 'self'» обходится для `/static/*`. Также `/static/` не учитывается в логах.

Фикс: либо завернуть `staticFS` в те же middleware, либо выставлять статику внутри `Routes()` с теми же обёртками.

**3. `POST /logout` не защищён CSRF и не требует `auth`**
`handler.go:144`, маршрут зарегистрирован вне `protected`. Сторонний сайт может разлогинить любого пользователя (logout-CSRF). Низкая severity, но тривиально чинится: добавить CSRF-валидацию и `auth`.

## 🟡 Средняя важность

**4. CSRF-токен = `HMAC(sessionID, secret)` — статичен на всё время жизни сессии (30 дней)**
`csrf.go:47-51`. При утечке токена (Referer, история браузера, кэш прокси) он валиден до истечения сессии. Для double-submit это допустимо, но желательно ротировать токен (в т.ч. на каждый логин/смену пароля) и привязывать к time-bucket'у.

**5. Нет ротации session_id при логине — потенциальная session fixation**
`session.go:26-41`. `Create` всегда генерирует новый ID и перезаписывает cookie — это неплохо. Но: старая сессия в БД не удаляется (есть `Delete`, но он не вызывается). И при смене пароля все старые сессии остаются валидны (а endpoint смены пароля и вовсе отсутствует).

**6. Отсутствует целостность `user_id` на уровне БД для `operations`**
`migrations/001_init.sql:44-55`. `wallet_id`/`category_id` — это FK на `wallets/categories`, но нет гарантии, что `operations.user_id == wallets.user_id` и `== categories.user_id`. Сейчас это валидируется только в service-слое (`service.go:257-273`); любая будущая ошибка в проверке открывает возможность создать операцию с чужим кошельком.

Фикс — composite FK: `FOREIGN KEY (wallet_id, user_id) REFERENCES wallets(id, user_id)` (потребуется уникальный индекс по `(id, user_id)` в `wallets`/`categories`).

**7. `operations.operated_at` объявлен NULLABLE, но код этого не предполагает**
`migrations/001_init.sql:53`. Запросы `date_trunc('month', operated_at)` падут или дадут странные группы при NULL. Сейчас handler всегда заполняет, но БД не гарантирует. Делайте `NOT NULL`.

**8. `WalletStore.SetDefault` — race-condition между параллельными запросами**
`wallet.go:180-219`. Сначала снимает `is_default` у предыдущего, потом ставит у нового. При двух параллельных запросах одного пользователя возможна гонка, ловится partial unique index `wallets_one_default_per_user` ошибкой `23505`, которая в `service.go` не маппится в специальный код → клиент получит 500 без понятного сообщения. Решение: `SELECT … FOR UPDATE` или маппинг 23505 в осмысленный конфликт.

**9. CSP допускает `'unsafe-inline'` и `'unsafe-eval'`**
`middleware.go:166-177`. Сильно ослабляет защиту от XSS. Для htmx можно перейти на nonce-based CSP (`script-src 'nonce-…'`) и хостить CDN-ресурсы у себя или с интегрити-хешами (SRI). Также `script-src` содержит сразу 4 CDN — это лишние векторы поставочной атаки.

## 🟢 Незначительные

**10. Регистрация: rate-limit и валидация пароля минимальны.** `auth.go:54` — только длина ≥ 8. Нет проверки на простые пароли/совпадение с email/максимальную длину (1MB body — единственный лимит).

**11. Реджистрация и логин используют один и тот же `loginRL` (5/мин на IP)** — учитывая (1), это легко обходится. После фикса (1) лимит станет реальным.

**12. Валидация `Currency` и `Type` на уровне сервиса отсутствует.** Невалидные значения уйдут в БД и вернутся 500 (`CHAR(3)` / enum). Лучше валидировать заранее.

**13. `getCategoryOptions` пишет вручную HTML через `fmt.Fprintf`.** Сейчас безопасно (UUID + `template.HTMLEscapeString` для имени), но хрупко: легко в будущем сломать. Лучше через `text/template` или `html/template`.

**14. `auth` redirect'ит на `/login` на любых XHR/HTMX-запросах (PUT/DELETE).** Не уязвимость, но плохое UX: htmx получит 302 и не покажет внятного сообщения. Для htmx-запросов лучше отдавать 401 с `HX-Redirect: /login`.

**15. `Wallets.Reorder` не лимитирует размер `ids`** — body-limit 1 MB защищает, но множитель N UPDATE можно ограничить разумным числом.

**16. UUID в БД — `gen_random_uuid()` (v4)**, тогда как правило в CLAUDE.md предписывает UUIDv7 для лучшей B-tree локальности. Можно перейти на `uuid.Must(uuid.NewV7())` в Go и убрать `DEFAULT gen_random_uuid()` (или поставить расширение для v7).

**17. `_ = tx.Rollback(ctx)` без `errcheck`-комментария дважды** в `operation.go:206,300` — приемлемо, отметка `//nolint:errcheck` стоит.

**18. Endpoint смены пароля отсутствует.** Серьёзный продуктовый пробел для приложения с финансовыми данными.

## Что НЕ нашёл проблем

- SQL-инъекций нет (везде parameterized queries; динамический `WHERE` в `OperationStore.List` строится через `$N`-плейсхолдеры).
- XSS в шаблонах — рендер через `html/template`, авто-экранирование работает.
- Хранение пароля — bcrypt cost=12, ОК.
- Изоляция данных по `user_id` — последовательно везде.
- Транзакции для трансферов выполняются последовательно в одной горутине (правило про concurrent `pgx.Tx` не нарушено).

## Если делать фиксы — порядок

1. (1) trusted proxies для XFF — критично против bruteforce.
2. (2) static files под общий middleware-стек.
3. (3) CSRF на logout.
4. (6)+(7) укрепить схему БД.
5. (8) корректный `FOR UPDATE` в SetDefault.
6. (9) CSP без `unsafe-*` через nonce.
7. Endpoint смены пароля + инвалидация всех сессий.
