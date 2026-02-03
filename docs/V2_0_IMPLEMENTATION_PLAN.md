# План реализации v2.0 — Планировщик и Link Building

**Дата создания:** 2025-01-XX  
**Статус:** Планирование  
**Оценка времени:** 6-8 недель

---

## 📋 Обзор

v2.0 добавляет два ключевых компонента:
1. **Планировщик генерации (Scheduler)** — гибкое управление очередью генераций
2. **Link Building** — пост-обработка сайтов для добавления ссылок

---

## 🎯 Цель этапа

**Планировщик:** Позволить пользователям настраивать автоматический запуск генераций по расписанию (35 сайтов в неделю, 5 в день, или всё сразу).

**Link Building:** Автоматизировать добавление ссылок на сайты после генерации — поиск анкоров и вставка ссылок, или генерация нового контента через LLM.

---

## 📦 Компонент 1: Планировщик генерации (Scheduler)

### 1.1 База данных

**Миграция:** `pbn-generator/internal/db/migrations/XXXX_add_scheduler_tables.sql`

```sql
-- Расписания генерации для проектов
CREATE TABLE IF NOT EXISTS generation_schedules (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    strategy TEXT NOT NULL CHECK (strategy IN ('immediate', 'daily', 'weekly', 'custom')),
    config JSONB NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_generation_schedules_project ON generation_schedules(project_id);
CREATE INDEX idx_generation_schedules_active ON generation_schedules(is_active) WHERE is_active = TRUE;

-- Очередь запланированных генераций
CREATE TABLE IF NOT EXISTS generation_queue (
    id TEXT PRIMARY KEY,
    domain_id TEXT NOT NULL REFERENCES domains(id) ON DELETE CASCADE,
    schedule_id TEXT REFERENCES generation_schedules(id) ON DELETE SET NULL,
    priority INT NOT NULL DEFAULT 0,
    scheduled_for TIMESTAMPTZ NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'queued', 'processing', 'completed', 'failed', 'cancelled')),
    error TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_generation_queue_scheduled ON generation_queue(scheduled_for, status);
CREATE INDEX idx_generation_queue_domain ON generation_queue(domain_id);
CREATE INDEX idx_generation_queue_schedule ON generation_queue(schedule_id);
CREATE INDEX idx_generation_queue_status ON generation_queue(status);
```

**Структура `config` JSONB:**
- `immediate`: `{}` (пустой объект)
- `daily`: `{ "limit": 5, "interval": "1d", "time": "09:00" }` (время в UTC)
- `weekly`: `{ "limit": 35, "interval": "7d", "day": 1, "time": "09:00" }` (day: 1=Monday, 7=Sunday)
- `custom`: `{ "cron": "0 9 * * 1" }` (cron выражение)

### 1.2 Store слой

**Файл:** `pbn-generator/internal/store/sqlstore/schedule.go`

```go
type Schedule struct {
    ID        string
    ProjectID string
    Name      string
    Strategy  string // 'immediate', 'daily', 'weekly', 'custom'
    Config    json.RawMessage
    IsActive  bool
    CreatedAt time.Time
    UpdatedAt time.Time
}

type ScheduleStore interface {
    Create(ctx context.Context, s Schedule) error
    Get(ctx context.Context, id string) (Schedule, error)
    ListByProject(ctx context.Context, projectID string) ([]Schedule, error)
    Update(ctx context.Context, s Schedule) error
    Delete(ctx context.Context, id string) error
    ListActive(ctx context.Context) ([]Schedule, error)
}

type QueueItem struct {
    ID          string
    DomainID    string
    ScheduleID  sql.NullString
    Priority    int
    ScheduledFor time.Time
    Status      string
    Error       sql.NullString
    CreatedAt   time.Time
    UpdatedAt   time.Time
}

type QueueStore interface {
    Create(ctx context.Context, item QueueItem) error
    Get(ctx context.Context, id string) (QueueItem, error)
    ListPending(ctx context.Context, before time.Time) ([]QueueItem, error)
    UpdateStatus(ctx context.Context, id, status string, err error) error
    ListByDomain(ctx context.Context, domainID string) ([]QueueItem, error)
    ListBySchedule(ctx context.Context, scheduleID string) ([]QueueItem, error)
    Delete(ctx context.Context, id string) error
}
```

**Задачи:**
- [ ] Создать `schedule.go` с реализацией `ScheduleStore`
- [ ] Создать методы для работы с `generation_queue`
- [ ] Добавить unit-тесты для store

### 1.3 Scheduler Worker

**Файл:** `pbn-generator/cmd/scheduler/main.go` (новый процесс) или интегрировать в `cmd/worker/main.go`

