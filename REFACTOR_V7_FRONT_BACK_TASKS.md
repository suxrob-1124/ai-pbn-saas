# Refactor V7: Frontend + Backend Stabilization Plan

Дата: 19.02.2026  
Статус: active backlog

## 1) Цель

Привести редактор и AI-контур к предсказуемому, поддерживаемому состоянию:

1. Разбить длинные страницы и серверные хендлеры на модули.
2. Ввести прозрачные состояния UI и строгие guard-правила действий.
3. Починить поток генерации картинок через AI.
4. Добавить понятную русскую локализацию и корректные подписи кнопок.
5. Исключить спам кнопок и гонки запросов.

---

## 2) Подтвержденные проблемы (по текущему состоянию)

1. `frontend/app/domains/[id]/editor/page.tsx` перегружен (2000+ строк), сложно сопровождать.
2. Состояния AI-запросов неочевидны: непонятно, что можно нажимать, а что уже в обработке.
3. Можно многократно дергать одни и те же AI-действия, создавая дубли и гонки.
4. Генерация страницы иногда возвращает JSON/служебный ответ вместо пользовательского результата.
5. Генерация ассетов (картинок) непрозрачна: нет отдельного UX-потока и причины ошибок неясны.
6. Локализация неполная, часть кнопок и сообщений смешана (ru/en), часть CTA не отражает действие.
7. Нет единого frontend-паттерна для AI state machine и async action guard.

---

## 3) Архитектурные решения (фиксируем перед реализацией)

1. Вводим feature-модуль `editor-v3` с разделением на:
- `components`
- `hooks`
- `context`
- `services`
- `types`

2. Для AI вводим явные state machine:
- `idle -> validating -> sending -> parsing -> ready -> applying -> done|error`.

3. Все destructive/дорогие действия делают:
- `single-flight` блокировку,
- `idempotency key`,
- кнопку `Повторить` только после terminal статуса.

4. Создание страницы и редактирование файла через AI:
- только strict contract,
- без silent fallback в файл.

5. Генерация картинок:
- отдельный action и отдельные настройки,
- отдельные статусы и ошибки,
- валидация ассета до применения.

---

## 4) Backlog (с приоритетом)

## P0 — Критичный рефактор структуры

1. Вынести из `frontend/app/domains/[id]/editor/page.tsx`:
- `EditorHeader`
- `EditorMainPane`
- `EditorPreviewPane`
- `EditorHistoryPane`
- `AIStudioPanel`
- `CreatePageWizard`
- `AssetGenerationPanel`

2. Вынести hooks:
- `useEditorState`
- `useFileActions`
- `useAISuggestFlow`
- `useAICreatePageFlow`
- `useAIAssetRegeneration`
- `useActionLocks`

3. Добавить context:
- `EditorContext` (selected file, dirty state, permissions, active operation),
- `AIContext` (current request, status, last result, source path, diagnostics visibility).

4. Разбить большой backend handler-контур editor в `server.go` на приватные модули:
- `editor_ai_handlers.go`
- `editor_file_handlers.go`
- `editor_context_pack.go`
- `editor_response_normalizer.go`

## P0 — UX-защита от ошибок пользователя

1. Блокировать кнопки при in-flight запросе с явным текстом причины.
2. Убрать возможность применить AI-результат не в исходный файл.
3. Добавить подтверждение для overwrite по каждому файлу.
4. Добавить глобальный баннер состояния:
- `Подготовка контекста`
- `Отправка в модель`
- `Парсинг ответа`
- `Готово`
- `Ошибка`

## P0 — Генерация картинок (починка контура)

1. Добавить отдельную кнопку `Сгенерировать изображение` в AI Studio.
2. Добавить форму параметров изображения:
- target path
- alt text
- style prompt
- модель image
- формат (webp/png)

3. Backend:
- сделать отдельный endpoint image-generation для редактора,
- возвращать структурированный результат (status, mime, size, diagnostics),
- валидировать сигнатуру файла и декодируемость.

4. На ошибке выводить:
- понятное сообщение,
- техническую причину в блоке диагностики,
- опции `Повторить` / `Загрузить вручную`.

## P1 — Локализация и названия действий

1. Ввести единый словарь UI-строк editor/ai на русском:
- кнопки
- статусы
- тултипы
- ошибки
- подтверждения

2. Переименовать кнопки по фактическому действию:
- `Suggest` -> `Сгенерировать предложение`
- `Apply to buffer` -> `Применить в редактор`
- `Generate files` -> `Сгенерировать файлы`
- `Apply all` -> `Применить выбранное`

