# Деплой на VDS

Архитектура: **PostgreSQL на хосте**, приложение в Docker, nginx снаружи.

```
Интернет → nginx (80/443) → localhost:8080 (Docker app) → localhost:5432 (PostgreSQL)
```

---

## Шаг 1. Пользователь deploy

```bash
adduser deploy
usermod -aG sudo deploy
# Дальше всё от пользователя deploy
su - deploy
```

---

## Шаг 2. Docker

```bash
curl -fsSL https://get.docker.com | sudo sh
sudo usermod -aG docker deploy
newgrp docker
```

---

## Шаг 3. PostgreSQL на хосте

```bash
sudo apt install -y postgresql-16

sudo -u postgres psql <<EOF
CREATE USER wimm WITH PASSWORD 'пароль';
CREATE DATABASE wimm OWNER wimm;
EOF
```

Проверь что слушает только localhost:
```bash
sudo -u postgres psql -c "SHOW listen_addresses;"
# ожидаемо: localhost
```

---

## Шаг 4. Миграции

Скачай бинарник goose:
```bash
sudo curl -fsSL \
  https://github.com/pressly/goose/releases/latest/download/goose_linux_x86_64 \
  -o /usr/local/bin/goose
sudo chmod +x /usr/local/bin/goose
```

Запусти миграции:
```bash
export DB_URL="postgres://wimm:пароль%@localhost:5432/wimm?sslmode=disable"
goose -dir /home/deploy/wimm/migrations postgres "$DB_URL" up
```

---

## Шаг 5. Репозиторий

```bash
cd /home/deploy
git clone https://github.com/dbashirov/wimm.git
cd wimm
```

Если репо приватный — нужен Personal Access Token:
```bash
git clone https://<TOKEN>@github.com/dbashirov/wimm.git
```

---

## Шаг 6. Файл .env на сервере

```bash
nano /home/deploy/wimm/.env
```

Содержимое (`.env` не в git — только на сервере):

```env
DB_URL=postgres://wimm:СЛОЖНЫЙ_ПАРОЛЬ@localhost:5432/wimm?sslmode=disable
SERVER_ADDR=:8080
CSRF_SECRET=  # openssl rand -base64 32
COOKIE_INSECURE=
HSTS=true
LOG_LEVEL=info
LOG_FORMAT=json
```

Сгенерируй `CSRF_SECRET`:
```bash
openssl rand -base64 32
```

---

## Шаг 7. Запуск приложения

```bash
cd /home/deploy/wimm
docker compose -f docker-compose.prod.yml up -d --build
```

Проверь:
```bash
curl http://localhost:8080/health   # 200 OK
curl http://localhost:8080/ready    # 200 OK (пингует БД)
docker logs wimm-app-1              # нет ошибок
```

---

## Шаг 8. nginx + HTTPS

Домен уже должен указывать на IP сервера (A-запись).

```bash
sudo apt install -y nginx certbot python3-certbot-nginx
```

Создай конфиг:
```bash
sudo nano /etc/nginx/sites-available/wimm
```

```nginx
server {
    server_name ваш-домен.ru;

    location / {
        proxy_pass http://localhost:8080;
        proxy_set_header Host              $host;
        proxy_set_header X-Real-IP         $remote_addr;
        proxy_set_header X-Forwarded-For   $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

```bash
sudo ln -s /etc/nginx/sites-available/wimm /etc/nginx/sites-enabled/
sudo nginx -t && sudo systemctl reload nginx

# SSL — certbot сам допишет listen 443 и редирект 80→443
sudo certbot --nginx -d ваш-домен.ru
```

---

## Шаг 9. Firewall

```bash
sudo ufw allow 22
sudo ufw allow 80
sudo ufw allow 443
sudo ufw enable
# 8080 и 5432 не открываем — только внутренний трафик
```

---

## Шаг 10. Бэкапы БД

```bash
sudo mkdir -p /backups && sudo chown deploy:deploy /backups

crontab -e  # от пользователя deploy
```

Добавь строку:
```
0 3 * * * pg_dump -U wimm wimm | gzip > /backups/wimm-$(date +\%Y\%m\%d).sql.gz
```

Проверь вручную:
```bash
pg_dump -U wimm wimm | gzip > /tmp/test-backup.sql.gz
ls -lh /tmp/test-backup.sql.gz
```

---

## Шаг 11. CI/CD через GitHub Actions

Скрипт деплоя на сервере:
```bash
nano /home/deploy/update.sh
```

```bash
#!/bin/bash
set -e
cd /home/deploy/wimm
git pull origin main
docker compose -f docker-compose.prod.yml up -d --build
docker image prune -f
```

```bash
chmod +x /home/deploy/update.sh
```

SSH-ключ для GitHub Actions (генерируй на сервере):
```bash
ssh-keygen -t ed25519 -f ~/.ssh/github_actions -N ""
cat ~/.ssh/github_actions.pub >> ~/.ssh/authorized_keys
cat ~/.ssh/github_actions   # скопируй приватный ключ
```

Добавь секреты в репозитории: **Settings → Secrets and variables → Actions**

| Секрет          | Значение                        |
|-----------------|---------------------------------|
| `SERVER_HOST`   | IP сервера                      |
| `SERVER_USER`   | `deploy`                        |
| `SERVER_SSH_KEY`| содержимое `~/.ssh/github_actions` |

После этого каждый `git push origin main` → тесты → автодеплой.

---

## Финальный чек-лист

- [ ] `curl https://ваш-домен.ru/health` → 200
- [ ] `curl https://ваш-домен.ru/ready` → 200
- [ ] Открыть браузер: зарегистрироваться и войти
- [ ] `docker logs wimm-app-1` — нет ошибок
- [ ] Бэкап отрабатывает вручную
- [ ] `git push` → GitHub Actions деплоит без ошибок