**Логика:**
1. Каждую минуту проверяет `generation_queue` на задачи со `scheduled_for <= NOW()` и `status = 'pending'`
2. Для каждой задачи:
   - Создает `Generation` запись в БД
   - Ставит задачу в Asynq очередь через `tasks.NewGenerateTask()`
   - Обновляет статус в `generation_queue` на `'queued'`
3. При ошибке обновляет статус на `'failed'` с текстом ошибки

**Интеграция с существующим worker:**
- Worker при завершении генерации обновляет статус в `generation_queue` на `'completed'` или `'failed'`

**Задачи:**
- [ ] Создать `cmd/scheduler/main.go` или добавить в `cmd/worker/main.go`
- [ ] Реализовать функцию `processScheduledTasks()`
- [ ] Добавить логирование и метрики
- [ ] Добавить graceful shutdown

### 1.4 API для управления расписаниями

**Файл:** `pbn-generator/internal/httpserver/server.go`

**Endpoints:**

1. **`POST /api/projects/:id/schedules`** — создать расписание
   ```json
   {
     "name": "Daily 5 sites",
     "strategy": "daily",
     "config": { "limit": 5, "time": "09:00" },
     "isActive": true
   }
   ```

2. **`GET /api/projects/:id/schedules`** — список расписаний проекта

3. **`GET /api/projects/:id/schedules/:schedule_id`** — получить расписание

4. **`PATCH /api/projects/:id/schedules/:schedule_id`** — обновить расписание

5. **`DELETE /api/projects/:id/schedules/:schedule_id`** — удалить расписание

6. **`POST /api/projects/:id/schedules/:schedule_id/trigger`** — запустить вручную (создает задачи для всех доменов проекта)

**Задачи:**
- [ ] Добавить handlers в `server.go`
- [ ] Валидация стратегий и конфигураций
- [ ] Проверка прав доступа (только owner/editor проекта)
- [ ] Добавить unit-тесты

### 1.5 Автоматическое создание задач в очереди

**Триггеры создания задач:**

1. **При создании домена:**
   - Проверяются активные расписания проекта
   - Для `immediate`: создается задача со `scheduled_for = NOW()`
   - Для `daily/weekly/custom`: вычисляется следующее доступное время с учетом лимита

2. **При активации расписания:**
   - Для всех доменов проекта без активных генераций создаются задачи

3. **При изменении расписания:**
   - Пересчитываются задачи для доменов проекта

**Файл:** `pbn-generator/internal/scheduler/scheduler.go` (новый модуль)

```go
type Scheduler struct {
    scheduleStore ScheduleStore
    queueStore    QueueStore
    domainStore   DomainStore
    generationStore GenerationStore
}

func (s *Scheduler) ScheduleDomain(ctx context.Context, domainID string, scheduleID string) error
func (s *Scheduler) CalculateNextRun(schedule Schedule, existingQueue []QueueItem) (time.Time, error)
func (s *Scheduler) ProcessNewDomain(ctx context.Context, domainID, projectID string) error
```

**Задачи:**
- [ ] Создать модуль `internal/scheduler/`
- [ ] Реализовать логику вычисления следующего запуска
- [ ] Интегрировать в создание доменов
- [ ] Добавить unit-тесты

### 1.6 UI для управления расписаниями

**Файл:** `frontend/app/projects/[id]/page.tsx`

**Компоненты:**
- Вкладка "Расписания" на странице проекта
- Форма создания/редактирования расписания
- Таблица расписаний с возможностью активации/деактивации
- Кнопка "Запустить сейчас" для каждого расписания
- Просмотр очереди генераций

**Задачи:**
- [ ] Добавить вкладку "Расписания"
- [ ] Форма создания расписания с выбором стратегии
- [ ] Таблица расписаний
- [ ] Интеграция с API

---

## 🔗 Компонент 2: Link Building

### 2.1 База данных

**Миграция:** `pbn-generator/internal/db/migrations/XXXX_add_link_building_tables.sql`

```sql
-- Задачи на простановку ссылок
CREATE TABLE IF NOT EXISTS link_tasks (
    id TEXT PRIMARY KEY,
    domain_id TEXT NOT NULL REFERENCES domains(id) ON DELETE CASCADE,
    anchor_text TEXT NOT NULL,
    target_url TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'searching', 'found', 'inserted', 'generated', 'failed')),
    found_location TEXT, -- JSON: { "file": "index.html", "position": 1234, "context": "..." }
    generated_content TEXT, -- сгенерированный абзац, если анкор не найден
    error TEXT,
    created_by TEXT NOT NULL REFERENCES users(email),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_link_tasks_domain ON link_tasks(domain_id);
CREATE INDEX idx_link_tasks_status ON link_tasks(status);
CREATE INDEX idx_link_tasks_created ON link_tasks(created_at);
```

