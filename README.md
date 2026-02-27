# Obzornik

Платформа для массовой работы с проектами и доменами: генерация сайтов, управление ссылками,
расписания, индекс‑мониторинг, AI‑редактор файлов и аудит LLM‑расходов.

## Что есть в продукте сейчас

- Аутентификация и RBAC (`admin`, `manager`, роли участников проекта).
- Проекты/домены: CRUD, импорт, сводки и очереди.
- Генерация сайтов по пайплайну с логами, артефактами и перезапусками.
- Link-flow: run/remove/retry, статусы задач и расписания ссылок.
- Indexing-flow: ручные и плановые проверки, статистика, календарь, failed checks.
- AI Editor: работа с файлами домена, версии/revert, AI suggest/create-page, регенерация ассетов.
- LLM Usage: события по токенам/стоимости, цены моделей, admin/project отчеты.
- Мониторинг: `healthz`, Prometheus, Grafana.

## Технологический стек

- Backend: Go (`pbn-generator/cmd/authserver` + воркеры)
- Frontend: Next.js App Router (`frontend`)
- DB: PostgreSQL
- Queue: Redis + Asynq
- Infra (dev): Docker Compose

## Архитектура сервисов (docker-compose)

Основные сервисы:

- `backend` — HTTP API.
- `worker` — обработка generation/link задач.
- `scheduler` — планировщик generation/link расписаний.
- `indexchecker` — проверки индексации.
- `migrate` — миграции.
- `seed` — сидинг SQL-данными.
- `frontend` — UI и docs.
- `db`, `redis`, `prometheus`, `grafana`.

## Быстрый старт (Docker)

### 1) Подготовка

Требования:

- Docker + Docker Compose

Проверьте `.env` (локально), при необходимости задайте свои значения.

### 2) Запуск

```bash
docker compose up --build
```

По умолчанию поднимутся API, воркеры, frontend, БД, Redis и мониторинг.

### 3) Доступные URL

- Frontend: `http://localhost:3000`
- API: `http://localhost:8080`
- Healthcheck: `http://localhost:8080/healthz`
- Metrics: `http://localhost:8080/metrics`
- Prometheus: `http://localhost:9090`
- Grafana: `http://localhost:3001`

### 4) Dev-сидинг через SQL

```bash
docker compose run --rm seed
```

## Локальная разработка (без Docker)

## Backend API

```bash
cd pbn-generator
export DB_DRIVER=pgx
export DB_DSN='postgres://auth:auth@localhost:5432/auth?sslmode=disable'
export JWT_SECRET='your-jwt-secret-changeme'
export API_KEY_SECRET='your-api-key-secret-changeme'
go run ./cmd/authserver
```

## Worker

```bash
cd pbn-generator
export DB_DRIVER=pgx
export DB_DSN='postgres://auth:auth@localhost:5432/auth?sslmode=disable'
export JWT_SECRET='your-jwt-secret-changeme'
export API_KEY_SECRET='your-api-key-secret-changeme'
export REDIS_ADDR='localhost:6379'
go run ./cmd/worker
```

## Scheduler

```bash
cd pbn-generator
export DB_DRIVER=pgx
export DB_DSN='postgres://auth:auth@localhost:5432/auth?sslmode=disable'
export JWT_SECRET='your-jwt-secret-changeme'
export API_KEY_SECRET='your-api-key-secret-changeme'
export REDIS_ADDR='localhost:6379'
go run ./cmd/scheduler
```

## Indexchecker

```bash
cd pbn-generator
export DB_DRIVER=pgx
export DB_DSN='postgres://auth:auth@localhost:5432/auth?sslmode=disable'
export JWT_SECRET='your-jwt-secret-changeme'
export API_KEY_SECRET='your-api-key-secret-changeme'
go run ./cmd/indexchecker
```

## Frontend

```bash
cd frontend
npm install
npm run dev
```

## Миграции вручную

```bash
cd pbn-generator
export DB_DRIVER=pgx
export DB_DSN='postgres://auth:auth@localhost:5432/auth?sslmode=disable'
export JWT_SECRET='your-jwt-secret-changeme'
export API_KEY_SECRET='your-api-key-secret-changeme'
go run ./cmd/migrate
```

