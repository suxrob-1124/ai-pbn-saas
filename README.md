# Obzornik — Система генерации сайтов с анализом конкурентов

Полнофункциональная платформа для управления проектами, доменами и автоматической генерации контента на основе анализа поисковой выдачи (SERP). Система включает аутентификацию, RBAC, очередь задач, анализ конкурентов и админ-панель.

## 🏗️ Архитектура

**Монолитное приложение** с модульной структурой:
- **Backend (Go)**: REST API сервер + воркер для фоновых задач
- **Frontend (Next.js)**: React-приложение с App Router
- **Database**: PostgreSQL (pgx драйвер)

- **Queue**: Redis + Asynq для асинхронных задач
- **Monitoring**: Prometheus + Grafana
- **Deployment**: Docker Compose (dev) + Helm (prod)

> **Примечание**: Решение по архитектуре (монолит vs микросервисы) см. [ARCHITECTURE_DECISION.md](./docs/ARCHITECTURE_DECISION.md)

### Структура модулей

```
pbn-generator/  # Backend сервер (было authserver, см. [RENAME_PLAN.md](./docs/RENAME_PLAN.md))
├── cmd/
│   ├── pbn-generator/    # HTTP API сервер
│   ├── worker/        # Воркер для обработки задач генерации
│   ├── migrate/       # Миграции БД
│   └── healthcheck/   # Healthcheck утилита
├── internal/
│   ├── auth/          # Аутентификация, авторизация, сессии
│   ├── httpserver/    # HTTP handlers, middleware
│   ├── store/         # SQL store (users, projects, domains, generations)
│   ├── tasks/         # Очередь задач (Asynq)
│   ├── analyzer/      # Анализатор SERP и страниц конкурентов
│   ├── config/        # Конфигурация
│   ├── db/            # Подключение к БД, миграции
│   ├── notify/        # Email уведомления (SMTP/noop)
│   └── crypto/        # Шифрование (Secretbox для API ключей)
└── init/
    └── seed.sql       # Начальные данные
scripts/
└── seed_backend.sh    # Сидинг через backend API

frontend/
├── app/               # Next.js App Router страницы
│   ├── login/        # Авторизация
│   ├── register/     # Регистрация
│   ├── projects/     # Управление проектами
│   ├── domains/      # Управление доменами
│   ├── queue/        # Очередь генераций
│   ├── admin/        # Админ-панель (RBAC)
│   ├── monitoring/   # Мониторинг метрик
│   └── me/           # Профиль пользователя
└── components/       # React компоненты
```

## ✅ Реализованный функционал

### Аутентификация и авторизация

- ✅ Регистрация пользователей (email/password)
- ✅ Логин/логаут (JWT access/refresh токены, HttpOnly cookies)
- ✅ Подтверждение email (опционально через `REQUIRE_EMAIL_VERIFICATION`)
- ✅ Сброс пароля через email
- ✅ Смена пароля (с ревокацией всех сессий)
- ✅ Смена email (двухэтапный процесс с подтверждением)
- ✅ Rate limiting (логин, регистрация, комбинированный email+IP)
- ✅ Account lockout после N неудачных попыток
- ✅ CAPTCHA (внутренняя реализация, опционально)
- ✅ Сессии с TTL и автоматической очисткой
- ✅ Ревокация всех сессий пользователя

### RBAC (Role-Based Access Control)

- ✅ Роли: `admin`, `manager`
- ✅ Флаг `is_approved` для активации аккаунтов
- ✅ Админ-панель для управления пользователями
- ✅ Авторизация проектов (только владелец или admin)
- ✅ Middleware `requireAdmin` для админских эндпоинтов

### Проекты и домены

- ✅ CRUD проектов (создание, просмотр, обновление, удаление)
- ✅ CRUD доменов в рамках проекта
- ✅ Импорт доменов (массовое добавление через текст)
- ✅ Настройки проекта: страна, язык, глобальный blacklist
- ✅ Настройки домена: ключевое слово, страна, язык, exclude domains, специфичный blacklist
- ✅ Статусы доменов: `waiting`, `published`
- ✅ Связь с серверами (таблица `servers`, пока не используется в UI)