3. Скрыть низкоуровневые технические детали по умолчанию в `Диагностика`.

## P1 — Прозрачность AI-контекста

1. Показывать карточку "Контекст запроса":
- какие файлы включены
- сколько символов/байт
- источник промпта (`domain|project|global|fallback`)

2. Добавить кнопку `Обновить контекст`.
3. Добавить индикатор stale-контекста после изменения файлов.

## P1 — Anti-spam и идемпотентность

1. Frontend single-flight per action key:
- `ai-suggest:<path>`
- `ai-create-page:<target>`
- `ai-generate-asset:<path>`

2. Backend idempotency key (header/body) для тяжелых AI-операций.
3. Серверный rate-limit на burst повторов одинаковых запросов.

## P2 — Backend cleanup и границы модулей

1. Перенести общую логику token/cost/logging для editor AI в отдельный сервис.
2. Выделить общий response sanitizer для всех AI-операций.
3. Добавить typed errors для UI:
- `invalid_format`
- `image_generation_failed`
- `context_too_large`
- `operation_locked`

---

## 5) Тесты (обязательные)

## Backend

1. Контрактные тесты editor AI endpoint-ов (strict format, no silent fallback).
2. Тесты image endpoint:
- валидный image
- битый image
- неправильный mime

3. Тесты idempotency/rate-limit.
4. Тесты typed errors -> корректные HTTP коды и payload.

## Frontend

1. Проверка lock-кнопок при in-flight.
2. Проверка source-path guard (нельзя применить не в тот файл).
3. Проверка локализации key UI-элементов.
4. Проверка create-page wizard + apply-plan.
5. Проверка image generation flow + error states.

---

## 6) Спринтовая разбивка

## Sprint R1 (2-3 дня)

1. Декомпозиция `editor/page.tsx` на компоненты и hooks.
2. Ввод `EditorContext` и `AIContext`.
3. Базовые action-locks и отключение спама кнопок.

## Sprint R2 (2-3 дня)

1. AI state machine + прозрачные статусы.
2. Строгие CTA и русская локализация editor/ai.
3. Source-path guard + overwrite confirmations.

## Sprint R3 (2-3 дня)

1. Полноценный поток генерации изображений.
2. Asset validation + structured diagnostics.
3. UX `Повторить/Загрузить вручную`.

### Статус выполнения R3 (обновлено)

1. `R3.1` завершен:
- введен typed frontend-контракт для image generation (`status/mime/size/warnings/error/token_usage`).
- убраны `any` в интеграции editor image flow.

2. `R3.2` завершен:
- добавлена отдельная секция `Генерация изображения` в AI Studio.
- отдельная кнопка запуска и понятный компактный результат генерации.

3. `R3.3` завершен:
- перед `Применить выбранное` добавлена валидация ассетов.
- проблемные ассеты блокируют массовое применение до явного решения.

4. `R3.4` завершен:
- ошибки image generation разделены на user-friendly сообщение и блок `Диагностика`.
- добавлены понятные бейджи статуса: `Успешно / Требует внимания / Ошибка`.
- для неуспешного результата добавлены действия `Повторить` и `Загрузить вручную`.

5. `R3.5` завершен:
- целевые проверки R3 проходят (`tsc` + verify-набор editor/ai/asset/apply/file-route).

## Sprint R4 (2 дня)

1. Backend cleanup модулей editor в `pbn-generator/internal/httpserver`.
2. Typed errors и унифицированный sanitizer.
3. Регрессионные тесты + smoke.

### Статус выполнения R4 (обновлено)

1. `R4.1` завершен:
- AI-хендлеры editor вынесены из `server.go` в `editor_ai_handlers.go`.

2. `R4.2` завершен:
- file-хендлеры editor вынесены из `server.go` в `editor_file_handlers.go`.
- роуты и контракты API сохранены.

3. `R4.3` завершен:
- добавлен `editor_errors.go` с типизированными кодами ошибок и единым `writeEditorError(...)`.
- editor endpoints возвращают консистентный `code/message/details` (additive, без ломки ключевых статус-кодов).

4. `R4.4` завершен:
- добавлен `editor_sanitizer.go` с общими sanitize/safety util-функциями.
- дубли path/AI-cleanup/payload validation в editor handlers убраны.

5. `R4.5` завершен:
- backend проверки пройдены:
  `go test ./internal/httpserver ./internal/store/sqlstore ./cmd/worker`
- frontend проверки пройдены:
  `npx tsc --noEmit`,
  `verify:file-editor-route`,
  `verify:ai-editor-panel`,
  `verify:ai-create-page-wizard`,
  `verify:ai-asset-resolution-actions`,
  `verify:ai-apply-plan-safety`.

