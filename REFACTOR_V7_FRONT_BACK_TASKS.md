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

### Wave 4 status (W4.1–W4.3, 25.02.2026)

1. `completed` — `W4.1` handler modular split:
- доменные хендлеры вынесены из `server.go` в отдельные файлы (`project_*`, `index_checks`, `editor_*` контуры).
- в `server.go` оставлены маршрутизация и минимальный dispatch.

2. `completed` — `W4.2` parse/validate/respond + typed errors:
- общие parse/query/body helpers централизованы в `http_utils.go`.
- unified error envelope для non-editor и typed editor errors подключены в новых модулях без изменений API-роутов.

3. `completed` — `W4.3` logging/diagnostics contract + regression:
- для `httpserver` зафиксирован единый request/error log-контракт (`request_id`, `error_code`, `error_message`, `error_kind`, `error_details`).
- policy уровней: `4xx => warn`, `5xx => error`.
- для non-editor ответов диагностические `details` не отдаются в клиентский payload, но сохраняются в логах; editor envelope оставлен совместимым (`details?`).
- regression подтвержден:
  - `go test ./internal/httpserver ./internal/store/sqlstore ./cmd/worker`
  - `npx tsc --noEmit`
  - `verify:file-editor-route`
  - `verify:ai-editor-panel`
  - `verify:ai-create-page-wizard`
  - `verify:ai-asset-resolution-actions`
  - `verify:ai-apply-plan-safety`

### Wave 4 leftovers / residual risks

1. `server.go` заметно уменьшен, но остаются legacy handler-блоки, требующие дальнейшего распила в следующей волне.
2. Нет сквозной e2e-корреляции `request_id` между `httpserver` и `worker` логами (корреляция частично ручная).
3. Сохранены разные текстовые формулировки user-facing ошибок в части legacy endpoint-ов; формат payload уже унифицирован, но тексты требуют отдельной нормализации.

### Wave 5 — LLM usage accounting
1. Подготовить data-model и store слой для usage/pricing.
2. Нормализовать usage metadata и fallback policy в LLM клиенте.
3. Инструментировать generation/editor/link операции usage-событиями.
4. Добавить admin/project usage API и frontend usage-экраны.
5. Закрыть wave regression и синхронизировать план.

### Wave 5 status (W5.1–W5.7, 25.02.2026)

1. `completed` — `W5.1` data model + migrations:
- добавлены схемы `llm_usage_events`, `llm_model_pricing`, индексы под фильтры отчетов.
- добавлен seed активных тарифов gemini.
- добавлен store-слой `LLMUsageStore`/`ModelPricingStore`.

2. `completed` — `W5.2` usage normalization:
- usage metadata нормализованы (`prompt/completion/total tokens`, `token_source`).
- включен fallback policy оценки токенов/стоимости.
- unit-тесты `internal/llm` для parsing/estimation/mixed-case проходят.

3. `completed` — `W5.3` instrumentation:
- generation pipeline, editor AI и link worker логируют usage events.
- для error-case создаются события со статусом `error` и доступной metadata.
- snapshot цен фиксируется на момент запроса.

4. `completed` — `W5.4` admin/project usage API:
- добавлены/стабилизированы:
  - `GET /api/admin/llm-usage/events`
  - `GET /api/admin/llm-usage/stats`
  - `GET /api/admin/llm-pricing`
  - `PUT /api/admin/llm-pricing/:model`
  - `GET /api/projects/:id/llm-usage/events`
  - `GET /api/projects/:id/llm-usage/stats`
- добавлены фильтры/пагинация/group_by.
- добавлено тест-покрытие auth + scope + filters + pagination.

5. `completed` — `W5.5` frontend usage pages:
- страницы:
  - `/monitoring/llm-usage` (admin)
  - `/projects/[id]/usage` (project scope)
- отображаются KPI, таблица событий, фильтры, пагинация.
- добавлены `estimated` badge и `n/a` tooltip для отсутствующей стоимости.
- интегрированы переходы в monitoring/project navigation.