### Генерация контента

- ✅ Очередь задач на основе Redis + Asynq
- ✅ Воркер для обработки задач генерации
- ✅ Анализатор SERP (интеграция с alfasearchspy.alfasearch.ru)
- ✅ Скачивание и анализ страниц конкурентов (Top-20)
- ✅ Извлечение контента: title, H1/H2, текст, HTML
- ✅ Расчет метрик: word count, char count, reading time
- ✅ TF-IDF анализ для выделения ключевых терминов
- ✅ Генерация артефактов: CSV отчет, текстовый файл с контентом
- ✅ Статусы генерации: `pending`, `processing`, `success`, `error`
- ✅ Прогресс выполнения (0-100%)
- ✅ Логи процесса генерации
- ✅ Хранение артефактов в JSONB

### Админ-панель

- ✅ Управление пользователями (список, изменение роли, активация/блокировка)
- ✅ Просмотр проектов пользователя
- ✅ Управление системными промптами (CRUD)
- ✅ Активация/деактивация промптов

### Мониторинг

- ✅ Prometheus метрики:
  - HTTP request duration
  - HTTP request count
  - Generation status counts
- ✅ Healthcheck endpoint (`/healthz`)
- ✅ Grafana дашборды (настраиваются вручную)

### Frontend

- ✅ Страницы: login, register, projects, domains, queue, admin, monitoring, profile
- ✅ Автоматическое обновление статусов генераций (polling)
- ✅ Просмотр артефактов генераций
- ✅ Темная тема (dark mode)
- ✅ Responsive дизайн

## 🔗 Link-flow (актуальная семантика)

- Канонические статусы link-task: `pending`, `searching`, `removing`, `inserted`, `generated`, `removed`, `failed`.
- Legacy-статус `found` больше не используется как рабочий и нормализуется в `searching`.
- `remove` идемпотентен: если ссылка уже отсутствует, задача завершается как `removed` (с warning в логах).
- `relink` без найденного источника замены завершается как `failed` без fallback-генерации нового блока.
- `POST /api/links/{id}/retry` сбрасывает lifecycle задачи: `attempts=0`, `scheduled_for=now`, `created_at=now`, очищает runtime-поля задачи.

### Инфраструктура

- ✅ Docker Compose для локальной разработки
- ✅ Helm charts для Kubernetes деплоя
- ✅ Отдельные сервисы: auth, worker, migrate, frontend, db, redis, prometheus, grafana
- ✅ Healthchecks для всех сервисов
- ✅ Миграции БД (автоматические или через отдельный job)

## 🚧 Что нужно доработать

### Критичные задачи

1. **Publisher (публикация артефактов)**
   - ❌ Реализация SFTP/S3 публикации
   - ❌ Интерфейс `Publisher` в `internal/publisher`
   - ❌ Эндпоинт `POST /api/generations/:id/publish`
   - ❌ Интеграция с таблицей `servers` для SFTP деплоя

2. **Project Members (совместная работа)**
   - ⚠️ Таблица `project_members` создана, но не используется
   - ❌ API для добавления/удаления участников проекта
   - ❌ Проверка прав доступа через `project_members`
   - ❌ UI для управления участниками

3. **API Keys**
   - ⚠️ Хранение зашифрованных API ключей реализовано
   - ❌ Генерация API ключей через UI
   - ❌ Использование API ключей для аутентификации
   - ❌ Ротация ключей

### Улучшения

4. **Генератор контента**
   - ❌ Интеграция с LLM (OpenAI, Anthropic и т.д.)
   - ❌ Использование системных промптов из БД
   - ❌ Генерация HTML на основе анализа конкурентов
   - ❌ Сохранение сгенерированных файлов (ZIP архив)

5. **Тестирование**
   - ⚠️ Есть unit тесты для `session` и `user` store
   - ❌ E2E тесты для критичных сценариев
   - ❌ Интеграционные тесты для API
   - ❌ Тесты для воркера