---

## 7) Definition of Done

1. Структура editor-модуля декомпозирована:
- file/API/AI логика вынесена из монолитного `server.go` в отдельные файлы (`editor_ai_handlers.go`, `editor_file_handlers.go`, `editor_errors.go`, `editor_sanitizer.go`).
- frontend editor использует feature-модуль `frontend/features/editor-v3/*` для типов/hooks/сервисов, а не хранит всё в одной странице.

2. Защита от спама и гонок включена и проверяема:
- повторные клики по тяжелым AI-действиям во время in-flight не запускают дублирующие запросы (single-flight behavior).
- `Apply`-действия блокируются, пока генерация/валидация не завершена.

3. Статусы AI-операций прозрачны для пользователя:
- для suggest/create-page/regenerate-asset отображаются понятные состояния (`idle/validating/sending/parsing/ready/error` или эквивалентные UI-тексты).
- при ошибке пользователь видит человекочитаемое сообщение, техдетали доступны отдельно в `Диагностика`.

4. Контур генерации изображений отделен и стабилен:
- есть отдельная секция/кнопка генерации изображения и отдельные параметры (путь, промпт, модель, формат).
- результат генерации возвращается/обрабатывается через типизированный контракт (`status`, `mime`, `size`, `warnings`, `error_code/error_message`).
- проблемные ассеты не применяются “молча”: перед применением есть явная валидация и резолюция.

5. Локализация и CTA консистентны:
- ключевые кнопки editor/AI блока на русском и отражают фактическое действие (`Сгенерировать предложение`, `Применить в редактор`, `Сгенерировать файлы`, `Применить выбранное` и т.д.).
- в базовом UX нет критичного смешения ru/en для основных действий.

6. Ошибки editor endpoints унифицированы:
- editor scope возвращает консистентный payload ошибки: `code`, `message`, `details?` (additive без ломки статусов).
- фронт может опираться на `code` для сценариев обработки.

7. Обязательная верификация релизного состояния:
- backend:
  `go test ./internal/httpserver ./internal/store/sqlstore ./cmd/worker`
- frontend:
  `npx tsc --noEmit`
  `npm run -s verify:file-editor-route`
  `npm run -s verify:ai-editor-panel`
  `npm run -s verify:ai-create-page-wizard`
  `npm run -s verify:ai-asset-resolution-actions`
  `npm run -s verify:ai-apply-plan-safety`

---

## 8) Что убираем из активного списка как устаревшее

1. `done` — "добавить базовый editor route" и "включить только v1 file API".
Причина: реализовано и покрыто текущими маршрутами/проверками, повторно в backlog не ставим.

2. `cancelled` — задачи по `AI create-page` с `silent fallback` (запись сырого ответа в файл).
Причина: это небезопасный паттерн, заменен на strict-contract подход и явную ошибку формата.

3. `migrated` — ручной ввод модели в текстовое поле.
Причина: переведено в `select`-выбор модели; старые задачи закрываются как перенесенные в новый UX-контур.

4. `migrated` — разрозненные TODO по editor-фиксам из отдельных заметок.
Причина: источник правды один — этот документ; внешние дубли не поддерживаются как активный план.

---

## 9) Масштабирование рефакторинга на другие разделы (после editor)

Решение: делаем **editor-first**, затем переносим те же паттерны на остальные страницы.

Почему так:
1. Editor сейчас самый рискованный по UX и самый дорогой по ошибкам.
2. После стабилизации editor получаем готовые переиспользуемые паттерны:
- state machine для async-действий,
- action-locks,
- единый слой локализации,
- единый error/diagnostics формат,
- модульный подход к большим страницам.

### Wave 2 — Domain / Project pages
1. Разбить крупные страницы на feature-компоненты и hooks.
2. Убрать дубли async-логики кнопок (run/retry/relink/generate).
3. Привести статусы и CTA к единому словарю.
4. Добавить единые баннеры прогресса/ошибок.

### Wave 2 target map (аудит)

#### Кандидаты на вынос (по размеру/ответственности)