6. `completed` — `W5.7` regression + release readiness:
- backend green:
  - `go test ./internal/llm ./internal/store/sqlstore ./internal/httpserver ./cmd/worker`
- frontend green:
  - `npx tsc --noEmit`
  - `verify:llm-usage-admin-page`
  - `verify:llm-usage-project-page`
  - `verify:llm-usage-filters-pagination`
  - `verify:llm-usage-cost-badges`
  - `verify:file-editor-route`
  - `verify:ai-editor-panel`
  - `verify:ai-create-page-wizard`
  - `verify:ai-asset-resolution-actions`
  - `verify:ai-apply-plan-safety`
- дополнительно подтвержден project/admin scope:
  - `go test ./internal/httpserver -run 'Test(AdminLLMUsageEventsAuthFiltersAndPagination|ProjectLLMUsageEventsForOwnerAndManager|ProjectLLMUsageEventsForbiddenForNonMember|ProjectLLMUsageStatsGroupBy)'`

### Wave 5 leftovers / residual risks

1. Для generation/editor/link usage нет отдельного e2e smoke c реальной БД и последовательными ручными операциями в одном сценарии; текущая уверенность основана на unit/integration regression и наличии instrumentation в коде.
2. Проверка корректности cost при смене цен в коротком временном окне требует отдельного integration теста с контролируемым `active_from/active_to`.
3. Часть verify-скриптов usage чувствительна к рефакторингу структуры UI-файлов (string-based asserts); при следующей декомпозиции стоит перевести их на более устойчивые проверки.

### Wave 6 — Frontend decomposition follow-up

Цель:
1. Продолжить декомпозицию оставшихся крупных page-level файлов batch-подходом без изменения API/UX-контрактов.
2. Синхронизировать фактические метрики размера страниц и verify-доказательства.

Ограничения:
1. Без backend-изменений.
2. Без редизайна.
3. Только безопасный вынос state/handlers/section-логики в feature hooks/components/services.

### Wave 6 status (W6.1–W6.3, 26.02.2026)

1. `completed` — `W6.1` editor decomposition continuation:
- добавлены feature hooks/services/components в `frontend/features/editor-v3/*`.
- runtime и API поведение сохранены.
- текущий размер `frontend/app/domains/[id]/editor/page.tsx`: `1245` строк (было `1491` в предыдущем срезе).

2. `completed` — `W6.2` project page orchestration cleanup:
- вынесен schedule/link-schedule state+handlers в `frontend/features/domain-project/hooks/useProjectSchedules.ts`.
- `frontend/app/projects/[id]/page.tsx` сокращен `1148 -> 807`.
- verify green:
  - `npx tsc --noEmit`
  - `npm run -s verify:schedule-ui`
  - `npm run -s verify:schedule-form`
  - `npm run -s verify:schedule-list`
  - `npm run -s verify:nav-tabs`

3. `completed` — `W6.3` queue/indexing decomposition:
- `frontend/app/projects/[id]/queue/page.tsx`:
  - вынесен data/action слой в `frontend/features/queue-monitoring/hooks/useProjectQueueData.ts`.
  - размер страницы сокращен `1017 -> 620`.
  - verify green:
    - `npm run -s verify:project-queue`
    - `npm run -s verify:project-queue-active-filters`
    - `npm run -s verify:project-queue-history`
    - `npm run -s verify:project-queue-link-normalization`
- `frontend/app/monitoring/indexing/page.tsx`:
  - вынесены scope/history hooks и общие parsing/date/sort utils:
    - `frontend/features/queue-monitoring/hooks/useIndexMonitoringScopeLabels.ts`
    - `frontend/features/queue-monitoring/hooks/useIndexCheckHistory.ts`
    - `frontend/features/queue-monitoring/services/indexingPageUtils.ts`
  - размер страницы сокращен `957 -> 784`.
  - verify green:
  - `npm run -s verify:index-monitoring-ui`
  - `npm run -s verify:index-monitoring-dashboard`
  - `npm run -s verify:index-stats`
  - `npm run -s verify:index-table`
  - `npm run -s verify:index-checks-pagination`
  - `npx tsc --noEmit`