6. **Документация**
   - ⚠️ README обновлен
   - ❌ API документация (OpenAPI/Swagger)
   - ❌ Архитектурные диаграммы
   - ❌ Руководство по деплою

7. **Безопасность**
   - ⚠️ Rate limiting реализован
   - ❌ Audit log (логирование действий админов)
   - ❌ 2FA (двухфакторная аутентификация)
   - ❌ IP whitelist для админ-панели

8. **Производительность**
   - ❌ Кэширование часто запрашиваемых данных (Redis)
   - ❌ Пагинация для списков (projects, domains, generations)
   - ❌ Оптимизация запросов к БД (индексы проверены)

9. **UX улучшения**
   - ❌ WebSocket для real-time обновлений статусов (вместо polling)
   - ❌ Экспорт данных (CSV, JSON)
   - ❌ Фильтры и поиск в списках
   - ❌ История изменений (audit trail)

## 🚀 Быстрый старт

### Требования

- Docker и Docker Compose
- Go 1.24+ (для локальной разработки)
- Node.js 18+ (для локальной разработки frontend)

### Локальный запуск через Docker Compose

1. Клонируйте репозиторий
2. Создайте файл `.env` (см. пример ниже)
3. Запустите:

```bash
docker compose up --build
```

Сервисы будут доступны:
- **API**: http://localhost:8080
- **Frontend**: http://localhost:3000
- **Postgres**: localhost:5432
- **Redis**: localhost:6379
- **Prometheus**: http://localhost:9090
- **Grafana**: http://localhost:3001 (admin/admin)

### Переменные окружения (.env)

```bash
# Database
POSTGRES_USER=auth
POSTGRES_PASSWORD=auth
POSTGRES_DB=auth
DB_DRIVER=pgx
DB_DSN=postgres://auth:auth@db:5432/auth?sslmode=disable

# JWT
JWT_SECRET=your-super-secret-key-change-in-production
JWT_ISSUER=pbn-generator

# Redis
REDIS_ADDR=redis:6379
REDIS_PASSWORD=
REDIS_DB=0

# CORS
ALLOWED_ORIGIN=*
# или ALLOWED_ORIGINS=http://localhost:3000,http://localhost:3001

# Email (опционально, если не задано - noop mailer)
SMTP_HOST=smtp.example.com
SMTP_PORT=587
SMTP_USER=user
SMTP_PASSWORD=pass
SMTP_SENDER=noreply@example.com
SMTP_USE_TLS=true
PUBLIC_APP_URL=http://localhost:3000

# Grafana
GF_SECURITY_ADMIN_USER=admin
GF_SECURITY_ADMIN_PASSWORD=admin
```

### Локальная разработка (без Docker)

#### Backend

```bash
cd pbn-generator
export DB_DRIVER=pgx
export DB_DSN="postgres://auth:auth@localhost:5432/auth?sslmode=disable"
export JWT_SECRET=dev-secret
go run ./cmd/authserver
```

#### Frontend

```bash
cd frontend
npm install
npm run dev
```

#### Worker

    ```bash
cd pbn-generator
export REDIS_ADDR=localhost:6379
go run ./cmd/worker
```

## 📚 API Документация

### Аутентификация

#### `POST /api/register`
Регистрация нового пользователя.

```bash
curl -X POST http://localhost:8080/api/register \
  -H "Content-Type: application/json" \
  -d '{"email":"user@example.com","password":"SecurePass123"}'
```

**Требования к паролю** (настраиваются через env):
- Минимум 10 символов (по умолчанию)
- Буквы верхнего и нижнего регистра
- Цифры
- Опционально: специальные символы

#### `POST /api/login`
Вход в систему. Возвращает JWT токены в HttpOnly cookies.

```bash
curl -X POST http://localhost:8080/api/login \
  -H "Content-Type: application/json" \
  -d '{"email":"user@example.com","password":"SecurePass123"}'
```

Опционально можно передать CAPTCHA:
```json
{
  "email": "user@example.com",
  "password": "SecurePass123",
  "captchaId": "captcha-id",
  "captchaAnswer": "answer"
}
```

#### `POST /api/logout`
Выход из системы (ревокация текущей сессии).