### 2.2 Store слой

**Файл:** `pbn-generator/internal/store/sqlstore/link_task.go`

```go
type LinkTask struct {
    ID              string
    DomainID        string
    AnchorText      string
    TargetURL       string
    Status          string
    FoundLocation   sql.NullString // JSON
    GeneratedContent sql.NullString
    Error           sql.NullString
    CreatedBy       string
    CreatedAt       time.Time
    CompletedAt     sql.NullTime
    UpdatedAt       time.Time
}

type LinkTaskStore interface {
    Create(ctx context.Context, task LinkTask) error
    Get(ctx context.Context, id string) (LinkTask, error)
    ListByDomain(ctx context.Context, domainID string) ([]LinkTask, error)
    ListPending(ctx context.Context, limit int) ([]LinkTask, error)
    Update(ctx context.Context, task LinkTask) error
    Delete(ctx context.Context, id string) error
}
```

**Задачи:**
- [ ] Создать `link_task.go` с реализацией store
- [ ] Добавить unit-тесты

### 2.3 Link Building Worker

**Файл:** `pbn-generator/internal/linkbuilder/linkbuilder.go` (новый модуль)

**Процесс обработки:**

1. **Поиск анкора:**
   - Загружает HTML файлы из последней успешной генерации домена
   - Ищет `anchor_text` в тексте (case-insensitive, частичное совпадение)
   - Если найден: сохраняет позицию и контекст

2. **Вставка ссылки:**
   - Если анкор найден: оборачивает его в `<a href="target_url">anchor_text</a>`
   - Обновляет HTML файл
   - Статус: `inserted`

3. **Генерация контента:**
   - Если анкор не найден: запрашивает LLM
   - Промпт: "Дополни текст в стилистике сайта, включив фразу 'anchor_text' и ссылку на target_url"
   - Вставляет сгенерированный абзац после первого H2 или в конец статьи
   - Обновляет HTML файл
   - Статус: `generated`

4. **Обработка ошибок:**
   - При ошибке: статус `failed`, сохраняется текст ошибки

**Интеграция:**
- Новый тип задачи в Asynq: `TaskLinkBuilding`
- Worker обрабатывает задачи из очереди `link_building`

**Задачи:**
- [ ] Создать модуль `internal/linkbuilder/`
- [ ] Реализовать поиск анкора в HTML
- [ ] Реализовать вставку ссылки
- [ ] Реализовать генерацию контента через LLM
- [ ] Интегрировать в worker
- [ ] Добавить unit-тесты

### 2.4 API для управления ссылками

**Файл:** `pbn-generator/internal/httpserver/server.go`

**Endpoints:**

1. **`POST /api/domains/:id/links`** — создать задачу на простановку ссылки
   ```json
   {
     "anchor_text": "best casino",
     "target_url": "https://example.com"
   }
   ```

2. **`GET /api/domains/:id/links`** — список задач для домена

3. **`GET /api/links`** — все задачи (с фильтрами: `?status=pending&domain_id=...`)

4. **`GET /api/links/:id`** — получить задачу

5. **`DELETE /api/links/:id`** — удалить задачу

6. **`POST /api/links/:id/retry`** — повторить задачу

7. **`POST /api/domains/:id/links/batch`** — массовое добавление (CSV импорт)
   ```json
   {
     "links": [
       { "anchor_text": "casino", "target_url": "https://example.com" },
       { "anchor_text": "betting", "target_url": "https://example2.com" }
     ]
   }
   ```

**Задачи:**
- [ ] Добавить handlers в `server.go`
- [ ] Валидация URL и anchor_text
- [ ] Проверка прав доступа (только owner/editor проекта)
- [ ] Добавить unit-тесты

### 2.5 UI для управления ссылками

**Файл:** `frontend/app/domains/[id]/page.tsx` или отдельная страница

**Компоненты:**
- Таблица задач на странице домена
- Форма добавления ссылки
- Массовое добавление (CSV импорт или форма с множественными полями)
- Фильтры по статусу
- Кнопка "Повторить" для failed задач
- Просмотр деталей задачи (где найдена ссылка, сгенерированный контент)

**Задачи:**
- [ ] Добавить раздел "Link Building" на странице домена
- [ ] Форма добавления ссылки
- [ ] Таблица задач с фильтрами
- [ ] Массовое добавление
- [ ] Интеграция с API

---

## 📊 Общая структура задач

