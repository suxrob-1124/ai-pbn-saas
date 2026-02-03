# TODO — Unified Release v2.0-v3.0

## 📌 Overview

Детальный список задач для объединенного релиза v2.0-v3.0 с новой стратегией локального хранения.

---

## 🗄️ Database Changes

### Sprint 1: File Storage

- [ ] **Миграция 001: Create site_files table**
  ```sql
  CREATE TABLE IF NOT EXISTS site_files (
    id TEXT PRIMARY KEY,
    domain_id TEXT NOT NULL REFERENCES domains(id) ON DELETE CASCADE,
    path TEXT NOT NULL,  -- 'index.html', 'css/main.css', 'images/logo.svg'
    content_hash TEXT,   -- SHA256 для обнаружения изменений
    size_bytes BIGINT NOT NULL,
    mime_type TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(domain_id, path)
  );
  CREATE INDEX idx_site_files_domain ON site_files(domain_id);
  ```

- [ ] **Миграция 002: Create file_edits table**
  ```sql
  CREATE TABLE IF NOT EXISTS file_edits (
    id TEXT PRIMARY KEY,
    file_id TEXT NOT NULL REFERENCES site_files(id) ON DELETE CASCADE,
    edited_by TEXT NOT NULL REFERENCES users(email),
    content_before_hash TEXT,
    content_after_hash TEXT,
    edit_type TEXT NOT NULL DEFAULT 'manual',  -- 'manual', 'link_injection', 'ai'
    edit_description TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
  );
  CREATE INDEX idx_file_edits_file ON file_edits(file_id);
  CREATE INDEX idx_file_edits_user ON file_edits(edited_by);
  ```

- [ ] **Миграция 003: Alter domains table**
  ```sql
  ALTER TABLE domains ADD COLUMN IF NOT EXISTS published_path TEXT;  -- '/server/example.com/'
  ALTER TABLE domains ADD COLUMN IF NOT EXISTS file_count INT DEFAULT 0;
  ALTER TABLE domains ADD COLUMN IF NOT EXISTS total_size_bytes BIGINT DEFAULT 0;
  ```

### Sprint 2: Schedulers

- [ ] **Миграция 004: Create generation_schedules table**
  ```sql
  CREATE TABLE IF NOT EXISTS generation_schedules (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    description TEXT,
    strategy TEXT NOT NULL,  -- 'immediate', 'daily', 'weekly', 'custom'
    config JSONB NOT NULL,   -- {"limit": 5, "interval": "1d"} or {"cron": "0 9 * * *"}
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_by TEXT NOT NULL REFERENCES users(email),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
  );
  CREATE INDEX idx_gen_schedules_project ON generation_schedules(project_id);
  CREATE INDEX idx_gen_schedules_active ON generation_schedules(is_active);
  ```

- [ ] **Миграция 005: Create generation_queue table**
  ```sql
  CREATE TABLE IF NOT EXISTS generation_queue (
    id TEXT PRIMARY KEY,
    domain_id TEXT NOT NULL REFERENCES domains(id) ON DELETE CASCADE,
    schedule_id TEXT REFERENCES generation_schedules(id) ON DELETE SET NULL,
    priority INT NOT NULL DEFAULT 0,
    scheduled_for TIMESTAMPTZ NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',  -- 'pending', 'queued', 'completed', 'failed'
    error_message TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    processed_at TIMESTAMPTZ
  );
  CREATE INDEX idx_gen_queue_scheduled ON generation_queue(scheduled_for, status);
  CREATE INDEX idx_gen_queue_domain ON generation_queue(domain_id);
  CREATE INDEX idx_gen_queue_status ON generation_queue(status);
  ```