```bash
curl -X POST http://localhost:8080/api/logout \
  -H "Authorization: Bearer <access-token>"
```

#### `POST /api/logout-all`
Выход из всех устройств. Опционально можно оставить текущую сессию:

```bash
curl -X POST http://localhost:8080/api/logout-all \
  -H "Authorization: Bearer <access-token>" \
  -H "Content-Type: application/json" \
  -d '{"keepCurrent": true}'
```

#### `GET /api/me`
Получение информации о текущем пользователе.

```bash
curl http://localhost:8080/api/me \
  -H "Authorization: Bearer <access-token>"
```

#### `POST /api/refresh`
Обновление access token через refresh token (из cookie).

```bash
curl -X POST http://localhost:8080/api/refresh \
  -b "refresh_token=<refresh-token>"
```

### Управление паролем

#### `POST /api/password`
Смена пароля (требует текущий пароль).

```bash
curl -X POST http://localhost:8080/api/password \
  -H "Authorization: Bearer <access-token>" \
  -H "Content-Type: application/json" \
  -d '{"currentPassword":"OldPass123","newPassword":"NewPass123"}'
```

#### `POST /api/password/reset/request`
Запрос сброса пароля.

```bash
curl -X POST http://localhost:8080/api/password/reset/request \
  -H "Content-Type: application/json" \
  -d '{"email":"user@example.com"}'
```

#### `POST /api/password/reset/confirm`
Подтверждение сброса пароля.

```bash
curl -X POST http://localhost:8080/api/password/reset/confirm \
  -H "Content-Type: application/json" \
  -d '{"token":"<reset-token>","newPassword":"NewPass123"}'
```

### Подтверждение email

#### `POST /api/verify/request`
Запрос письма для подтверждения email.

```bash
curl -X POST http://localhost:8080/api/verify/request \
  -H "Content-Type: application/json" \
  -d '{"email":"user@example.com"}'
```

#### `POST /api/verify/confirm`
Подтверждение email по токену.

```bash
curl -X POST http://localhost:8080/api/verify/confirm \
  -H "Content-Type: application/json" \
  -d '{"token":"<verification-token>"}'
```

### Проекты

#### `GET /api/projects`
Список проектов текущего пользователя (admin видит все).

```bash
curl http://localhost:8080/api/projects \
  -H "Authorization: Bearer <access-token>"
```

#### `POST /api/projects`
Создание проекта.

```bash
curl -X POST http://localhost:8080/api/projects \
  -H "Authorization: Bearer <access-token>" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "My Project",
    "target_country": "se",
    "target_language": "sv"
  }'
```

#### `GET /api/projects/:id`
Получение проекта по ID.

```bash
curl http://localhost:8080/api/projects/<project-id> \
  -H "Authorization: Bearer <access-token>"
```

#### `PATCH /api/projects/:id`
Обновление проекта.

```bash
curl -X PATCH http://localhost:8080/api/projects/<project-id> \
  -H "Authorization: Bearer <access-token>" \
  -H "Content-Type: application/json" \
  -d '{"name":"Updated Name","target_country":"no"}'
```

#### `DELETE /api/projects/:id`
Удаление проекта (каскадно удаляет домены и генерации).

```bash
curl -X DELETE http://localhost:8080/api/projects/<project-id> \
  -H "Authorization: Bearer <access-token>"
```

### Домены

#### `GET /api/projects/:id/domains`
Список доменов проекта.

```bash
curl http://localhost:8080/api/projects/<project-id>/domains \
  -H "Authorization: Bearer <access-token>"
```

#### `POST /api/projects/:id/domains`
Создание домена.

```bash
curl -X POST http://localhost:8080/api/projects/<project-id>/domains \
  -H "Authorization: Bearer <access-token>" \
  -H "Content-Type: application/json" \
  -d '{
    "url": "example.com",
    "main_keyword": "keyword",
    "target_country": "se",
    "target_language": "sv"
  }'
```

#### `POST /api/projects/:id/domains/import`
Массовый импорт доменов (по одному на строку).

