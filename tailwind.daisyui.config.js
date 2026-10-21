// Темы и компоненты daisyui (static/daisyui.css), подключаются до style.css:
// style.css переопределяет card, btn-primary, badge-* и др.
const base = require('./tailwind.config.js')

/** @type {import('tailwindcss').Config} */
module.exports = {
  ...base,
  plugins: [require('daisyui')],
  daisyui: {
    themes: ['light', 'dark'],
    logs: false,
  },
}
