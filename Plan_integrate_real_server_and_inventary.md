# Plan: Integrate Real Server and Inventory (v2)

## 0) Baseline (что уже есть в проекте)

- `domains.published_path` уже существует.
- `domains.deployment_mode` уже существует (сейчас фактически используется `local_mock`).
- Перед запуском генерации уже есть pre-check API-ключа владельца/админа.
- Публикация реализована через `LocalPublisher` (локальная ФС).
- Editor V2 API сейчас читает/пишет файлы из локального `server/<domain>`, а не из remote target.

Вывод: мы не начинаем с нуля. Нужен поэтапный переход от `local_mock` к `ssh_remote` без ломки текущего UX/API.

## 1) Цели и ограничения

### Цели

- Подключить реальные серверы для чтения/записи файлов и публикации генераций.
- Сохранить существующие API-контракты editor и generation.
- Сделать rollout без массовых регрессий (canary -> batch).

### Ограничения

- Только additive-изменения в БД и API.
- Без big-bang миграции всей файловой логики за один шаг.
- Без хранения приватных ключей в БД.

## 2) Архитектурные решения (фиксируем до кода)

1. Источник target-сервера:
- `domain.server_id` (приоритет)
- fallback: `project.default_server_id`

2. Режим деплоя:
- `local_mock` (текущий)
- `ssh_remote` (новый)

3. Файловый backend:
- единый интерфейс `SiteContentBackend`
- реализации:
  - `LocalFSBackend`
  - `SSHBackend`

4. Безопасность:
- только key-based SSH
- строгий `HostKeyCallback` (по `known_hosts`/зафиксированным host key), без `ssh.InsecureIgnoreHostKey()` в production
- `sudo` только через ограниченный `sudoers` whitelist
- никакого shell interpolation из пользовательского input

5. Производительность:
- bounded connection pool per target
- timeout/retry/backoff
- кэш дерева файлов в Redis с явной инвалидацией

## 3) Этапы внедрения

## Этап A: Data model + config

### Изменения БД (additive)

- `domains.site_owner TEXT NULL` (`user:group`)
- `domains.inventory_status TEXT NULL` (`pending|ok|partial|failed`)
- `domains.inventory_checked_at TIMESTAMPTZ NULL`
- `domains.inventory_error TEXT NULL`

Опционально (если нужен ручной override):
- `domains.publish_root_override TEXT NULL`

### Изменения config

Добавить:
- `DEPLOY_MODE=local_mock|ssh_remote`
- `DEPLOY_TIMEOUT=30s`
- `DEPLOY_MAX_PARALLEL=5`
- `DEPLOY_STAGING_STRATEGY=co_located` (staging рядом с target на той же ФС)
- `DEPLOY_STAGING_DIR_NAME=.tmp_deploy`
- `DEPLOY_TARGETS_JSON=...` (или альтернативно через таблицу `servers` + env только для dev)
- `DEPLOY_KNOWN_HOSTS_PATH=./secrets/ssh/known_hosts`
- `DEPLOY_SSH_POOL_MAX_OPEN=5`
- `DEPLOY_SSH_POOL_MAX_IDLE=2`
- `DEPLOY_SSH_POOL_IDLE_TTL=60s`

### DoD этапа A

- Миграции применяются без ошибок.
- Конфиг валидируется при старте.
- Значения по умолчанию не ломают текущий `local_mock` flow.

---

## Этап B: Инвентаризация 600+ legacy-сайтов

### Новый CLI

- `cmd/inventory_legacy_sites`
- режимы:
  - `--dry-run`
  - `--apply`
- вход:
  - CSV manifest
  - alias/server-id target

### Логика

1. Для домена найти `DocumentRoot`/фактический путь.
2. Через `stat` получить owner/group.
3. Сохранить в `domains`:
- `published_path`
- `site_owner`
- `inventory_status/inventory_checked_at/inventory_error`

### Важные требования

- Конкурентность ограничена (например 10-20 workers).
- Retry с backoff.
- Полный отчёт: `found`, `not_found`, `ambiguous`, `permission_denied`.
- Нормализация доменов через Punycode (`golang.org/x/net/idna`) перед поиском vhost/DocumentRoot.
  - Прогонять через `idna.ToASCII` только host-часть домена, не полный URL.