```bash
curl -X POST http://localhost:8080/api/projects/<project-id>/domains/import \
  -H "Authorization: Bearer <access-token>" \
  -H "Content-Type: application/json" \
  -d '{"urls":"domain1.com\ndomain2.com\ndomain3.com"}'
```

#### `GET /api/domains/:id`
Получение домена.

```bash
curl http://localhost:8080/api/domains/<domain-id> \
  -H "Authorization: Bearer <access-token>"
```

#### `PATCH /api/domains/:id`
Обновление домена.

```bash
curl -X PATCH http://localhost:8080/api/domains/<domain-id> \
  -H "Authorization: Bearer <access-token>" \
  -H "Content-Type: application/json" \
  -d '{"main_keyword":"new keyword","target_country":"no"}'
```

#### `POST /api/domains/:id/generate`
Запуск генерации для домена.

```bash
curl -X POST http://localhost:8080/api/domains/<domain-id>/generate \
  -H "Authorization: Bearer <access-token>"
```

#### `GET /api/domains/:id/generations`
Список генераций домена.

```bash
curl http://localhost:8080/api/domains/<domain-id>/generations \
  -H "Authorization: Bearer <access-token>"
```

#### `DELETE /api/domains/:id`
Удаление домена.

```bash
curl -X DELETE http://localhost:8080/api/domains/<domain-id> \
  -H "Authorization: Bearer <access-token>"
```

### Генерации

#### `GET /api/generations`
Список последних генераций (для текущего пользователя или всех для admin).

```bash
curl http://localhost:8080/api/generations \
  -H "Authorization: Bearer <access-token>"
```

#### `GET /api/generations/:id`
Получение генерации с артефактами.

```bash
curl http://localhost:8080/api/generations/<generation-id> \
  -H "Authorization: Bearer <access-token>"
```

### Админ-панель (только для admin)

#### `GET /api/admin/users`
Список всех пользователей.

```bash
curl http://localhost:8080/api/admin/users \
  -H "Authorization: Bearer <admin-access-token>"
```

#### `PATCH /api/admin/users/:email`
Изменение роли или статуса пользователя.

```bash
curl -X PATCH http://localhost:8080/api/admin/users/user@example.com \
  -H "Authorization: Bearer <admin-access-token>" \
  -H "Content-Type: application/json" \
  -d '{"role":"admin","isApproved":true}'
```

**Ограничения на изменение ролей:**

- **Допустимые роли**: только `admin` и `manager`
- **Запрещено**: админ не может изменить свою собственную роль
- **Запрещено**: админ не может понизить роль другого админа (изменить роль админа на `manager`)
- **Валидация**: при попытке установить недопустимую роль вернется ошибка `400 Bad Request` с сообщением `"invalid role: <role> (allowed: admin, manager)"`

**Примеры ответов:**

- `200 OK` — роль успешно изменена
- `400 Bad Request` — недопустимая роль или пользователь не найден
- `403 Forbidden` — попытка изменить свою роль или роль другого админа
- `401 Unauthorized` — требуется авторизация с ролью `admin`

#### `GET /api/admin/users/:email/projects`
Проекты пользователя (для админа).

```bash
curl http://localhost:8080/api/admin/users/user@example.com/projects \
  -H "Authorization: Bearer <admin-access-token>"
```

#### `GET /api/admin/prompts`
Список системных промптов.

```bash
curl http://localhost:8080/api/admin/prompts \
  -H "Authorization: Bearer <admin-access-token>"
```

#### `POST /api/admin/prompts`
Создание промпта.

```bash
curl -X POST http://localhost:8080/api/admin/prompts \
  -H "Authorization: Bearer <admin-access-token>" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Default Prompt",
    "description": "Description",
    "body": "You are a content generator...",
    "isActive": true
  }'
```

#### `PATCH /api/admin/prompts/:id`
Обновление промпта.

```bash
curl -X PATCH http://localhost:8080/api/admin/prompts/<prompt-id> \
  -H "Authorization: Bearer <admin-access-token>" \
  -H "Content-Type: application/json" \
  -d '{"body":"Updated prompt text","isActive":false}'
```

#### `DELETE /api/admin/prompts/:id`
Удаление промпта.