### Приоритет 1: Планировщик (4-5 недель)

**Неделя 1-2: База данных и Store**
- [ ] Миграция для `generation_schedules` и `generation_queue`
- [ ] Реализация `ScheduleStore` и `QueueStore`
- [ ] Unit-тесты для store

**Неделя 2-3: Scheduler Worker**
- [ ] Создание scheduler worker (отдельный процесс или интеграция)
- [ ] Логика обработки запланированных задач
- [ ] Интеграция с существующим worker для обновления статусов

**Неделя 3-4: API и автоматизация**
- [ ] API endpoints для управления расписаниями
- [ ] Модуль `internal/scheduler/` для автоматического создания задач
- [ ] Интеграция в создание доменов

**Неделя 4-5: UI**
- [ ] UI для управления расписаниями
- [ ] Просмотр очереди генераций
- [ ] Тестирование end-to-end

### Приоритет 2: Link Building (3-4 недели)

**Неделя 5-6: База данных и Store**
- [ ] Миграция для `link_tasks`
- [ ] Реализация `LinkTaskStore`
- [ ] Unit-тесты

**Неделя 6-7: Link Builder Worker**
- [ ] Модуль `internal/linkbuilder/`
- [ ] Поиск анкоров в HTML
- [ ] Вставка ссылок
- [ ] Генерация контента через LLM
- [ ] Интеграция в worker

**Неделя 7-8: API и UI**
- [ ] API endpoints для управления ссылками
- [ ] UI для управления ссылками
- [ ] Массовое добавление ссылок
- [ ] Тестирование end-to-end

---

## 🔄 Интеграция с существующей системой

### Изменения в существующих компонентах

1. **`cmd/worker/main.go`:**
   - Добавить обработку задач `TaskLinkBuilding`
   - При завершении генерации обновлять статус в `generation_queue`

2. **`internal/httpserver/server.go`:**
   - При создании домена вызывать `scheduler.ProcessNewDomain()`
   - Добавить endpoints для scheduler и link building

3. **`internal/tasks/tasks.go`:**
   - Добавить константу `TaskLinkBuilding = "link:build"`
   - Добавить `NewLinkBuildingTask()`

4. **`frontend/app/projects/[id]/page.tsx`:**
   - Добавить вкладку "Расписания"

5. **`frontend/app/domains/[id]/page.tsx`:**
   - Добавить раздел "Link Building"

---

## 🧪 Тестирование

### Unit-тесты
- [ ] Store слой для scheduler и link building
- [ ] Логика вычисления следующего запуска
- [ ] Поиск анкоров в HTML
- [ ] Генерация контента для ссылок

### Интеграционные тесты
- [ ] Полный цикл scheduler: создание расписания → создание задачи → обработка
- [ ] Полный цикл link building: создание задачи → поиск/генерация → вставка

### E2E тесты
- [ ] Создание расписания через UI → проверка автоматического запуска
- [ ] Добавление ссылки → проверка обработки

---

## 📝 Документация

- [ ] Обновить `ROADMAP.md` с деталями реализации
- [ ] Обновить `README.md` с описанием новых API
- [ ] Добавить примеры использования scheduler и link building
- [ ] Документация по конфигурации расписаний

---

## 🎯 Критерии готовности

### Планировщик готов, когда:
- ✅ Можно создать расписание для проекта
- ✅ При добавлении домена автоматически создаются задачи в очереди
- ✅ Scheduler worker обрабатывает задачи по расписанию
- ✅ Можно просмотреть очередь генераций
- ✅ UI позволяет управлять расписаниями

### Link Building готов, когда:
- ✅ Можно создать задачу на простановку ссылки
- ✅ Worker находит анкор в HTML и вставляет ссылку
- ✅ Если анкор не найден, LLM генерирует контент
- ✅ Можно массово добавлять ссылки
- ✅ UI показывает статус задач и результаты

---

## 🚀 Следующие шаги

1. **Начать с планировщика:**
   - Создать миграцию БД
   - Реализовать store слой
   - Создать базовый scheduler worker

2. **После планировщика:**
   - Перейти к link building
   - Использовать опыт из планировщика

3. **Параллельно:**
   - UI можно разрабатывать параллельно с backend
   - Тесты писать по мере реализации

---

## 📌 Примечания

- Scheduler worker можно интегрировать в существующий `cmd/worker/main.go` или сделать отдельным процессом
- Для cron выражений использовать библиотеку `github.com/robfig/cron/v3`
- Link building требует доступа к HTML файлам из генераций — использовать artifacts из БД
- При генерации контента для ссылок использовать промпт из БД (создать новый промпт `link_building`)