- Добавить jitter перед SSH dial в воркерах инвентаризации (например 100-500ms), чтобы избежать burst-коннектов.

### DoD этапа B

- На dry-run есть детальный отчёт.
- На apply обновляются только целевые домены.
- Повторный запуск идемпотентен.
- Punycode применяется корректно (только для host), домены с IDN не теряются при поиске.
- Нет burst-подключений к SSH при старте батча (jitter включён).

---

## Этап C: Backend storage abstraction + SSH pool

### Что делаем

1. Вводим интерфейс:
- `ReadFile`
- `WriteFile`
- `ListTree`
- `Move/Delete`
- `PublishBundle` (или отдельный publisher слой)

2. Перенос текущей логики editor file handlers на интерфейс backend-а.

3. Реализация `SSHBackend`:
- connection pool (bounded, lazy-open)
- health-check/reconnect
- operation timeout
- централизованный маппинг ошибок (not found, permission denied, timeout)
- защита от burst-дозвона (учёт `MaxStartups`/Fail2Ban на target-хостах)
- обязательный graceful shutdown: при остановке приложения закрывать все активные SSH-сессии пула (`Close()`).
- верификация подлинности SSH-хоста через `HostKeyCallback` и `known_hosts`/pinned host key.

### Почему так

- Избегаем дублирования логики editor API.
- Можно включать `ssh_remote` по флагу для отдельных доменов/проектов.

### DoD этапа C

- API editor работает в `local_mock` как раньше.
- В `ssh_remote` читается/пишется минимум smoke-сценарий.
- Нет path traversal у remote операций.
- Пул SSH корректно закрывается при graceful shutdown (без висячих сессий на target).
- Для production не используется `ssh.InsecureIgnoreHostKey()`; host key верификация обязательна.

---

## Этап D: Editor V2 + Redis cache

### Что делаем

1. Кэшируем `GET /api/domains/{id}/files` в Redis (`TTL 5-10m`).
2. Инвалидация кэша на:
- create
- move/rename
- delete/restore
- upload
- revert
- save

3. Ленивая загрузка файла остаётся (уже корректный подход).
4. Единый user-facing message + diagnostics details для remote ошибок.

### DoD этапа D

- Дерево файлов не перегружает remote сервер при частом открытии страницы.
- После изменения файлов дерево обновляется консистентно.
- Нет “немых зависаний” в UI.

---

## Этап E: Publish pipeline на remote

### Что делаем

1. Добавляем `SSHPublisher` рядом с `LocalPublisher`.
2. Поддерживаем два режима:
- `local_mock`
- `ssh_remote`

3. Для `ssh_remote`:
- upload artifact bundle во временную директорию на ТОМ ЖЕ filesystem, что и target
- распаковка в target
- атомарный switch (где возможно)
- подготовка прав на `staging` ДО переключения (`chown/chmod`)

Примечание по атомарности:
- не использовать staging в `/tmp`, если target находится на другой ФС;
- staging должен быть co-located рядом с сайтом (пример: `<parent>/.tmp_deploy_<domain>`), чтобы `rename` оставался атомарным.
- для замены уже существующего non-empty каталога использовать `double rename`:
  - подготовить `staging`: `chown/chmod` до switch:
    - `chown -R <site_owner> <staging>`
    - `find <staging> -type d -exec chmod 755 {} \\;`
    - `find <staging> -type f -exec chmod 644 {} \\;`
  - `mv <target> <target>.__backup__<ts>`
  - `mv <staging> <target>` (новый target уже с корректными правами в момент переключения)
  - cleanup backup после smoke-проверки (`rm -rf <target>.__backup__<ts>`)
  - при сбое второго шага выполнять rollback: `mv <target>.__backup__<ts> <target>`

4. Обновление БД:
- `published_path`
- `site_owner` (если известен/изменился)
- статус домена
- deployment attempt details

### DoD этапа E