```bash
curl -X DELETE http://localhost:8080/api/admin/prompts/<prompt-id> \
  -H "Authorization: Bearer <admin-access-token>"
```

### Мониторинг

#### `GET /healthz`
Healthcheck endpoint.

```bash
curl http://localhost:8080/healthz
```

#### `GET /metrics`
Prometheus метрики.

```bash
curl http://localhost:8080/metrics
```

## 🔧 Конфигурация

### Переменные окружения

#### Общие
- `PORT` — порт HTTP сервера (по умолчанию `8080`)
- `ALLOWED_ORIGIN` или `ALLOWED_ORIGINS` — CORS origins (по умолчанию `*`)
- `LOG_LEVEL` — уровень логирования (по умолчанию `info`)
#### Bootstrap (dev)
- `BOOTSTRAP_ADMIN_EMAIL` — email пользователя, который при регистрации станет админом (dev)
- `AUTO_APPROVE_USERS` — автоматически одобрять пользователей при регистрации (dev)

#### База данных
- `DB_DRIVER` — драйвер БД (только `pgx`)
- `DB_DSN` — connection string PostgreSQL
- `DB_CONNECT_RETRIES` — количество попыток подключения (по умолчанию `10`)
- `DB_CONNECT_INTERVAL` — интервал между попытками (по умолчанию `1s`)
- `MIGRATE_ON_START` — запускать миграции при старте (по умолчанию `true`, в проде `false`)

#### JWT и сессии
- `JWT_SECRET` — секретный ключ для подписи JWT (**обязательно изменить в проде**)
- `JWT_ISSUER` — issuer для JWT (по умолчанию `pbn-generator`)
- `ACCESS_TOKEN_TTL` — время жизни access token (по умолчанию `15m`)
- `REFRESH_TOKEN_TTL` — время жизни refresh token (по умолчанию `7d`)
- `SESSION_TTL` — время жизни сессии (по умолчанию `24h`)
- `SESSION_CLEAN_INTERVAL` — интервал очистки истекших сессий (по умолчанию `5m`)

#### Cookies
- `COOKIE_DOMAIN` — домен для cookies (например `localhost`)
- `COOKIE_SECURE` — использовать Secure флаг (по умолчанию `false`, в проде `true`)

#### Rate limiting
- `LOGIN_RATE_LIMIT` / `LOGIN_RATE_WINDOW` — лимит попыток входа (по умолчанию `30`/`1m`)
- `LOGIN_EMAIL_IP_LIMIT` / `LOGIN_EMAIL_IP_WINDOW` — комбинированный лимит email+IP (по умолчанию `10`/`1m`)
- `REGISTER_RATE_LIMIT` / `REGISTER_RATE_WINDOW` — лимит регистраций (по умолчанию `10`/`1m`)
- `LOGIN_LOCKOUT_FAILS` / `LOGIN_LOCKOUT_DURATION` — блокировка после N попыток (по умолчанию `5`/`15m`)

#### Пароли
- `PASSWORD_MIN_LENGTH` / `PASSWORD_MAX_LENGTH` — длина пароля (по умолчанию `10`/`128`)
- `PASSWORD_REQUIRE_UPPER` / `PASSWORD_REQUIRE_LOWER` — требовать заглавные/строчные (по умолчанию `true`)
- `PASSWORD_REQUIRE_DIGIT` — требовать цифры (по умолчанию `true`)
- `PASSWORD_REQUIRE_SPECIAL` — требовать спецсимволы (по умолчанию `false`)

#### Email
- `REQUIRE_EMAIL_VERIFICATION` — требовать подтверждение email (по умолчанию `false`)
- `EMAIL_VERIFICATION_TTL` — время жизни токена верификации (по умолчанию `24h`)
- `PASSWORD_RESET_TTL` — время жизни токена сброса пароля (по умолчанию `1h`)
- `SMTP_HOST`, `SMTP_PORT` (по умолчанию `587`), `SMTP_USER`, `SMTP_PASSWORD`, `SMTP_SENDER`, `SMTP_USE_TLS` (по умолчанию `true`)
- `PUBLIC_APP_URL` — URL приложения для ссылок в письмах (по умолчанию `http://localhost:3000`)