1. `frontend/app/projects/[id]/page.tsx` (`2211` строк)
- Проблема: смешаны настройки проекта, CRUD доменов, schedule/link-schedule, member management, project errors, queue actions.
- Вынос:
  - `frontend/features/domain-project/components/ProjectSettingsPanel.tsx`
  - `frontend/features/domain-project/components/ProjectDomainsTable.tsx`
  - `frontend/features/domain-project/components/ProjectMembersPanel.tsx`
  - `frontend/features/domain-project/components/ProjectSchedulesSection.tsx`
  - `frontend/features/domain-project/hooks/useProjectPageData.ts`
  - `frontend/features/domain-project/hooks/useProjectDomainActions.ts`
  - `frontend/features/domain-project/hooks/useProjectScheduleActions.ts`
  - `frontend/features/domain-project/services/projectStatusMeta.ts`

2. `frontend/app/domains/[id]/page.tsx` (`1664` строки)
- Проблема: в одном файле конфиг домена, generation actions, live preview, link tasks, generation details, prompt overrides.
- Вынос:
  - `frontend/features/domain-project/components/DomainHeaderActions.tsx`
  - `frontend/features/domain-project/components/DomainGenerationSection.tsx`
  - `frontend/features/domain-project/components/DomainLinkTasksSection.tsx`
  - `frontend/features/domain-project/components/DomainResultSection.tsx`
  - `frontend/features/domain-project/hooks/useDomainPageData.ts`
  - `frontend/features/domain-project/hooks/useDomainGenerationActions.ts`
  - `frontend/features/domain-project/hooks/useDomainLinkActions.ts`
  - `frontend/features/domain-project/services/domainStatusMeta.ts`

3. `frontend/app/projects/[id]/queue/page.tsx` (`1010` строк)
- Проблема: в одном файле active/history queue, link queue, фильтры, polling, cleanup/retry/delete actions.
- Вынос:
  - `frontend/features/queue-monitoring/components/ProjectQueueActiveTable.tsx`
  - `frontend/features/queue-monitoring/components/ProjectQueueHistoryTable.tsx`
  - `frontend/features/queue-monitoring/components/ProjectQueueLinkTasksTable.tsx`
  - `frontend/features/queue-monitoring/hooks/useProjectQueueData.ts`
  - `frontend/features/queue-monitoring/hooks/useProjectQueueActions.ts`
  - `frontend/features/queue-monitoring/services/queueFilters.ts`

#### Дублирующаяся async-логика (что унифицируем в Wave 2)

1. Повторяющиеся fetch/load методы с одинаковой схемой `loading + toast + reload`:
- `load*`, `run*`, `remove*`, `triggerGeneration`, `deleteGeneration`, `pause/resume/cancel` в `domains/[id]/page.tsx`.
- `load*`, `addDomain/importDomains`, `runGeneration`, `runLinkTask/removeLinkTask`, schedule handlers в `projects/[id]/page.tsx`.
- `load/loadHistory/loadLinkTasks`, `handleLinkRetry/Delete`, `handleCleanup/Refresh` в `projects/[id]/queue/page.tsx`.

2. Повторяющиеся guards и маппинги статусов:
- link status normalization/meta частично централизованы, но дублируются в page-level условной логике.
- availability rules для кнопок (retry/delete/run/cancel) повторяются в разных страницах.

3. Повторяющиеся query/filter state-машины:
- `status/date/search/page` фильтры и их синхронизация разнесены по страницам и локальным хукам.

#### Общий словарь статусов/CTA (база для W2.2)

1. Статусы:
- generation: `waiting|queued|processing|done|error|paused|cancelled`
- link: `pending|searching|removing|inserted|generated|removed|failed`
- queue/history: `pending|queued|completed|failed`

2. CTA:
- генерация: `Запустить`, `Перезапустить с шага`, `Пауза`, `Возобновить`, `Отменить`, `Удалить`
- ссылки: `Добавить ссылку`, `Удалить ссылку`, `Повторить`, `Удалить задачу`
- очередь: `Обновить`, `Очистить очередь`, `Открыть`, `Удалить`

#### Порядок выполнения W2.2–W2.5

1. `W2.2` — вынести shared словарь статусов/CTA и action guards для `projects/[id]` + `domains/[id]`.
2. `W2.3` — вынести async handlers в hooks и подключить single-flight lock для тяжелых действий.
3. `W2.4` — добавить flow-state индикацию и единые баннеры прогресса/ошибок.
4. `W2.5` — полный regression pass (`tsc` + verify) и фиксация Wave 2 статуса в этом документе.

### Wave 2 status (W2.5, 24.02.2026)

1. `completed` — `W2.2`:
- status/CTA маппинги для domain/project централизованы в общем модуле.
- локальные дубли словарей и label/helper логики удалены со страниц.

2. `completed` — `W2.3`:
- async handlers run/retry/relink/generate вынесены в hooks.
- для тяжелых действий подключен single-flight guard (повторный клик in-flight не создает дублирующие запросы).