### Wave 6 leftovers / residual risks

1. `frontend/app/domains/[id]/editor/page.tsx` остается выше целевого `1200` (текущий размер `1245`), нужен дополнительный вынос asset/image handlers.
2. `frontend/app/monitoring/indexing/page.tsx` и `frontend/app/queue/page.tsx` все еще содержат заметные page-level блоки логики; требуется следующий safe-split в batch B.
3. Часть verify-проверок по-прежнему string-based и чувствительна к перемещению JSX/handler-кода между файлами.

### DOD.1A File-Scoped Split Map (26.02.2026)

Аудит выполнен без изменений runtime-логики. Line-count ниже зафиксирован по фактическому состоянию репозитория на дату аудита.

Контекст для переиспользования:
- `frontend/features/editor-v3/*`
- `frontend/components/LinkTaskList.tsx`
- `frontend/components/ScheduleList.tsx`
- `frontend/components/PromptOverridesPanel.tsx`
- `frontend/components/indexing/*`

| file | lines | target_modules | priority | risk | batch |
|---|---:|---|---|---|---|
| `frontend/app/domains/[id]/editor/page.tsx` | 2811 | `features/editor-v3/{components,hooks,services,types}` + `app/domains/[id]/editor/page.tsx` как orchestrator | P0 | high | A |
| `frontend/app/projects/[id]/page.tsx` | 1737 | `features/domain-project/{components,hooks,services,types}` | P0 | high | A |
| `frontend/app/domains/[id]/page.tsx` | 1257 | `features/domain-project/{components,hooks,services,types}` | P1 | medium | A |
| `frontend/app/projects/[id]/queue/page.tsx` | 906 | `features/queue-monitoring/{components,hooks,services,types}` | P1 | medium | B |
| `frontend/app/monitoring/indexing/page.tsx` | 871 | `components/indexing/*` + `features/queue-monitoring/{hooks,services}` | P1 | medium | B |
| `frontend/app/admin/page.tsx` | 867 | `features/admin/{components,hooks,services,types}` | P2 | medium | C |
| `frontend/app/queue/page.tsx` | 624 | `features/queue-monitoring/{components,hooks,services}` | P2 | low | B |

#### 1) `frontend/app/domains/[id]/editor/page.tsx` (2811)
- Текущая ответственность (смешано):
  - layout/editor shell, file-tree, live preview, ai-studio, diff/apply, history/revert, permissions, async guards.
  - локальные нормализации path/context/model/error + оркестрация нескольких API-контуров.
- Целевая декомпозиция:
  - components: `EditorShell`, `EditorToolbar`, `EditorFilePane`, `EditorPreviewPane`, `EditorAIPanel`, `EditorDiffPanel`, `EditorHistoryPanel`.
  - hooks: `useEditorState`, `useEditorFileActions`, `useEditorAIActions`, `useEditorPreview`, `useEditorHistory`.
  - services: `editorApi`, `editorPathPolicy`, `editorAiContract`, `editorErrorMapper`.
  - types/context: `editor.types.ts`, `EditorPageContext`.
- Зависимости:
  - API: `/api/domains/:id/files*`, `/api/domains/:id/editor/*`.
  - reuse: `frontend/features/editor-v3/*`.
  - shared: `single-flight`/status helpers из текущего frontend.
- Риск/сложность: `high` (много in-flight сценариев и синхронизации editor/preview/AI).
- Целевой line-budget после split:
  - page-orchestrator: `<= 700`
  - каждый feature-module: `<= 350`
  - итог на feature-папку вместо монолита: `~1300-1600`.

#### 2) `frontend/app/projects/[id]/page.tsx` (1737)
- Текущая ответственность (смешано):
  - project summary/settings, domains CRUD/import, members, schedules, link-actions, статусы/CTA, guards.