- [ ] **Миграция 006: Create link_tasks table**
  ```sql
  CREATE TABLE IF NOT EXISTS link_tasks (
    id TEXT PRIMARY KEY,
    domain_id TEXT NOT NULL REFERENCES domains(id) ON DELETE CASCADE,
    anchor_text TEXT NOT NULL,
    target_url TEXT NOT NULL,
    scheduled_for TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    status TEXT NOT NULL DEFAULT 'pending',  -- 'pending', 'searching', 'found', 'inserted', 'generated', 'failed'
    found_location TEXT,           -- 'index.html:line 45'
    generated_content TEXT,        -- LLM generated paragraph if anchor not found
    error_message TEXT,
    attempts INT NOT NULL DEFAULT 0,
    created_by TEXT NOT NULL REFERENCES users(email),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ
  );
  CREATE INDEX idx_link_tasks_domain ON link_tasks(domain_id);
  CREATE INDEX idx_link_tasks_status ON link_tasks(status);
  CREATE INDEX idx_link_tasks_scheduled ON link_tasks(scheduled_for, status);
  ```

- [ ] **Миграция 007: Create link_schedules table**
  ```sql
  CREATE TABLE IF NOT EXISTS link_schedules (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    config JSONB NOT NULL,  -- {"cron": "0 14 * * *"}
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_by TEXT NOT NULL REFERENCES users(email),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
  );
  ```

### Sprint 3: Index Checker

- [ ] **Миграция 008: Create domain_index_checks table**
  ```sql
  CREATE TABLE IF NOT EXISTS domain_index_checks (
    id TEXT PRIMARY KEY,
    domain_id TEXT NOT NULL REFERENCES domains(id) ON DELETE CASCADE,
    check_date DATE NOT NULL,          -- Дата проверки (один раз в день)
    status TEXT NOT NULL DEFAULT 'pending',  -- 'pending', 'checking', 'success', 'failed_investigation'
    is_indexed BOOLEAN,                -- NULL если еще не определено, TRUE/FALSE после проверки
    attempts INT NOT NULL DEFAULT 0,   -- Количество попыток за день
    last_attempt_at TIMESTAMPTZ,
    next_retry_at TIMESTAMPTZ,         -- Когда следующий retry
    error_message TEXT,
    completed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(domain_id, check_date)
  );
  CREATE INDEX idx_index_checks_domain ON domain_index_checks(domain_id);
  CREATE INDEX idx_index_checks_date ON domain_index_checks(check_date);
  CREATE INDEX idx_index_checks_status ON domain_index_checks(status);
  CREATE INDEX idx_index_checks_retry ON domain_index_checks(next_retry_at) WHERE status = 'checking';
  ```

- [ ] **Миграция 009: Create index_check_history table**
  ```sql
  CREATE TABLE IF NOT EXISTS index_check_history (
    id TEXT PRIMARY KEY,
    check_id TEXT NOT NULL REFERENCES domain_index_checks(id) ON DELETE CASCADE,
    attempt_number INT NOT NULL,
    result TEXT,  -- 'success', 'error', 'timeout'
    response_data JSONB,
    error_message TEXT,
    duration_ms INT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
  );
  CREATE INDEX idx_check_history_check ON index_check_history(check_id);
  ```

---

## 🔧 Backend (Go) Tasks

### Sprint 1: Local Storage & File API

#### Publisher Module

- [ ] **Создать интерфейс Publisher**
  - Файл: `internal/publisher/publisher.go`
  - Методы:
    - `Publish(ctx, domainID, files map[string][]byte) error`
    - `Unpublish(ctx, domainID) error`
    - `GetPublishedPath(domainID) string`

- [ ] **Реализовать LocalPublisher**
  - Файл: `internal/publisher/local.go`
  - Логика:
    - Создать папку `server/{domain_name}/`
    - Распаковать ZIP из artifacts
    - Сохранить все файлы в папку
    - Обновить `domains.published_path`
  - Валидация:
    - Проверка существования папки server/
    - Sanitize domain name (защита от Path Traversal)

- [ ] **Обновить Worker Pipeline**
  - Файл: `internal/worker/step_publish.go`
  - После step_assembly вызвать `Publisher.Publish()`
  - Сохранить published_path в domains
  - Обновить статус домена на `published`

#### File Storage Module