- Успешная генерация действительно публикуется на remote target.
- Ошибки публикации корректно логируются и не ломают pipeline state machine.
- Возможен rollback/retry без ручной правки БД.
- Замена существующего сайта выполняется через `double rename`, без вложения staging-папки внутрь target.
- Права файлов/директорий нормализованы на `staging` ДО switch (`644`/`755`), нет окна `403` после переключения.

---

## Этап F: Preflight gating + rollout

### Что делаем

1. Расширяем preflight перед enqueue generation:
- API key (уже есть)
- deploy readiness для `ssh_remote`:
  - есть target server
  - есть `published_path`/policy для initial publish
  - SSH health-check ok
  - для новых доменов без `published_path`: проверка существования target root на сервере;
    при отсутствии возвращать user-facing ошибку:
    `Сайт еще не настроен на сервере, обратитесь к администратору`.

2. Rollout:
- canary: 5-10 доменов
- batch rollout по 50-100
- stop rule при росте error rate

### DoD этапа F

- Чужие/невалидные домены не уходят в бесполезную генерацию.
- Метрики стабильны на canary.
- Есть план rollback.
- Новые домены без подготовленного server root не попадают в очередь генерации (блокируются preflight).

## 4) Security checklist (обязательно)

- Ключи только в `secrets/ssh/*`, не в git.
- Приватные ключи должны иметь права `0600` (fail-fast проверка при старте для `ssh_remote`).
- SSH host key verification обязателен (`known_hosts` или pinned key), `InsecureIgnoreHostKey` запрещён в production.
- SSH user без shell-привилегий сверх нужного.
- Ограниченный `sudoers` для `chown/chmod` на нужных путях.
  - Явно требовать `NOPASSWD` для deploy-user, иначе `sudo` по SSH зависнет в non-interactive режиме.
  - Пример: `n8n_deploy_user ALL=(ALL) NOPASSWD: /usr/bin/chown, /usr/bin/chmod`
- Path normalization + denylist/allowlist перед любой remote командой.
- Структурные логи без утечки секретов и private key path contents.

## 5) Observability и SLO

### Метрики

- `editor_remote_read_latency_ms` (p50/p95/p99)
- `editor_remote_write_latency_ms`
- `deploy_success_total` / `deploy_failed_total`
- `deploy_duration_ms`
- `inventory_found_total` / `inventory_failed_total`

### Логи (минимальный контракт)

- `request_id`
- `domain_id`
- `project_id`
- `server_id`
- `operation`
- `status`
- `error_code`

## 6) Порядок реализации (микро-шаги)

1. `R1`: migration + config scaffold.
2. `R2`: `inventory_legacy_sites` dry-run + apply.
3. `R3`: `SiteContentBackend` + `LocalFSBackend` адаптер.
4. `R4`: `SSHBackend` + pool + smoke tests.
5. `R5`: Redis cache + invalidation for editor tree.
6. `R6`: `SSHPublisher` + pipeline switch by `deployment_mode`.
7. `R7`: preflight deploy readiness + canary rollout.

## 7) Риски и контрмеры

1. Риск: медленный/нестабильный remote FS.
- Контрмера: timeout, retry, circuit-breaker-like temporary disable target.

2. Риск: неверный `published_path`.
- Контрмера: inventory confidence + ручной override + preflight check.

3. Риск: права (`permission denied`) после деплоя.
- Контрмера: post-deploy ownership verification + explicit alert.

4. Риск: перегрузка сервера при list-tree.
- Контрмера: Redis cache + debounce refresh в UI.

5. Риск: бан по SSH (Fail2Ban/MaxStartups) при пиковом подключении.
- Контрмера: lazy-open pool, `max_open` низкий (например 3-5), ограничение burst reconnect.

6. Риск: временное удвоение дискового пространства при `double rename` (target + backup).
- Контрмера: pre-deploy проверка свободного места на target FS + алерты по low-disk + быстрый cleanup backup.

## 8) Критерий “готово к прод”

- `local_mock` и `ssh_remote` оба проходят regression.
- Canary batch завершён без критичных инцидентов.
- Для `ssh_remote`:
  - editor CRUD стабилен
  - publish стабилен
  - preflight блокирует невалидные запуски
