# Roadmap — Unified Release v2.0-v3.0

## 🎯 Стратегия

**Текущий статус:** MVP (v1.0) завершен на 95%

**Новая стратегия:** Объединение запланированных версий v2.0 и v3.0 в единый релиз с фокусом на:
- Локальное хранение сайтов (вместо SFTP на данном этапе)
- Системы планирования (генерация + link building)
- Надежная проверка индексации
- Редактор сайтов

---

## 📅 Sprint Plan

### Sprint 1: Core Refactoring & Editor Backend

**Цель:** Переход на локальную стратегию хранения и основа для редактора

#### Ключевые задачи:

**1.1 Local Storage Strategy**
- Рефакторинг модуля публикации: замена SFTP на локальное хранилище
- Создание папки `server/{domain_name}/` для каждого домена
- Интерфейс `Publisher` с реализацией `LocalPublisher`
- Распаковка ZIP архивов в `server/{domain_name}/`
- Обновление статуса домена после локальной "публикации"

**1.2 File Storage & Management**
- Создание таблицы `site_files` для индексации файлов на диске
- Store методы: `SaveFile()`, `GetFile()`, `ListFiles()`, `DeleteFile()`
- Синхронизация файлов с базой данных
- API для работы с файлами домена

**1.3 Editor Backend**
- API эндпоинт `GET /api/domains/:id/files` - список файлов
- API эндпоинт `GET /api/domains/:id/files/:path` - получение содержимого
- API эндпоинт `PUT /api/domains/:id/files/:path` - сохранение изменений
- Валидация путей файлов (защита от Path Traversal)
- История изменений (таблица `file_edits`)

**Результат:** Локальное хранилище работает, API для редактора готов

---

### Sprint 2: Schedulers

**Цель:** Автоматизация генерации и link building через расписания

#### Ключевые задачи:

**2.1 Generation Scheduler**
- Таблицы: `generation_schedules`, `generation_queue`
- Миграции для новых таблиц
- API для управления расписаниями:
  - `POST /api/projects/:id/schedules` - создать расписание
  - `GET /api/projects/:id/schedules` - список расписаний
  - `PATCH /api/projects/:id/schedules/:id` - обновить
  - `DELETE /api/projects/:id/schedules/:id` - удалить
  - `POST /api/projects/:id/schedules/:id/trigger` - запуск вручную
- Worker для планировщика (Cron + Asynq)
- Стратегии: `immediate`, `daily`, `weekly`, `custom` (cron)

**2.2 Link Building Scheduler**
- Таблица `link_tasks` с полем `scheduled_for`
- API для создания задач с расписанием:
  - `POST /api/domains/:id/links` - создать задачу
  - `POST /api/domains/:id/links/import` - CSV импорт
  - `GET /api/domains/:id/links` - список задач
  - `PATCH /api/links/:id` - обновить (изменить время)
  - `DELETE /api/links/:id` - удалить
  - `POST /api/links/:id/retry` - повторить
- Worker для обработки link tasks:
  - Поиск анкора в HTML файлах `server/{domain}/`
  - Вставка ссылки `<a href="...">`
  - LLM генерация контента если анкор не найден
  - Сохранение изменений на диск

**2.3 Scheduler Worker**
- Создание `cmd/scheduler/main.go` или расширение существующего worker
- Регистрация Cron задач в Asynq
- Логика обработки `generation_queue`
- Логика обработки `link_tasks` по времени

**Результат:** Полная автоматизация генерации и link building по расписанию

---

### Sprint 3: Reliability System

**Цель:** Гарантированная проверка индексации доменов

#### Ключевые задачи:

**3.1 Index Checker Database**
- Таблица `domain_index_checks`:
  - `id`, `domain_id`, `check_date`, `status`, `is_indexed`
  - `attempts`, `last_attempt_at`, `error_message`
  - Статусы: `pending`, `checking`, `success`, `failed_investigation`
- Индексы для быстрого поиска по домену и дате
- Миграции

**3.2 Index Checker Logic**
- Модуль `internal/indexchecker/`:
  - Интерфейс `IndexChecker` 
  - `MockChecker` - всегда возвращает `false` (для разработки)
  - `RealChecker` - интеграция с реальным сервисом (заглушка для будущего)
- Умная логика повторов (Retry):
  - Первая попытка в 00:00 (настраивается)
  - Если ошибка → повтор через 30 мин (экспоненциальная задержка)
  - Максимум 8 попыток в сутки
  - Если за 24 часа не получен ответ → `failed_investigation`
  - Если успех → статус `success`, больше не проверяем в этот день

**3.3 Index Checker Worker**
- Создание `cmd/indexchecker/main.go` или расширение worker
- Cron задача: каждый час проверяет pending checks
- Логика обработки:
  - Берет все `pending` или `checking` с датой = сегодня
  - Вызывает `IndexChecker.Check(domain)`
  - Обрабатывает результат (success/error)
  - Планирует retry если нужно
  - Устанавливает `failed_investigation` через 24 часа

**3.4 API для Index Checks**
- `GET /api/domains/:id/index-checks` - история проверок
- `POST /api/domains/:id/index-checks` - запустить проверку вручную
- `GET /api/admin/index-checks` - все проверки (для мониторинга)
- `GET /api/admin/index-checks/failed` - проблемные домены