#### CAPTCHA
- `CAPTCHA_REQUIRED` — требовать CAPTCHA (по умолчанию `false`)
- `CAPTCHA_PROVIDER` — провайдер (по умолчанию `internal`)
- `CAPTCHA_SECRET`, `CAPTCHA_ATTEMPTS`, `CAPTCHA_WINDOW`

#### Redis (для очереди)
- `REDIS_ADDR` — адрес Redis (по умолчанию `localhost:6379`)
- `REDIS_PASSWORD` — пароль Redis (опционально)
- `REDIS_DB` — номер БД Redis (по умолчанию `0`)

## 🐳 Docker Compose

### Локальная разработка

```bash
docker compose up --build
```

Сервисы:
- `db` — PostgreSQL
- `auth` — API сервер
- `worker` — воркер для задач
- `migrate` — миграции БД (запускается один раз)
- `seed` — начальные данные (запускается один раз)
- `seed_backend` — сидинг через backend API (после запуска backend)
- `frontend` — Next.js приложение
- `redis` — Redis для очереди
- `prometheus` — метрики
- `grafana` — дашборды

### Сидинг через backend API (dev)

Скрипт создаёт пользователей, проекты и домены через API и идемпотентен:

```bash
./scripts/seed_backend.sh
```

По умолчанию используются пароли:
- `admin@example.com` / `Admin123!!`
- `manager@example.com` / `Manager123!!`
- `manager2@example.com` / `Manager123!!`
- `user@example.com` / `User123!!!`

Подробнее: `docs/SEEDING.md`

### Защита от утечки секретов в Git

В репозитории есть локальная проверка секретов перед `commit` и `push`.

Установка:

```bash
./scripts/install_git_hooks.sh
```

Ручной запуск проверки:

```bash
./scripts/check_no_secrets.sh --staged
```

Проверка сканирует добавленные строки diff и блокирует:
- потенциальные ключи/токены/секреты;
- коммит `.env` и ключевых файлов (`.pem`, `.key`, `id_rsa` и т.п.).

### Импорт legacy-сайтов (CSV -> DB)

CLI импортера:

```bash
cd pbn-generator
go run ./cmd/import_legacy --help
```

Полная инструкция (режимы `dry-run/apply`, формат CSV, troubleshooting):  
`pbn-generator/cmd/import_legacy/README.md`

### UI-редактор файлов домена (Sprint 4)

- Маршрут: `/domains/:id/editor`
- Поддерживается редактирование опубликованных сайтов (импортированных и сгенерированных)
- Права:
  - `viewer` — только чтение
  - `owner/editor/admin` — чтение и сохранение
- История изменений в v1: metadata-only (без diff/revert)

Актуальный backlog и DoD по спринту: `todo-v4.md`

### V2: Domain Result & Legacy Decode

- На странице домена `/domains/:id` добавлен явный action `Открыть в редакторе`.
- Добавлен блок `Результат` с быстрыми действиями:
  - `Просмотр HTML` (final_html)
  - `Скачать ZIP` (zip_archive)
  - `К артефактам`
- Для legacy-импортов поддержан synthetic generation decode (`prompt_id=legacy_decode_v2`).

Backfill для уже импортированных доменов:

```bash
cd pbn-generator
go run ./cmd/backfill_legacy_artifacts --help
```

Документация:
- `pbn-generator/cmd/import_legacy/README.md`
- `pbn-generator/cmd/backfill_legacy_artifacts/README.md`

### Production

Используйте `docker-compose.prod.yml`:

```bash
docker compose -f docker-compose.prod.yml up --build
```

**Важно для продакшена:**
- Установите сильные секреты (`JWT_SECRET`, пароли БД)
- Настройте `ALLOWED_ORIGINS` на конкретные домены
- Отключите `MIGRATE_ON_START=false` и используйте job `migrate`
- Включите `COOKIE_SECURE=true`
- Настройте SMTP для отправки писем

## ☸️ Kubernetes (Helm)

