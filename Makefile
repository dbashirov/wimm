# CSS: Tailwind + daisyui. Без локального npm команды идут через Docker-образ node.
NODE_IMAGE := node:22-alpine
HAS_NPM := $(shell command -v npm 2>/dev/null)
NPM := $(if $(HAS_NPM),npm,docker run --rm -v "$(CURDIR)":/app -w /app $(NODE_IMAGE) npm)
NPM_TTY := $(if $(HAS_NPM),npm,docker run --rm -it -v "$(CURDIR)":/app -w /app $(NODE_IMAGE) npm)

.PHONY: css css-watch

css: node_modules
	$(NPM) run build:css

# пересборка при правке шаблонов; Ctrl+C - остановить
css-watch: node_modules
	$(NPM_TTY) run watch:css

node_modules: package.json package-lock.json
	$(NPM) ci --no-audit --no-fund
	@touch node_modules