- Целевая декомпозиция:
  - components: `ProjectHeaderActionsSection`, `ProjectSettingsSection`, `ProjectDomainsSection`, `ProjectMembersSection`, `ProjectSchedulesSection`.
  - hooks: `useProjectPageQuery`, `useProjectMutations`, `useProjectAsyncActions`.
  - services/types: `projectStatusDictionary`, `projectPermissions`, `project.types.ts`.
- Зависимости:
  - API: `/api/projects/:id*`, `/api/projects/:id/domains*`, `/api/projects/:id/members*`, `/api/projects/:id/schedules*`.
  - reuse: `ScheduleList`, `PromptOverridesPanel`.
- Риск/сложность: `high`.
- Целевой line-budget после split: page `<= 550`, feature-модули `<= 250-300` каждый.

#### 3) `frontend/app/domains/[id]/page.tsx` (1257)
- Текущая ответственность (смешано):
  - domain summary/settings, generation CTA/status, artifacts/result, links/index checks shortcuts, prompt overrides.
- Целевая декомпозиция:
  - components: `DomainHeaderActionsSection`, `DomainGenerationStatusSection`, `DomainResultSection`, `DomainLinksSection`.
  - hooks: `useDomainPageQuery`, `useDomainAsyncActions`.
  - services/types: `domainStatusDictionary`, `domain.types.ts`.
- Зависимости:
  - API: `/api/domains/:id*`, `/api/domains/:id/summary`, `/api/domains/:id/generate`, `/api/domains/:id/links`.
  - reuse: `LinkTaskList`, `PromptOverridesPanel`.
- Риск/сложность: `medium`.
- Целевой line-budget после split: page `<= 500`, секции `<= 220-280`.

#### 4) `frontend/app/projects/[id]/queue/page.tsx` (906)
- Текущая ответственность (смешано):
  - active/history queue, filters/query-string, bulk actions, link-task view, polling/busy-state.
- Целевая декомпозиция:
  - components: `ProjectQueueHeader`, `ProjectQueueFilters`, `ProjectQueueActiveTable`, `ProjectQueueHistoryTable`.
  - hooks: `useProjectQueueFilters`, `useProjectQueueData`, `useProjectQueueActions`.
  - services/types: `queueQueryState`, `queueStatusMeta`, `queue.types.ts`.
- Зависимости:
  - API: `/api/projects/:id/queue*`.
  - reuse: queue primitives из `features/queue-monitoring`.
- Риск/сложность: `medium`.
- Целевой line-budget после split: page `<= 380`, каждый модуль `<= 220`.

#### 5) `frontend/app/monitoring/indexing/page.tsx` (871)
- Текущая ответственность (смешано):
  - dashboard KPI, таблица проверок, календарь, фильтры, stats-запросы, retry/run actions.
- Целевая декомпозиция:
  - components: `IndexingFiltersBar`, `IndexingKPISection`, `IndexingChecksTable`, `IndexingCalendarSection`, `IndexingFailedAlert`.
  - hooks: `useIndexingFilters`, `useIndexingDashboardData`, `useIndexingActions`.
  - services/types: `indexingQueryAdapter`, `indexingStatusMeta`, `indexing.types.ts`.
- Зависимости:
  - API: `/api/*/index-checks*` (project/domain/admin scope).
  - reuse: `frontend/components/indexing/*`.
- Риск/сложность: `medium`.
- Целевой line-budget после split: page `<= 360`, feature-компоненты `<= 200-240`.

#### 6) `frontend/app/admin/page.tsx` (867)
- Текущая ответственность (смешано):
  - users table/actions, prompts/audit правила, фильтры, локальная нормализация статусов.
- Целевая декомпозиция:
  - components: `AdminUsersSection`, `AdminPromptsSection`, `AdminAuditRulesSection`.
  - hooks: `useAdminUsers`, `useAdminPrompts`, `useAdminAuditRules`.
  - services/types: `adminApi`, `adminStatusLabels`, `admin.types.ts`.
