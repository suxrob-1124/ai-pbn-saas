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

1. `editor/page.tsx` не содержит "монолитную лапшу", логика разнесена по модулям.
2. Невозможно многократно спамить тяжелые AI-действия.
3. Пользователь всегда видит понятный статус выполнения AI-операции.
4. Генерация изображений работает отдельным и прозрачным контуром.
5. Кнопки и поля локализованы и названы по фактическому действию.
6. Ошибки представлены в пользовательском и диагностическом слоях отдельно.

---

## 8) Что убираем из активного списка как устаревшее

1. Старые пункты "добавить базовый editor route" и "включить только v1 file API" — уже реализованы.
2. Старые задачи без строгого контракта AI create-page (с silent fallback) — отменены.
3. Старые таски с ручным вводом модели в текстовое поле — заменяются на select-список.
4. Разрозненные TODO по editor фиксам переносим в этот единый backlog.

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

### Wave 3 — Queue / Schedule / Monitoring
1. Унифицировать таблицы, фильтры, пагинацию и loading/error states.
2. Ввести общий паттерн disable/guard для операций очереди.
3. Убрать расхождения в статусах между страницами.
4. Привести тексты и термины к одной русской локализации.

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
