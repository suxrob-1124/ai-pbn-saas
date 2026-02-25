# D0.1 Gap Report: README + Front Docs + OpenAPI

Дата: 2026-02-25

## 1. Scope и источники

Проверены:
- `README.md`
- `frontend/app/docs/*`
- `frontend/openapi.yaml`
- backend routing: `pbn-generator/internal/httpserver/server.go` (+ editor handlers)

Цель D0.1: зафиксировать, где документация расходится с текущим продуктом, и задать безопасный порядок исправлений.

## 2. Executive summary

1. `README.md` содержит большой исторический слой и ручной API-каталог, который частично устарел и местами конфликтует с реальными endpoint-ами.
2. Front docs в `frontend/app/docs` полезны как обзор, но описывают не все рабочие сценарии и неудобны для частого редактирования (контент зашит в TSX).
3. `frontend/openapi.yaml` покрывает много API (75 path), но есть явные пробелы относительно текущего backend routing.
4. Нужна контрактная синхронизация: один источник правды по API (`openapi.yaml`), а `README` и docs должны ссылаться на него, а не дублировать длинные endpoint-листы вручную.

## 3. Что найдено (факты)

## 3.1 OpenAPI: подтвержденные пробелы (P0)

В backend есть роуты, которых нет в `frontend/openapi.yaml`:
- `/api/captcha` (`server.go:305`)
- `/api/profile` (`server.go:307`)
- `/api/profile/api-key` (`server.go:308`)
- `/api/password` (`server.go:310`)
- `/api/email/change/request` (`server.go:311`)
- `/api/email/change/confirm` (`server.go:312`)
- `/api/verify/request` (`server.go:313`)
- `/api/verify/confirm` (`server.go:314`)
- `/api/password/reset/request` (`server.go:315`)
- `/api/password/reset/confirm` (`server.go:316`)
- `/api/domains/{domainId}/editor/ai-regenerate-asset` (dispatch в `server.go:2418`, handler в `editor_ai_handlers.go:332`)
- `/api/projects/{projectId}/prompts/{stage?}` (`server.go:1227`)
- `/api/domains/{domainId}/prompts/{stage?}` (`server.go:2028`)
- `/api/domains/{domainId}/deployments` (`server.go:2030`)

Риск: Swagger и generated types не отражают полный API-контракт -> клиенты/QA ориентируются на неполную схему.

## 3.2 README: зоны устаревания/конфликта (P0/P1)

Подтверждено:
- В `README.md` есть утверждения «не реализовано», которые противоречат текущему коду (например, OpenAPI/Swagger уже есть в `frontend/openapi.yaml` и docs UI).
- В `README.md` длинный ручной API-каталог дублирует OpenAPI и быстро устаревает.
- Есть исторические названия/команды (например, `go run ./cmd/authserver`), требующие сверки с текущим layout.
- Есть смешение «текущего состояния» и «roadmap», из-за чего сложно понять, что уже в проде, а что план.

Риск: README вводит в заблуждение при онбординге и ручных операциях.

## 3.3 Front docs (`frontend/app/docs`): содержательные и UX-пробелы (P1)

Подтверждено:
- Docs-страницы короткие и обзорные, не закрывают все рабочие сценарии (особенно editor AI, LLM usage, role-specific workflows, troubleshooting по ошибкам).
- Контент хранится прямо в TSX (`page.tsx`), что неудобно для регулярных правок контента и ревью изменений документации.
- Нет единого шаблона doc-страниц (audience, prerequisites, happy path, failure modes, related API).

Риск: документация есть, но не служит как полноценный runbook.

## 4. Приоритизация

## P0 (блокирует актуальность контрактов)
1. Закрыть пробелы OpenAPI по отсутствующим endpoint-ам.
2. Убрать в README ручной endpoint-справочник как primary source; оставить ссылку на Swagger + краткий API map.
3. Сверить команды/пути запуска в README с текущим кодом.

## P1 (качество и поддерживаемость docs)
1. Перевести docs-контент в контентный слой (MDX/structured content), чтобы не править TSX при каждой текстовой правке.
2. Добавить подробные сценарии по generation/link/indexing/editor/llm usage.
3. Добавить секцию troubleshooting с типовыми фейлами и действиями.

## P2 (операционная устойчивость)
1. Добавить CI-проверки синхронизации docs/openapi.
2. Добавить lint/check broken links и policy «API-PR без OpenAPI update не проходит».

## 5. План реализации (D1–D5)

## D1: README cleanup (содержимое и структура)
- Упростить README до: обзор продукта, quick start, env, ops, ссылки на docs/openapi.
- Сжать API-раздел: без сотен curl-блоков, только ключевые группы + ссылка на Swagger/docs.
- Развести «Current state» и «Roadmap».

Результат: README не конфликтует с кодом и не дублирует OpenAPI.

## D2: Front docs rework (контент + редактируемость)
- Вынести контент docs из TSX в `frontend/docs-content/*` (например, MDX или JSON+renderer).
- Оставить в `app/docs` только layout/navigation/rendering.
- Ввести единый шаблон doc-страницы.

Результат: документацию можно быстро править, не трогая UI-код.

## D3: OpenAPI parity patch
- Добавить отсутствующие endpoint-ы и схемы.
- Проверить status-codes и error payload consistency.
- Синхронизировать Swagger UI docs page.

Результат: `frontend/openapi.yaml` покрывает фактический API.

## D4: Content hardening
- Расписать end-to-end сценарии использования сервиса (по ролям).
- Добавить troubleshooting-карту «симптом -> причина -> решение».
- Добавить «что делать при частых сбоях» (timeouts, queue stuck, asset errors, permission errors).

## D5: Quality gates
- Добавить автоматические проверки:
  - openapi lint,
  - doc link check,
  - минимальный route coverage check (backend vs openapi inventory).
- Добавить PR checklist для документации.

## 6. Предложенный порядок микро-шагов

1. `[x]` `D1.1` README skeleton rewrite (без OpenAPI правок).
2. `[x]` `D1.2` README factual sync (команды/пути/разделы).
3. `[x]` `D3.1` OpenAPI missing endpoints patch (P0).
4. `[x]` `D3.2` OpenAPI schema/error consistency patch.
5. `[x]` `D2.1` Docs content-layer scaffold.
6. `[ ]` `D2.2` Миграция 2-3 ключевых docs-страниц на новый формат.
7. `[ ]` `D4.1` Расширение сценариев + troubleshooting.
8. `[ ]` `D5.1` CI checks + policy.

## 7. Критерии завершения блока

1. README не содержит явных противоречий с текущим продуктом.
2. OpenAPI описывает все публичные endpoint-ы backend в текущем scope.
3. Front docs покрывают основные рабочие сценарии и ошибки.
4. Документация редактируется через контентный слой, а не только через TSX.
5. Есть автоматическая защита от повторного рассинхрона.
