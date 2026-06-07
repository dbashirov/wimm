# TODO: перевести фронтенд на собранный Tailwind CSS

## Контекст

В коммите `da30f6b` («фронтенд аудит») из `base.html` убрали runtime-движок
Tailwind (`cdn.tailwindcss.com`) ради ускорения загрузки. Но взамен статический
CSS собран не был, а DaisyUI (`daisyui/full.min.css`) содержит только компоненты,
без базовых utility-классов Tailwind (`flex`, `max-w-6xl`, `px-4`, `lg:flex` и т.д.).
В итоге вся вёрстка развалилась.

Временное решение (коммит-фикс): вернули `cdn.tailwindcss.com` обратно.
Это чинит вёрстку, но возвращает медленную загрузку — движок Tailwind (~120 КБ gz
JS) скачивается и компилирует CSS в браузере при каждой полной загрузке страницы
(+100–300 мс до появления стилей, плюс FOUC). Особенно заметно при переходах по
верхнему меню (обычные `<a href>`, без htmx → полная перезагрузка).

## Цель

Убрать runtime-CDN Tailwind, заменив его на заранее собранный минифицированный CSS.
Тогда: загрузка быстрая, вёрстка на месте, нет FOUC.

## Что нужно сделать

1. **Node-инструментарий для сборки** (в рантайме Go не требуется):
   - `package.json` с dev-зависимостями `tailwindcss@3` + `daisyui@4`
     (те же мажорные версии, что сейчас на CDN — классы менять не придётся).

2. **`tailwind.config.js`**:
   - `content: ['./web/templates/**/*.html']`
   - `plugins: [require('daisyui')]` + настройки тем (`dark`/`light` под `data-theme`)
   - `theme.extend.fontFamily.sans: ['Inter', 'sans-serif']`
   - **`safelist`** для классов, которые в JS собираются конкатенацией строк
     (тосты: `'wimm-toast ' + typeClass`, активный `nav-link`) — иначе purge их выкосит.

3. **Входной CSS** `web/styles/input.css`:
   ```css
   @tailwind base;
   @tailwind components;
   @tailwind utilities;
   ```
   Рукописный `web/static/style.css` оставить отдельным `<link>` (от Tailwind не зависит).

4. **Команда сборки** (в `Makefile` или npm-скрипт):
   ```bash
   npx tailwindcss -i web/styles/input.css -o web/static/app.css --minify
   ```

5. **Правки `web/templates/base.html`**:
   - убрать `<script src="https://cdn.tailwindcss.com">` и инлайн `tailwind.config`
   - убрать `<link ... daisyui/full.min.css>` (DaisyUI теперь внутри `app.css`)
   - подключить `<link rel="stylesheet" href="/static/app.css">`

6. **Закоммитить собранный `web/static/app.css`** — `go:embed` встраивает файлы на
   момент `go build`, поэтому CSS должен лежать в репозитории. Процесс ручной, как с
   миграциями: поменял шаблоны/классы → пересобрал CSS → закоммитил. Описать в `CLAUDE.md`.

## Тонкости

- **Flatpickr**: его CSS/JS тоже убрали из `base.html` в том же аудите. Если календари
  нужны (`operations.html`) — вернуть через `head_extra`/`scripts` в нужных страницах.
- **CI/Docker**: если есть сборка в пайплайне — добавить шаг `npx tailwindcss` перед
  `go build`, иначе забытая пересборка снова сломает вёрстку. Либо договориться, что
  CSS генерится только локально и коммитится.
- **Версии**: остаёмся на Tailwind 3 + DaisyUI 4. Переход на Tailwind 4 / DaisyUI 5
  не нужен — там изменился синтаксис, придётся править классы во всех шаблонах.
