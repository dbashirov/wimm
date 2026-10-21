// Core-утилиты Tailwind, подключаются после style.css (static/tailwind.css)
// и перекрывают его, как раньше стили Play CDN. daisyui здесь нет: его темы
// и компоненты собираются отдельно и идут до style.css.
/** @type {import('tailwindcss').Config} */
module.exports = {
  content: ['./web/templates/**/*.html'],
  theme: {
    extend: {
      fontFamily: { sans: ['Inter', 'sans-serif'] },
    },
  },
}
