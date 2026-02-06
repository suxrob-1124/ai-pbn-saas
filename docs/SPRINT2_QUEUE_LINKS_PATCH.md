# Sprint 2: Queue + Links Business Patch

Этот файл описывает план патча по бизнес‑логике очередей и ссылок. Реализация будет разбита на отдельные коммиты (см. в конце).

## DB

- [x] **Миграция: domain link fields**
  - Таблица `domains`: `link_anchor_text`, `link_acceptor_url`, `link_status`, `link_updated_at`, `link_last_task_id`
  - Дополнительно: `link_file_path` и `link_anchor_snapshot` (для точной замены)

- [x] **Миграция: schedules metadata**
  - Таблица `generation_schedules`: `last_run_at`, `next_run_at`, `timezone`

- [x] **Миграция: link_schedules metadata**
  - Таблица `link_schedules`: `last_run_at`, `next_run_at`, `timezone`

- [x] **Миграция: уникальность расписаний**
  - `generation_schedules.project_id` UNIQUE
  - `link_schedules.project_id` UNIQUE

## Backend

- [x] **Domain API: link settings**
  - `PATCH /api/domains/:id` принимает `link_anchor_text`, `link_acceptor_url`
  - `domainDTO` отдаёт link‑поля

- [x] **Link run endpoint**
  - `POST /api/domains/:id/link/run`
  - Upsert link task по домену (одна активная задача)

- [x] **Link worker: replace‑логика (без HTML‑маркеров)**
  - Опора на данные в БД: `domains.link_last_task_id` и `link_tasks.found_location`
  - При обновлении сначала пытаемся заменить ранее вставленную ссылку:
    - берём предыдущий task (anchor/target + found_location)
    - открываем файл из found_location и заменяем ссылку по старым данным
  - Если не найдено — поиск по всем HTML (по старому anchor/target), затем по новому anchor
  - Если якорь не найден — генерируем параграф и вставляем

- [x] **Scheduler: single schedule + next run**
  - 1 расписание на проект
  - Ручной запуск не блокирует авто‑расписание
  - `next_run_at` и `last_run_at` используются вместо queue history
  - Принять `weekday` и `day` из config

- [x] **Scheduler: links**
  - 1 расписание ссылок на проект
  - Отбор доменов: `published_path IS NOT NULL` (или `status='published'`)
  - Отбор доменов: заполнены `link_anchor_text` + `link_acceptor_url`
  - Upsert link task для каждого домена по лимиту

- [x] **Link schedules API**
  - Эндпоинт: `GET /api/projects/:id/link-schedule`
  - Эндпоинт: `PUT /api/projects/:id/link-schedule` (upsert, одно расписание на проект)
  - Эндпоинт: `DELETE /api/projects/:id/link-schedule` (удалить/деактивировать)
  - Эндпоинт: `POST /api/projects/:id/link-schedule/trigger` (ручной запуск, не блокирует авто‑расписание)
  - Входные поля:
    - `name` (string, required)
    - `config` (object, required)
    - `is_active` (bool, optional, default true)
    - `timezone` (string, optional, default `UTC` или проектный TZ)
  - `config` поддерживает:
    - `limit` (int > 0)
    - `time` (HH:mm)
    - `weekday` (mon..sun) или `day` (1..31)
    - `cron` (string) или `interval` (например, `1d`)
  - Поведение:
    - при upsert пересчитываем `next_run_at`
    - `last_run_at` обновляется после фактического запуска
  - Ответ:
    - базовые поля расписания
    - `next_run_at`, `last_run_at`, `timezone`

- [x] **Queue API: домен в ответе**
  - `/api/generations` возвращает `domain_url`

## Frontend

- [x] **Project domains list**
  - Колонки: `Анкор`, `Акцептор`
  - Кнопка: `Добавить ссылку` / `Обновить ссылку`

- [x] **Domain settings**
  - Поля `Анкор` и `Акцептор` в секции «Настройки домена»
  - Удалить вкладку Links и связанные компоненты

- [x] **Schedules UI**
  - Только 1 расписание генерации на проект
  - Только 1 расписание ссылок на проект (отдельный блок)
  - Показ `next_run_at` в списке
  - Подсказка текущего времени и таймзоны возле time‑поля

- [ ] **Queue UI**
  - Общая очередь показывает домен, а не UUID

## Tests

- [x] **Schedules**
  - Создание только одного расписания
  - `next_run_at` и `last_run_at` корректны
  - Ручной запуск не блокирует авто‑запуск
  - Link‑schedule: одно на проект + next/last/timezone

- [x] **Links**
  - Replace‑логика по данным в БД (found_location/last_task)
  - Upsert link task по домену

- [x] **Domain API**
  - PATCH link‑полей
  - `link/run` endpoint

## План коммитов

1. **DB: link fields + schedule metadata**
   - Миграции domains и generation_schedules
   - UNIQUE по расписаниям (generation/link)
   - Обновление `db.go` и `db_test.go`

2. **Backend: schedules single + next_run**
   - Ограничение 1 расписания на проект (API/Store)
   - `next_run_at`/`last_run_at` вычисление
   - Ручной запуск без блокировки авто‑периода
   - Поддержка `day` и `weekday` в config

3. **Backend: links business**
   - `domains` link‑поля в store + DTO
   - `POST /api/domains/:id/link/run`
   - Link worker replace по маркеру
   - Link scheduler по `link_schedules`
   - `/api/generations` + `domain_url`

4. **Frontend: UI changes**
   - Проект: колонки анкор/акцептор + кнопка запуска
   - Домен: link‑поля в «Настройки домена»
   - Удаление вкладки Links
   - Schedules: единичное расписание + next_run_at + таймзона
   - Queue: домены вместо UUID

5. **Tests**
   - Schedule: one‑per‑project, next_run, manual vs auto
   - Link worker: replace behavior
   - Domain API: link‑fields + link/run
