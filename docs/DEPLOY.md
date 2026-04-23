# План продакшен-деплоя

## Архитектура

```
Интернет
    ↓
nginx (80/443, SSL)
    ↓
Go app (localhost:8080)
    ↓
PostgreSQL (localhost:5432)
```

## Шаги

### 1. Сервер

- Заказать VPS на Beget: Ubuntu 22.04, минимум 1 CPU / 1 GB RAM
- Привязать домен к IP сервера (A-запись)
- Настроить SSH-доступ по ключу, отключить вход по паролю

```bash
# На сервере — создать пользователя для деплоя
adduser deploy
usermod -aG sudo deploy
```

### 2. PostgreSQL на сервере (не в Docker)

```bash
apt install -y postgresql-16

# Создать БД и пользователя
sudo -u postgres psql <<EOF
CREATE USER wimm WITH PASSWORD 'СЛОЖНЫЙ_ПАРОЛЬ';
CREATE DATABASE wimm OWNER wimm;
EOF
```

PostgreSQL слушает только `localhost` — по умолчанию так и есть, не менять.

### 3. Бэкапы БД

Крон-задача: раз в день делать дамп и отправлять в Яндекс Object Storage (или любой S3-совместимый).

```bash
# /etc/cron.d/wimm-backup
0 3 * * * deploy pg_dump -U wimm wimm | gzip > /backups/wimm-$(date +\%Y\%m\%d).sql.gz
```

Хранить последние 30 дней. Настроить алерт если бэкап не пришёл (позже).

### 4. Приложение через Docker (только app)

Убрать postgres из docker-compose — БД живёт на хосте.

```yaml
# docker-compose.prod.yml
services:
  app:
    build: .
    restart: unless-stopped
    network_mode: host
    env_file: .env
```

Файл `.env` на сервере (не в git):

```env
DB_URL=postgres://wimm:СЛОЖНЫЙ_ПАРОЛЬ@localhost:5432/wimm?sslmode=disable
SERVER_ADDR=:8080
```

### 5. nginx + HTTPS

```bash
apt install -y nginx certbot python3-certbot-nginx

# Получить SSL-сертификат
certbot --nginx -d ваш-домен.ru
```

Конфиг `/etc/nginx/sites-available/wimm`:

```nginx
server {
    server_name ваш-домен.ru;

    location / {
        proxy_pass http://localhost:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }

    listen 443 ssl;
    # Остальное добавит certbot автоматически
}

server {
    listen 80;
    server_name ваш-домен.ru;
    return 301 https://$host$request_uri;
}
```

```bash
ln -s /etc/nginx/sites-available/wimm /etc/nginx/sites-enabled/
nginx -t && systemctl reload nginx
```

### 6. CI/CD через GitHub Actions

Секреты в репозитории (Settings → Secrets and variables → Actions):

| Секрет | Значение |
|--------|----------|
| `SERVER_HOST` | IP сервера |
| `SERVER_USER` | `deploy` |
| `SERVER_SSH_KEY` | приватный SSH-ключ |

Deploy-скрипт на сервере `/home/deploy/update.sh`:

```bash
#!/bin/bash
set -e
cd /home/deploy/wimm
git pull origin main
docker compose -f docker-compose.prod.yml up -d --build
docker image prune -f
```

Файл `.github/workflows/deploy.yml` — см. раздел CI/CD ниже.

### 7. Firewall

```bash
ufw allow 22    # SSH
ufw allow 80    # HTTP
ufw allow 443   # HTTPS
ufw enable

# Закрыть прямой доступ к приложению и БД снаружи
# 8080 и 5432 не открываем — они доступны только через nginx и localhost
```

## CI/CD

Файл `.github/workflows/deploy.yml`:

```yaml
name: Deploy

on:
  push:
    branches: [main]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
      - run: go test ./...

  deploy:
    needs: test
    runs-on: ubuntu-latest
    steps:
      - name: Deploy via SSH
        uses: appleboy/ssh-action@v1
        with:
          host: ${{ secrets.SERVER_HOST }}
          username: ${{ secrets.SERVER_USER }}
          key: ${{ secrets.SERVER_SSH_KEY }}
          script: bash /home/deploy/update.sh
```

Процесс после настройки:

```bash
git push  # → тесты → автодеплой на сервер
```

## Чеклист перед запуском

- [ ] VPS создан, SSH-доступ настроен
- [ ] Домен привязан к IP (A-запись)
- [ ] PostgreSQL установлен, БД создана
- [ ] Файл `.env` создан на сервере (не в git)
- [ ] Приложение запускается через `docker-compose.prod.yml`
- [ ] nginx настроен, SSL получен через certbot
- [ ] Firewall настроен (ufw)
- [ ] Бэкапы настроены (крон + pg_dump)
- [ ] GitHub Actions секреты добавлены
- [ ] Тестовый `git push` прошёл успешно