- Зависимости:
  - API: `/api/admin/users*`, `/api/admin/prompts*`, `/api/admin/audit-rules*`.
- Риск/сложность: `medium`.
- Целевой line-budget после split: page `<= 350`, секции `<= 220-260`.

#### 7) `frontend/app/queue/page.tsx` (624)
- Текущая ответственность (смешано):
  - global queue/history view, фильтры, pagination, item-actions, status badges.
- Целевая декомпозиция:
  - components: `GlobalQueueFilters`, `GlobalQueueTable`, `GlobalQueueActionsBar`.
  - hooks: `useGlobalQueueFilters`, `useGlobalQueueData`.
  - services/types: `queueStatusMeta`, `queueQueryState`.
- Зависимости:
  - API: `/api/generations`, `/api/queue/:id`.
  - reuse: queue primitives из `features/queue-monitoring`.
- Риск/сложность: `low`.
- Целевой line-budget после split: page `<= 280`, модули `<= 180-220`.

### DOD.1A Batch Order (готово к DOD.1B)

#### Batch A — projects/domain core
- Owner: `frontend-domain-core`.
- Scope: `projects/[id]`, `domains/[id]`, `domains/[id]/editor`.
- Verify commands:
  - `npx tsc --noEmit`
  - `npm run -s verify:domain-result-block`
  - `npm run -s verify:domain-editor-button`
  - `npm run -s verify:file-editor-route`
  - `npm run -s verify:file-editor-permissions`
  - `npm run -s verify:ai-editor-panel`
  - `npm run -s verify:ai-create-page-wizard`
  - `npm run -s verify:ai-asset-resolution-actions`
  - `npm run -s verify:ai-apply-plan-safety`

#### Batch B — queue/schedule/monitoring
- Owner: `frontend-queue-monitoring`.
- Scope: `projects/[id]/queue`, `queue`, `monitoring/indexing`.
- Verify commands:
  - `npx tsc --noEmit`
  - `npm run -s verify:project-queue`
  - `npm run -s verify:project-queue-active-filters`
  - `npm run -s verify:project-queue-history`
  - `npm run -s verify:project-queue-link-normalization`
  - `npm run -s verify:index-monitoring-ui`
  - `npm run -s verify:index-monitoring-dashboard`
  - `npm run -s verify:index-stats`
  - `npm run -s verify:index-table`
  - `npm run -s verify:index-checks-pagination`
  - `npm run -s verify:schedule-ui`
  - `npm run -s verify:schedule-list`

#### Batch C — admin/docs leftovers
- Owner: `frontend-admin-docs`.
- Scope: `admin/page` + доработка docs-навигации и consistency-словарей.
- Verify commands:
  - `npx tsc --noEmit`
  - `npm run -s verify:llm-usage-admin-page`
  - `npm run -s verify:nav-tabs`
  - `npm run -s verify:docs-links`
  - `npm run -s verify:docs-quality`

### Gate к DOD.1B
1. Все файлы `>1500` имеют утвержденный split-map:
   - `frontend/app/domains/[id]/editor/page.tsx`
   - `frontend/app/projects/[id]/page.tsx`
2. Для каждого batch зафиксированы owner и verify-команды.
3. В DOD.1B допускаются только batch-driven изменения по этой карте.

### Definition of Done для этапа "весь проект"
1. Нет страниц-«монолитов» на 1500+ строк без модульного деления.
2. Все долгие действия имеют прозрачный жизненный цикл в UI.
3. Нет кнопок, которые можно безлимитно спамить в in-flight.
4. Локализация и названия действий консистентны во всех ключевых разделах.

### DOD.FINAL gate (26.02.2026)

#### Regression evidence (green)

Backend:
1. `go test ./internal/httpserver ./internal/store/sqlstore ./cmd/worker` — green.