## Документация

## Product docs (UI)

Разделы доступны в frontend:

- `/docs` — обзор и базовые сценарии
- `/docs/projects`
- `/docs/domains`
- `/docs/schedules`
- `/docs/queue`
- `/docs/links`
- `/docs/indexing`
- `/docs/indexing-api`
- `/docs/errors`

## Swagger/OpenAPI

- Source of truth: `frontend/openapi.yaml`
- Swagger UI: `/docs/api` (только для `admin`)
- Proxy endpoint (admin-only): `/api/openapi`

Важно: ручной список всех endpoint в README не ведем; API актуализируется через OpenAPI.

## Основные API-группы

- Auth/Profile: логин, сессии, профиль, API key, пароль, email flows.
- Projects/Domains: CRUD, summary, members, prompts.
- Generations/Queue: управление задачами, retry/cancel/pause/resume.
- Links: list/run/remove/retry/import + link schedule.
- Files/Editor: file operations, history/revert, AI suggest/create-page/regenerate-asset.
- Index checks: domain/project/admin monitoring API.
- LLM usage/pricing: admin + project analytics.
- Admin: users, prompts, audit-rules.

## Переменные окружения (минимум)

Критичные переменные:

- `DB_DSN`, `DB_DRIVER`
- `JWT_SECRET`, `JWT_ISSUER`
- `API_KEY_SECRET`
- `REDIS_ADDR`, `REDIS_PASSWORD`, `REDIS_DB`
- `ALLOWED_ORIGINS` или `ALLOWED_ORIGIN`
- `PUBLIC_APP_URL`

Для AI-функций:

- `GEMINI_API_KEY`

Для deploy scaffold (`local_mock`/`ssh_remote`):

- `DEPLOY_MODE`
- `DEPLOY_TIMEOUT`
- `DEPLOY_MAX_PARALLEL`
- `DEPLOY_STAGING_STRATEGY`
- `DEPLOY_STAGING_DIR_NAME`
- `DEPLOY_TARGETS_JSON` (должен быть валидным JSON)
- `DEPLOY_KNOWN_HOSTS_PATH` (обязателен для `ssh_remote`)
- `DEPLOY_SSH_POOL_MAX_OPEN`
- `DEPLOY_SSH_POOL_MAX_IDLE`
- `DEPLOY_SSH_POOL_IDLE_TTL`

Для SMTP/verification (опционально):

- `SMTP_*`
- `REQUIRE_EMAIL_VERIFICATION`

Используйте `.env` для локальной разработки и `.env.prod` как шаблон прод‑настроек.

## Тестирование

## Backend

```bash
cd pbn-generator
go test ./...
```

## Frontend типы

```bash
cd frontend
npx tsc --noEmit
```

## Frontend verify-скрипты

Примеры:

```bash
cd frontend
npm run -s verify:file-editor-route
npm run -s verify:ai-editor-panel
npm run -s verify:index-monitoring-ui
npm run -s verify:project-queue
```

Полный список: `frontend/package.json` (`scripts.verify:*`).

## Безопасность

- Не храните реальные секреты в git.
- Используйте CI-проверки и локальный pre-commit workflow команды.

## Бэкап PostgreSQL

Создание бэкапа одновременно:
- в контейнере `db` (`/var/lib/postgresql/backups`)
- на локальной машине (`backups/postgres`)

```bash
./ops/db/backup_postgres.sh
```

Переопределение путей при необходимости:

```bash
BACKUP_CONTAINER_DIR=/var/lib/postgresql/backups \
BACKUP_LOCAL_DIR=./backups/postgres \
./ops/db/backup_postgres.sh
```

## Импорт/бэкфилл legacy данных

- Импорт: `pbn-generator/cmd/import_legacy/README.md`
- Бэкфилл артефактов: `pbn-generator/cmd/backfill_legacy_artifacts/README.md`

## Лицензия

Пока не задана в репозитории.