- [ ] **Создать Store для site_files**
  - Файл: `internal/store/sqlstore/site_files.go`
  - Интерфейс `SiteFileStore`:
    - `Create(ctx, file) error`
    - `Get(ctx, fileID) (*SiteFile, error)`
    - `GetByPath(ctx, domainID, path) (*SiteFile, error)`
    - `List(ctx, domainID) ([]SiteFile, error)`
    - `Update(ctx, fileID, content) error`
    - `Delete(ctx, fileID) error`
    - `UpdateHash(ctx, fileID, hash) error`

- [ ] **Создать Store для file_edits**
  - Файл: `internal/store/sqlstore/file_edits.go`
  - Интерфейс `FileEditStore`:
    - `Create(ctx, edit) error`
    - `ListByFile(ctx, fileID, limit) ([]FileEdit, error)`
    - `ListByUser(ctx, userEmail, limit) ([]FileEdit, error)`

- [ ] **Синхронизация файлов с БД**
  - Файл: `internal/publisher/sync.go`
  - После публикации:
    - Scan папки `server/{domain}/`
    - Для каждого файла создать запись в `site_files`
    - Вычислить SHA256 hash
    - Определить MIME type

#### File API

- [ ] **Эндпоинт: GET /api/domains/:id/files**
  - Получить список всех файлов домена
  - Авторизация: viewer и выше
  - Ответ: `[{id, path, size, mimeType, updatedAt}]`