Frontend:
1. `npx tsc --noEmit` — green.
2. `verify:file-editor-route` — green.
3. `verify:ai-editor-panel` — green.
4. `verify:ai-create-page-wizard` — green.
5. `verify:ai-asset-resolution-actions` — green.
6. `verify:ai-apply-plan-safety` — green.
7. `verify:project-queue` — green.
8. `verify:project-queue-active-filters` — green.
9. `verify:project-queue-history` — green.
10. `verify:project-queue-link-normalization` — green.
11. `verify:index-monitoring-ui` — green.
12. `verify:index-monitoring-dashboard` — green.
13. `verify:index-stats` — green.
14. `verify:index-table` — green.
15. `verify:index-checks-pagination` — green.
16. `verify:schedule-ui` — green.
17. `verify:schedule-list` — green.
18. `verify_global_queue_domain.ts` — green.

#### Short evidence

Файлы, которые были `>1500` и текущий статус после split:
1. `frontend/app/domains/[id]/editor/page.tsx`: `2811 -> 1245` (done для порога >1500, декомпозиция продолжена на `features/editor-v3/components/*` и `hooks/*`).
2. `frontend/app/projects/[id]/page.tsx`: `2211 -> 807` (done для порога >1500).
3. `frontend/app/domains/[id]/page.tsx`: `1664 -> 445` (done для порога >1500).

Где включен flow-state:
1. `frontend/app/domains/[id]/page.tsx` (generation/link flow banners).
2. `frontend/app/projects/[id]/page.tsx` (domain/link operation flow banners).
3. `frontend/app/queue/page.tsx` (refresh + link actions flow banners).
4. `frontend/app/projects/[id]/queue/page.tsx` (queue + links flow banners).
5. `frontend/app/monitoring/indexing/page.tsx` (monitoring refresh + manual run flow banners).

Где включен single-flight:
1. `frontend/features/domain-project/hooks/useDomainActions.ts`.
2. `frontend/features/domain-project/hooks/useProjectActions.ts`.
3. `frontend/app/queue/page.tsx`.
4. `frontend/app/projects/[id]/queue/page.tsx`.
5. `frontend/app/monitoring/indexing/page.tsx`.

Где финализована RU-локализация (core scopes):
1. `frontend/features/editor-v3/services/i18n-ru.ts`.
2. `frontend/features/queue-monitoring/services/i18n-ru.ts`.
3. `frontend/features/queue-monitoring/services/statusMeta.ts`.
4. `frontend/features/domain-project/services/statusCta.ts`.

#### DoD status (1-4)

1. DoD-1 (`монолиты >1500`) — `done`.
Подтверждение: все исходные страницы с `>1500` строк приведены ниже порога (`editor/page.tsx` теперь `1245`).

2. DoD-2 (`прозрачный lifecycle долгих действий`) — `done` для ключевых продуктовых контуров.
Подтверждение: flow-state баннеры и статусы присутствуют в editor/domain/project/queue/monitoring маршрутах.

3. DoD-3 (`anti-spam in-flight`) — `partial`.
Подтверждение: single-flight включен на ключевых async-действиях; остаток — нет единого e2e multi-tab/multi-role сценария конкурентных кликов.

4. DoD-4 (`консистентная RU-терминология`) — `partial`.
Подтверждение: унифицированы core-слои queue/monitoring/editor/domain-project; остаток — legacy admin/docs тексты и отдельные fallback-сообщения.

#### Residuals (owner + due-date)

1. Остаток: дополнительная декомпозиция `frontend/app/domains/[id]/editor/page.tsx` до `<1200` (не DoD-блокер).
- Owner: `frontend-editor-core`.
- Due date: `2026-03-18`.

2. Остаток: e2e сценарий конкурентных in-flight действий (multi-tab/multi-role) для подтверждения DoD-3.
- Owner: `qa-frontend`.
- Due date: `2026-03-10`.

3. Остаток: финальный RU-pass по admin/docs и legacy fallback-текстам для полного закрытия DoD-4.
- Owner: `frontend-admin-docs`.
- Due date: `2026-03-14`.