- Документация обновлена: runbook + incident playbook + rollback.

## 9) Backlog R1 (implementation-ready)

Цель `R1`: подготовить схему БД и config scaffold без изменения runtime-поведения (`DEPLOY_MODE` по умолчанию остаётся `local_mock`).

### R1.1 Migration: поля inventory/owner в domains

Файлы:
- `pbn-generator/internal/db/migrations/202603xx_domains_inventory_and_owner.sql` (новый)
- `pbn-generator/internal/db/db.go` (если нужно подключение migration runner в текущей схеме)
- `pbn-generator/internal/store/sqlstore/project.go`
- `pbn-generator/internal/store/sqlstore/domain_test.go`

Изменения:
- добавить в `domains`:
  - `site_owner TEXT NULL`
  - `inventory_status TEXT NULL`
  - `inventory_checked_at TIMESTAMPTZ NULL`
  - `inventory_error TEXT NULL`
- обновить `Domain` struct + scan/select запросы в store.
- зафиксировать стратегию rollback:
  - если migration runner остаётся up-only: добавить companion rollback SQL в `docs/db-rollbacks/`;
  - если подключаем инструменты с down-migrations: добавить `down` для новых колонок.

Приёмка:
- миграции применяются на чистой БД и на существующей (idempotent).
- `go test ./internal/store/sqlstore` green.

### R1.2 Config scaffold для remote deploy

Файлы:
- `pbn-generator/internal/config/config.go`
- `.env` (пример)
- `.env.prod` (пример)
- `README.md` (секция env, кратко)

Изменения:
- новые переменные:
  - `DEPLOY_MODE` (`local_mock|ssh_remote`)
  - `DEPLOY_TIMEOUT` (duration)
  - `DEPLOY_MAX_PARALLEL` (int)
  - `DEPLOY_STAGING_STRATEGY` (`co_located`)
  - `DEPLOY_STAGING_DIR_NAME` (например `.tmp_deploy`)
  - `DEPLOY_TARGETS_JSON` (string, optional)
  - `DEPLOY_KNOWN_HOSTS_PATH` (path, для host key verification)
  - `DEPLOY_SSH_POOL_MAX_OPEN` (int)
  - `DEPLOY_SSH_POOL_MAX_IDLE` (int)
  - `DEPLOY_SSH_POOL_IDLE_TTL` (duration)
- валидация:
  - при `DEPLOY_MODE=local_mock` новые поля optional;
  - при `DEPLOY_MODE=ssh_remote` mandatory минимум timeout/max_parallel/staging strategy.
  - `DEPLOY_TARGETS_JSON` проверять через `json.Valid(...)` (fail-fast на старте).
  - `DEPLOY_TIMEOUT` парсить в `time.Duration` через `time.ParseDuration`.
  - для `ssh_remote` валидировать права файла приватного ключа (`0600`) при старте.
  - для `ssh_remote` валидировать доступность и формат `known_hosts` (или pinned host key), fail-fast.

Приёмка:
- `go test ./internal/config` (или смежные пакеты, где конфиг парсится) green.
- старт сервера в `local_mock` не меняет поведение.

### R1.3 Non-functional guardrails

Файлы:
- `Plan_integrate_real_server_and_inventary.md` (текущий файл)
- при необходимости `docs/` runbook-черновик

Изменения:
- зафиксировать явный rollback: переключение `DEPLOY_MODE=local_mock`.
- зафиксировать, что `R1` не включает SSH IO runtime.
- зафиксировать операционный чек: приватные ключи для `ssh_remote` должны быть `0600` перед rollout.

Приёмка:
- в документе нет двусмысленностей по scope `R1`.

### Verify-команды для R1

- `go test ./internal/store/sqlstore`
- `go test ./internal/httpserver`
- `go test ./cmd/worker`
- (при добавлении отдельного пакета) `go test ./internal/config`

### Коммит R1 (целевой)

- `feat(deploy): add domains inventory schema and deploy config scaffold`

### Owner и срок

- Owner: backend (`@owner-backend`)
- Review: infra (`@owner-infra`)
- Target due-date: `2026-03-03`