- [ ] **Эндпоинт: GET /api/domains/:id/files/*path**
  - Получить содержимое конкретного файла
  - Авторизация: viewer и выше
  - Читать из `server/{domain}/{path}`
  - Ответ: `{content: string, mimeType: string}`

- [ ] **Эндпоинт: PUT /api/domains/:id/files/*path**
  - Сохранить изменения файла
  - Авторизация: editor и выше
  - Валидация:
    - Path Traversal защита
    - Проверка MIME type
  - Логика:
    - Сохранить старый контент в `file_edits`
    - Записать новый контент на диск
    - Обновить `site_files.updated_at` и `content_hash`

- [ ] **Эндпоинт: GET /api/domains/:id/files/:fileId/history**
  - История изменений конкретного файла
  - Авторизация: viewer и выше
  - Ответ: `[{id, editedBy, editType, description, createdAt}]`

- [ ] **Эндпоинт: DELETE /api/domains/:id/files/*path**
  - Удалить файл (только для admin)
  - Удалить с диска и из `site_files`

### Sprint 2: Schedulers

#### Generation Scheduler

- [ ] **Создать Store для schedules**
  - Файл: `internal/store/sqlstore/schedules.go`
  - Интерфейс `ScheduleStore`:
    - `Create(ctx, schedule) error`
    - `Get(ctx, scheduleID) (*Schedule, error)`
    - `List(ctx, projectID) ([]Schedule, error)`
    - `Update(ctx, scheduleID, updates) error`
    - `Delete(ctx, scheduleID) error`
    - `ListActive(ctx) ([]Schedule, error)`

- [ ] **Создать Store для generation_queue**
  - Файл: `internal/store/sqlstore/gen_queue.go`
  - Интерфейс `GenQueueStore`:
    - `Enqueue(ctx, queueItem) error`
    - `GetPending(ctx, limit) ([]QueueItem, error)`
    - `MarkProcessed(ctx, itemID, status, error) error`
    - `ListByDomain(ctx, domainID) ([]QueueItem, error)`

- [ ] **API для Generation Schedules**
  - `POST /api/projects/:id/schedules` - создать
  - `GET /api/projects/:id/schedules` - список
  - `PATCH /api/projects/:id/schedules/:scheduleId` - обновить
  - `DELETE /api/projects/:id/schedules/:scheduleId` - удалить
  - `POST /api/projects/:id/schedules/:scheduleId/trigger` - запуск вручную

- [ ] **API для Queue**
  - `GET /api/projects/:id/queue` - просмотр очереди
  - `DELETE /api/queue/:itemId` - удалить из очереди

- [ ] **Scheduler Worker**
  - Файл: `cmd/scheduler/main.go` или расширить `cmd/worker/`
  - Регистрация Cron задач через Asynq:
    - Каждую минуту проверять `generation_queue`
    - Берет items где `scheduled_for <= NOW()` и `status = 'pending'`
    - Ставит задачу в Asynq: `tasks.GenerateSite`
    - Обновляет статус на `queued`
  - Обработка стратегий:
    - `immediate`: добавить все домены в очередь сразу
    - `daily`: добавлять N доменов каждый день в указанное время
    - `weekly`: добавлять N доменов раз в неделю
    - `custom`: парсить cron выражение и добавлять по расписанию

#### Link Building

- [ ] **Создать модуль linkbuilder**
  - Файл: `internal/linkbuilder/linkbuilder.go`
  - Интерфейс:
    - `ProcessTask(ctx, taskID) error`
    - `FindAnchor(htmlContent, anchorText) (position int, found bool)`
    - `InsertLink(htmlContent, position, anchorText, targetURL) string`
    - `GenerateContent(ctx, anchorText, targetURL, context) (string, error)`

- [ ] **Создать Store для link_tasks**
  - Файл: `internal/store/sqlstore/link_tasks.go`
  - Интерфейс `LinkTaskStore`:
    - `Create(ctx, task) error`
    - `Get(ctx, taskID) (*LinkTask, error)`
    - `ListByDomain(ctx, domainID, filters) ([]LinkTask, error)`
    - `ListPending(ctx, limit) ([]LinkTask, error)`
    - `Update(ctx, taskID, updates) error`
    - `Delete(ctx, taskID) error`

- [ ] **API для Link Tasks**
  - `POST /api/domains/:id/links` - создать задачу
  - `POST /api/domains/:id/links/import` - CSV импорт
  - `GET /api/domains/:id/links` - список задач домена
  - `GET /api/links` - все задачи (с фильтрами)
  - `PATCH /api/links/:id` - обновить (scheduled_for)
  - `DELETE /api/links/:id` - удалить
  - `POST /api/links/:id/retry` - повторить задачу

- [ ] **Link Building Worker**
  - Файл: `internal/worker/link_worker.go`
  - Asynq task: `tasks.ProcessLinkTask`
  - Логика:
    1. Получить task из БД
    2. Получить путь к папке домена (`server/{domain}/`)
    3. Загрузить все HTML файлы
    4. Найти anchor_text в HTML
    5. Если найден:
       - Вставить `<a href="target_url">anchor_text</a>`
       - Сохранить изменения на диск
       - Создать запись в `file_edits`
       - Статус: `inserted`
    6. Если не найден:
       - Запросить LLM: генерация параграфа с анкором
       - Вставить сгенерированный контент
       - Сохранить на диск
       - Статус: `generated`
    7. Если ошибка: статус `failed`, сохранить error_message

- [ ] **Scheduler для Link Tasks**
  - Cron задача: каждую минуту
  - Берет tasks где `scheduled_for <= NOW()` и `status = 'pending'`
  - Ставит в Asynq очередь: `tasks.ProcessLinkTask`

### Sprint 3: Index Checker

- [ ] **Создать модуль indexchecker**
  - Файл: `internal/indexchecker/checker.go`
  - Интерфейс `IndexChecker`:
    - `Check(ctx, domain string) (indexed bool, err error)`
  - Реализации:
    - `MockChecker` - всегда возвращает `false`
    - `RealChecker` - заглушка для будущей интеграции

- [ ] **Создать Store для index_checks**
  - Файл: `internal/store/sqlstore/index_checks.go`
  - Интерфейс `IndexCheckStore`:
    - `Create(ctx, check) error`
    - `Get(ctx, checkID) (*IndexCheck, error)`
    - `GetByDomainAndDate(ctx, domainID, date) (*IndexCheck, error)`
    - `ListByDomain(ctx, domainID, limit) ([]IndexCheck, error)`
    - `ListPendingRetries(ctx) ([]IndexCheck, error)`
    - `UpdateStatus(ctx, checkID, status, isIndexed, error) error`
    - `IncrementAttempts(ctx, checkID) error`
    - `SetNextRetry(ctx, checkID, nextRetry time.Time) error`

- [ ] **Создать Store для check_history**
  - Файл: `internal/store/sqlstore/check_history.go`
  - Методы для логирования каждой попытки

- [ ] **Retry Logic**
  - Файл: `internal/indexchecker/retry.go`
  - Функция `CalculateNextRetry(attempts int) time.Duration`:
    - Attempt 1: 30 min
    - Attempt 2: 1 hour
    - Attempt 3: 2 hours
    - Attempt 4: 4 hours
    - Max: 8 attempts в сутки
  - Функция `ShouldRetry(check) bool`:
    - Проверить количество попыток
    - Проверить время с момента создания (< 24 часа)

- [ ] **Index Checker Worker**
  - Файл: `cmd/indexchecker/main.go` или расширить worker
  - Cron задача: каждый час
  - Логика:
    1. Создать pending checks для всех domains (если нет за сегодня)
    2. Получить список pending/checking checks с `next_retry_at <= NOW()`
    3. Для каждого:
       - Вызвать `IndexChecker.Check(domain)`
       - Логировать попытку в `index_check_history`
       - Если успех (получен четкий ответ):
         * Статус: `success`
         * Установить `is_indexed`
         * `completed_at = NOW()`
       - Если ошибка:
         * Инкремент `attempts`
         * Если `ShouldRetry()`:
           - Вычислить `next_retry_at`
           - Статус остается `checking`
         * Иначе (24 часа прошло или превышен лимит):
           - Статус: `failed_investigation`
           - Алерт администратору

- [ ] **API для Index Checks**
  - `GET /api/domains/:id/index-checks` - история проверок
  - `POST /api/domains/:id/index-checks` - запустить вручную
  - `GET /api/admin/index-checks` - все проверки
  - `GET /api/admin/index-checks/failed` - проблемные

---

## 🎨 Frontend (Next.js) Tasks

### Sprint 1: File Editor Foundation

- [ ] **Установить Monaco Editor**
  - Package: `@monaco-editor/react`
  - Настроить TypeScript types

- [ ] **Создать API клиент для файлов**
  - Файл: `lib/fileApi.ts`
  - Методы:
    - `listFiles(domainId)`
    - `getFile(domainId, path)`
    - `saveFile(domainId, path, content)`
    - `getFileHistory(fileId)`

### Sprint 4: UI Integration

#### File Editor Page

- [ ] **Страница: /domains/[id]/editor**
  - Файл: `app/domains/[id]/editor/page.tsx`
  - Компоненты:
    - `FileTree` - дерево файлов (sidebar)
    - `MonacoEditor` - редактор кода
    - `EditorToolbar` - кнопки Save, Revert, Download
    - `FileHistory` - история изменений (modal/drawer)

- [ ] **Компонент: FileTree**
  - Файл: `components/FileTree.tsx`
  - Props: `files: FileNode[]`, `onSelect: (file) => void`
  - Рекурсивная структура для вложенных папок
  - Иконки для типов файлов (HTML, CSS, JS, изображения)

- [ ] **Компонент: MonacoEditor**
  - Файл: `components/MonacoEditor.tsx`
  - Props: `content, language, onChange, readOnly`
  - Настройка:
    - Подсветка синтаксиса (HTML, CSS, JS)
    - Minimap
    - Line numbers
    - Auto-completion
  - Автосохранение (debounce 2 секунды)

- [ ] **Компонент: FileHistory**
  - Файл: `components/FileHistory.tsx`
  - Список изменений:
    - Кто изменил
    - Когда
    - Тип изменения
  - Кнопка "View Diff" (Monaco Diff Editor)

#### Scheduler UI

- [ ] **Вкладка Schedules на странице проекта**
  - Файл: `app/projects/[id]/page.tsx` - добавить вкладку
  - Компоненты:
    - `ScheduleList` - таблица расписаний
    - `ScheduleForm` - форма создания/редактирования
    - `ScheduleTrigger` - кнопка запуска вручную

- [ ] **Компонент: ScheduleForm**
  - Файл: `components/ScheduleForm.tsx`
  - Поля:
    - Name (text input)
    - Strategy (select: immediate, daily, weekly, custom)
    - Config (условно показываем разные поля):
      - daily: limit (number), start time (time picker)
      - weekly: limit (number), day (select), time (time picker)
      - custom: cron expression (text with hint)
    - Active (checkbox)
  - Валидация:
    - Проверка cron выражения (библиотека cron-parser)
    - Limit > 0

- [ ] **Компонент: ScheduleList**
  - Файл: `components/ScheduleList.tsx`
  - Таблица:
    - Название, стратегия, статус (active/inactive)
    - Кнопки: Edit, Delete, Trigger Now
  - Модалка подтверждения удаления

- [ ] **Страница: /projects/[id]/queue**
  - Файл: `app/projects/[id]/queue/page.tsx`
  - Таблица generation_queue:
    - Домен, время запуска, статус, приоритет
    - Фильтры: по статусу, по дате
    - Кнопка "Remove from Queue"

#### Link Building UI

- [ ] **Вкладка Links на странице домена**
  - Файл: `app/domains/[id]/page.tsx` - добавить вкладку
  - Компоненты:
    - `LinkTaskList` - таблица задач
    - `LinkTaskForm` - форма добавления
    - `CSVImport` - drag & drop CSV

- [ ] **Компонент: LinkTaskForm**
  - Файл: `components/LinkTaskForm.tsx`
  - Поля:
    - Anchor Text (text input)
    - Target URL (URL input с валидацией)
    - Scheduled For (datetime picker)
  - Кнопки: Save, Save & Add Another

- [ ] **Компонент: LinkTaskList**
  - Файл: `components/LinkTaskList.tsx`
  - Таблица:
    - Анкор, URL, статус, время
    - Цветовые индикаторы статуса:
      - pending: серый
      - searching: синий
      - inserted: зеленый
      - generated: желтый
      - failed: красный
  - Фильтры: по статусу
  - Кнопки для каждого task: Retry, Edit, Delete
  - Массовые действия: Bulk Retry, Bulk Delete

- [ ] **Компонент: CSVImport**
  - Файл: `components/CSVImport.tsx`
  - Drag & drop зона
  - Парсинг CSV:
    - Формат: `anchor_text,target_url,scheduled_for`
    - Валидация каждой строки
  - Preview перед импортом
  - Кнопка "Import All"

#### Index Monitoring Dashboard

- [ ] **Страница: /monitoring/indexing**
  - Файл: `app/monitoring/indexing/page.tsx`
  - Компоненты:
    - `IndexCalendar` - календарь с индикаторами
    - `IndexTable` - таблица проверок
    - `IndexStats` - статистика и графики
    - `FailedChecksAlert` - алерты

- [ ] **Компонент: IndexCalendar**
  - Файл: `components/IndexCalendar.tsx`
  - Библиотека: react-calendar или кастомный
  - День = ячейка с индикатором:
    - Зеленый: indexed = true
    - Красный: indexed = false
    - Желтый: checking/pending
    - Серый: failed_investigation
  - Tooltip при hover: детали проверки

- [ ] **Компонент: IndexTable**
  - Файл: `components/IndexTable.tsx`
  - Таблица:
    - Домен, дата, статус, попытки, результат, время последней попытки
  - Фильтры:
    - По домену (select/autocomplete)
    - По статусу (multi-select)
    - По дате (date range picker)
  - Пагинация
  - Сортировка по колонкам

- [ ] **Компонент: IndexStats**
  - Файл: `components/IndexStats.tsx`
  - Метрики:
    - Процент индексации за последние 30 дней
    - Среднее количество попыток до успеха
    - Количество failed_investigation за неделю
  - Графики (библиотека recharts):
    - Line chart: процент индексации по дням
    - Bar chart: количество проверок в день

- [ ] **Компонент: FailedChecksAlert**
  - Файл: `components/FailedChecksAlert.tsx`
  - Alert banner вверху страницы
  - Показывать количество failed_investigation
  - Кнопка "View Details" → фильтр таблицы

#### Server Folder Browser (Admin)

- [ ] **Страница: /admin/server**
  - Файл: `app/admin/server/page.tsx`
  - Только для admin роли
  - Компоненты:
    - `ServerFolderList` - список папок

- [ ] **Компонент: ServerFolderList**
  - Файл: `components/ServerFolderList.tsx`
  - Таблица:
    - Домен, размер на диске, количество файлов, дата последнего изменения
  - Для каждого:
    - Кнопка "Open Editor" → redirect to `/domains/[id]/editor`
    - Кнопка "Download ZIP" - сгенерировать и скачать
    - Кнопка "Delete" - с подтверждением

---

## 🧪 Testing Tasks

### Sprint 1: Local Storage Tests

- [ ] **Unit тесты для LocalPublisher**
  - Создание папки
  - Распаковка ZIP
  - Path Traversal защита

- [ ] **Integration тесты для File API**
  - Получение списка файлов
  - Чтение файла
  - Сохранение изменений
  - История изменений

### Sprint 2: Scheduler Tests

- [ ] **Unit тесты для Scheduler Store**
  - CRUD операции
  - Получение активных расписаний

- [ ] **Unit тесты для Link Builder**
  - Поиск анкора в HTML
  - Вставка ссылки
  - LLM генерация (mock)

- [ ] **Integration тесты для Scheduler Worker**
  - Постановка задач в очередь по времени
  - Обработка разных стратегий

### Sprint 3: Index Checker Tests

- [ ] **Unit тесты для Retry Logic**
  - Расчет next_retry_at
  - Проверка ShouldRetry()

- [ ] **Unit тесты для MockChecker**
  - Всегда возвращает false

- [ ] **Integration тесты для Index Worker**
  - Создание pending checks
  - Обработка retry
  - Установка failed_investigation

### Sprint 4: E2E Tests

- [ ] **E2E: File Editor Flow**
  - Открыть editor
  - Выбрать файл
  - Отредактировать
  - Сохранить
  - Проверить изменения на диске

- [ ] **E2E: Schedule Generation**
  - Создать расписание
  - Дождаться срабатывания
  - Проверить creation generation в очереди

- [ ] **E2E: Link Injection**
  - Создать link task
  - Дождаться обработки
  - Проверить вставку ссылки в HTML

---

## 📋 Additional Tasks

### Documentation

- [ ] Обновить README с новой архитектурой
- [ ] Документировать API эндпоинты (OpenAPI/Swagger)
- [ ] Создать руководство по настройке Cron задач
- [ ] Документировать формат CSV для link import

### Infrastructure

- [ ] Создать папку `server/` в корне проекта (`.gitignore`)
- [ ] Настроить backup папки `server/`
- [ ] Добавить healthcheck для scheduler worker
- [ ] Настроить алерты для failed_investigation

### Security Review

- [ ] Аудит Path Traversal защиты
- [ ] Проверка прав доступа к редактору (только editor+)
- [ ] Валидация cron выражений (защита от инъекций)
- [ ] Rate limiting для file API

---

## ✅ Definition of Done

Каждая задача считается завершенной когда:
- [ ] Код написан и работает локально
- [ ] Unit тесты покрывают основную логику
- [ ] API документирован (комментарии в коде)
- [ ] UI компонент отзывчивый и работает на мобильных
- [ ] Проведен code review
- [ ] Миграции применены без ошибок
- [ ] Нет критичных багов

---

## 🚨 Критические риски

1. **Сложность Retry логики** → Выделить больше времени на Sprint 3
2. **Monaco Editor performance** → Fallback на простой textarea
3. **Cron parsing** → Использовать проверенную библиотеку (robfig/cron)
4. **Disk I/O** → Мониторить производительность, добавить кэширование