Чарт находится в `deploy/helm/pbn-generator/`.

### Установка

```bash
helm upgrade --install pbn-generator deploy/helm/pbn-generator \
  --set image.repository=your-registry/pbn-generator \
  --set image.tag=latest \
  --set secretEnv[0].name=JWT_SECRET \
  --set secretEnv[0].value=super-secret-key \
  --set ingress.enabled=true \
  --set ingress.hosts[0].host=api.example.com
```

### Настройка values.yaml

- `image.repository` и `image.tag` — образ контейнера
- `secretEnv` — секретные переменные окружения
- `ingress` — настройки Ingress
- `resources` — лимиты ресурсов
- `replicas` — количество реплик

## 🧪 Тестирование

### Unit тесты

```bash
cd pbn-generator
go test ./...
```

### E2E тесты

```bash
cd pbn-generator
go test ./internal/httpserver -tags=e2e
```

## 📊 Мониторинг

### Prometheus метрики

- `auth_http_request_duration_seconds` — длительность HTTP запросов
- `auth_http_requests_total` — количество HTTP запросов
- `auth_generation_status_total` — количество генераций по статусам

### Grafana

Импортируйте дашборды в Grafana (настраиваются вручную):
- HTTP метрики
- Статусы генераций
- Ошибки

## 🔒 Безопасность

### Рекомендации для продакшена

1. **Секреты**: Используйте сильные случайные значения для `JWT_SECRET`
2. **HTTPS**: Всегда используйте HTTPS в проде
3. **CORS**: Ограничьте `ALLOWED_ORIGINS` конкретными доменами
4. **Cookies**: Включите `COOKIE_SECURE=true` и настройте `COOKIE_DOMAIN`
5. **Rate limiting**: Настройте лимиты под вашу нагрузку
6. **Email verification**: Включите `REQUIRE_EMAIL_VERIFICATION=true`
7. **Database**: Используйте SSL для подключения к БД
8. **Secrets management**: Храните секреты в Kubernetes Secrets или внешнем менеджере (Vault)

## 🎯 Стратегия развития

### Текущее решение: Монолит с модульной архитектурой

Проект развивается как монолитное приложение с четким разделением на модули. Это позволяет:
- Быстро разрабатывать новые функции
- Легко тестировать и дебажить
- При необходимости выделить модули в отдельные сервисы позже

**См. [docs/ARCHITECTURE_DECISION.md](./docs/ARCHITECTURE_DECISION.md)** для детального анализа и обоснования решения.

### Roadmap

Проект развивается в три этапа:

- **MVP (v1.0)** — Полная замена 8n8: LLM интеграция, сборка, публикация, базовый шеринг проектов
- **v2.0** — Планировщик генерации и Link Building (пост-обработка)
- **v3.0** — Preview, Edit, Multipage (многостраничность)

**См. [docs/ROADMAP.md](./docs/ROADMAP.md)** для детального плана развития.

**См. [docs/TECHNICAL_DESIGN.md](./docs/TECHNICAL_DESIGN.md)** для технических решений сложных фич.

**См. [TODO.md](./TODO.md)** для списка конкретных задач.

## 📖 Документация

Вся техническая документация находится в папке [`docs/`](./docs/):

- **[docs/ARCHITECTURE_DECISION.md](./docs/ARCHITECTURE_DECISION.md)** — Решение по архитектуре (монолит vs микросервисы)
- **[docs/ROADMAP.md](./docs/ROADMAP.md)** — План развития проекта (MVP, v2.0, v3.0)
- **[docs/TECHNICAL_DESIGN.md](./docs/TECHNICAL_DESIGN.md)** — Технические решения для сложных фич
- **[docs/RENAME_PLAN.md](./docs/RENAME_PLAN.md)** — План переименования проекта

См. [docs/README.md](./docs/README.md) для полного списка документации.

## 📝 Лицензия

[Указать лицензию]

## 👥 Авторы

[Указать авторов]

---

**Примечание**: Этот README актуализирован на основе полного анализа кодовой базы. Для получения актуальной информации о статусе задач см. `TODO.md`.