**Результат:** Надежная система проверки индексации с гарантией результата

---

### Sprint 4: UI Integration & Dashboards

**Цель:** Полная UI интеграция всех новых фич

#### Ключевые задачи:

**4.1 File Editor UI**
- Страница `/domains/[id]/editor`
- Дерево файлов (File Tree)
- Интеграция Monaco Editor:
  - Подсветка синтаксиса (HTML, CSS, JS)
  - Автосохранение (debounce)
  - Diff view для истории изменений
- Кнопки: Save, Revert, Download
- История изменений (sidebar)

**4.2 Scheduler UI**
- Вкладка "Schedules" на странице проекта
- Форма создания расписания:
  - Название
  - Стратегия (dropdown: immediate/daily/weekly/custom)
  - Конфигурация (input для limit/interval или cron)
- Таблица расписаний:
  - Название, стратегия, статус (active/inactive)
  - Кнопки: Edit, Delete, Trigger Now
- Просмотр очереди генерации (`/projects/[id]/queue`)

**4.3 Link Building UI**
- Вкладка "Links" на странице домена
- Форма добавления ссылки:
  - Anchor text
  - Target URL
  - Scheduled for (datetime picker)
- Таблица задач:
  - Анкор, URL, статус, время
  - Фильтры по статусу
  - Кнопки: Retry, Delete
- CSV импорт (drag & drop)
- Массовые действия (bulk retry, bulk delete)

**4.4 Index Monitoring Dashboard**
- Страница `/monitoring/indexing`
- Календарь с индикаторами проверок:
  - Зеленый = indexed
  - Красный = not indexed
  - Желтый = pending/checking
  - Серый = failed_investigation
- Таблица проверок:
  - Домен, дата, статус, попытки, результат
  - Фильтры: по домену, по статусу, по дате
- Графики:
  - Процент индексации по дням
  - Количество проверок в день
- Алерты для `failed_investigation`

**4.5 Server Folder Browser**
- Страница `/admin/server` (только admin)
- Список папок в `server/`
- Для каждого домена:
  - Название, размер на диске, количество файлов
  - Кнопка "Open Editor"
  - Кнопка "Download ZIP"
  - Кнопка "Delete" (с подтверждением)

**Результат:** Полнофункциональный UI для всех новых модулей

---

## 🎯 Критерии готовности (Definition of Done)

### Sprint 1
- [ ] Сайты сохраняются в `server/{domain_name}/` в распакованном виде
- [ ] API редактора отвечает на GET/PUT запросы
- [ ] История изменений записывается в БД

### Sprint 2
- [ ] Можно создать расписание генерации через UI
- [ ] Cron задача автоматически запускает генерацию
- [ ] Link tasks обрабатываются по времени
- [ ] Анкоры вставляются в HTML файлы на диске

### Sprint 3
- [ ] Проверка индексации запускается автоматически каждый день
- [ ] Retry логика работает при ошибках
- [ ] `failed_investigation` устанавливается через 24 часа
- [ ] История проверок доступна через API

### Sprint 4
- [ ] Monaco Editor работает в браузере
- [ ] Можно редактировать и сохранять файлы
- [ ] Dashboard показывает статус индексации
- [ ] Все CRUD операции работают через UI

---

## 📊 Зависимости

```
Sprint 1
  ├── Local Storage (база для всего)
  └── File API (требуется для Editor + Link Building)

Sprint 2
  ├── Зависит от Sprint 1 (нужен Local Storage)
  ├── Generation Scheduler
  └── Link Building (работает с файлами на диске)

Sprint 3
  ├── Независим от Sprint 2
  └── Index Checker (отдельная система)

Sprint 4
  ├── Зависит от Sprint 1, 2, 3
  └── UI для всех модулей
```

---

## 🚀 Будущие улучшения (Post-Release)

### Фаза 5: Production Deployment (после релиза)
- [ ] Замена `LocalPublisher` на `SFTPPublisher`
- [ ] Интеграция с реальным API проверки индексации
- [ ] Бэкапы файлов перед обновлением
- [ ] Rollback функциональность
- [ ] Версионирование публикаций

### Фаза 6: Advanced Features
- [ ] Multipage генерация
- [ ] AI-powered редактор (AI Edit)
- [ ] Автоматическая оптимизация контента
- [ ] A/B тестирование

---

## 📝 Примечания

**Архитектурные решения:**
1. **Local Storage First:** Позволяет быстро тестировать редактирование и link building. Переход на SFTP — простая замена реализации `Publisher`.
2. **Mock Index Checker:** Разрабатываем логику retry без зависимости от внешних сервисов. Интеграция с реальным API — один метод.
3. **Unified Scheduler:** Один worker для всех типов расписаний (генерация + links) через Asynq + Cron.

**Приоритеты:**
- **Sprint 1:** Критично — без Local Storage не работает редактор и link building
- **Sprint 2:** Высокий — автоматизация главная бизнес-ценность
- **Sprint 3:** Высокий — надежность проверки индексации — ключевое требование
- **Sprint 4:** Средний — UI можно доделывать постепенно

**Риски:**
- Перегрузка Sprint 2 (много логики) → может потребовать больше времени
- Сложность retry логики в Sprint 3 → выделить отдельный модуль
- Интеграция Monaco Editor → fallback: простой textarea