3. `completed` — `W2.4`:
- добавлены flow-state индикаторы и баннеры прогресса/ошибок для ключевых операций на `/domains/[id]` и `/projects/[id]`.
- ошибки отображаются в user-friendly слое, диагностические детали остаются вторичным слоем.

4. `completed` — `W2.5` regression:
- `npx tsc --noEmit` — green.
- verify (domain/project/editor маршруты) — green:
`verify:domain-result-block`, `verify:domain-editor-button`, `verify:project-queue`,
`verify:project-queue-active-filters`, `verify:project-queue-history`,
`verify:project-queue-link-normalization`, `verify:file-editor-route`,
`verify:ai-editor-panel`, `verify:ai-create-page-wizard`,
`verify:ai-asset-resolution-actions`, `verify:ai-apply-plan-safety`.

### Wave 2 leftovers / residual risks

1. Страницы `frontend/app/projects/[id]/page.tsx` и `frontend/app/domains/[id]/page.tsx` все еще большие по объему и требуют дальнейшей компонентной декомпозиции (вне минимального объема W2.2–W2.5).
2. Выравнивание async/guard паттернов для `frontend/app/projects/[id]/queue/page.tsx` и schedule/monitoring контуров остается в следующей волне (`Wave 3`).
3. Покрытие verify-скриптами есть, но отсутствует единый e2e-сценарий со смешанными in-flight действиями между несколькими вкладками/ролями пользователя.

### Wave 3 — Queue / Schedule / Monitoring
1. Унифицировать таблицы, фильтры, пагинацию и loading/error states.
2. Ввести общий паттерн disable/guard для операций очереди.
3. Убрать расхождения в статусах между страницами.
4. Привести тексты и термины к одной русской локализации.

### Wave 3 status (W3.1–W3.4, 25.02.2026)

1. `completed` — `W3.1` shared queue/filter/pagination primitives:
- общие компоненты `FilterSelect`, `FilterDateInput`, `PaginationControls` используются в queue/schedule/monitoring контурах.
- примитивы и query helper слой подключены в `/queue`, `/projects/[id]/queue`, `/monitoring/indexing`.

2. `completed` — `W3.2` action guard parity:
- disable/guard причины унифицированы через общий `actionGuards` слой.
- правила `run/retry/delete/cancel` выровнены между queue/schedule экранами без изменений backend-контрактов.

3. `completed` — `W3.3` status normalization + RU parity:
- добавлен общий слой `frontend/features/queue-monitoring/services/statusMeta.ts`.
- локальные status mapping-дубли убраны из queue/schedule/monitoring UI.
- основные статусы/бейджи/фильтры приведены к единому русскому словарю и нормализации legacy-статусов.

4. `completed` — `W3.4` regression + docs sync:
- без фич, только стабилизация и синхронизация плана.
- подтверждено green:
  - `npx tsc --noEmit`
  - `verify:project-queue`
  - `verify:project-queue-active-filters`
  - `verify:project-queue-history`
  - `verify:project-queue-link-normalization`
  - `verify:schedule-ui`
  - `verify:schedule-list`
  - `verify:schedule-form`
  - `verify:index-monitoring-ui`
  - `verify:index-monitoring-dashboard`
  - `verify:index-table`
  - `verify:index-checks-pagination`
  - `verify:index-stats`
  - `verify:failed-checks-alert`
  - `verify:link-status-consistency`

### Wave 3 leftovers / residual risks

1. В queue/schedule/monitoring остаются крупные page-level компоненты (`/queue`, `/projects/[id]/queue`, `/monitoring/indexing`) — нужна дальнейшая декомпозиция (вне Wave 3).
2. Нет единого e2e-сценария для конкурентных in-flight действий (несколько вкладок/ролей/параллельные операции).
3. Для редких неизвестных/кастомных статусов сохранен fallback-показ raw-значения; при расширении backend-статусов потребуется обновление словаря `statusMeta`.

### Wave 4 — Backend HTTP слой целиком
1. Распилить монолитные handler-файлы по доменным модулям.
2. Вынести общие parse/validate/respond утилиты.
3. Стандартизировать typed errors и API payload ошибок.
4. Зафиксировать единый контракт логирования и диагностики.

### Definition of Done для этапа "весь проект"
1. Нет страниц-«монолитов» на 1500+ строк без модульного деления.
2. Все долгие действия имеют прозрачный жизненный цикл в UI.
3. Нет кнопок, которые можно безлимитно спамить в in-flight.
4. Локализация и названия действий консистентны во всех ключевых разделах.
