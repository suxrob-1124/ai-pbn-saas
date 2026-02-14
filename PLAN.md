# Editor V2: Детальный план по спринтам (без поломки текущего v1)

## Краткое резюме
Цель: вывести редактор домена из “v1-текстового” состояния в полноценный рабочий инструмент:
1. визуальная работа с файлами (включая изображения),
2. создание/переименование/удаление файлов,
3. live preview HTML как сайта, а не только сырого фрагмента,
4. история с `Diff/Revert`,
5. AI-режим: “предложить изменения → пользователь применяет” + создание новых страниц через AI.

Выбранные решения (зафиксировано):
- История: **full snapshot**.
- AI UX: **suggest then apply**.
- AI scope первого релиза: **single-file edit + new page generation**.
- Проектные prompt overrides остаются как есть; доменные — с фокусом на актуальный этап.

---

## Что уже есть (зафиксировано по коду)
1. Уже реализован editor route: `frontend/app/domains/[id]/editor/page.tsx`.
2. File API v1 есть: list/get/put/delete/history (`/api/domains/:id/files...`), но:
- create/move/rename/upload отсутствуют,
- история хранит только metadata/hash (`file_edits`), без snapshot content.
3. На `/domains/[id]` уже есть:
- кнопка “Открыть в редакторе”,
- блок “Результат”,
- modal preview `final_html`,
- live preview из `index.html/style.css/script.js` через runtime rewrite.
4. Prompt overrides и выбор моделей уже есть (`PromptOverridesPanel`), плюс подсказка по переменным.

---

## Scope / Out of scope

### In scope (V2)
1. File Manager v2 (preview images, create/rename/move/delete, upload binary).
2. История версий со snapshot + diff + revert.
3. Live preview внутри editor.
4. AI-assisted edit (single file) + AI page creation (multi-file proposal).
5. Совместимость с импортированными и сгенерированными сайтами.

### Out of scope (этот контур)
1. Реальный SSH деплой и runtime web-server orchestration.
2. Совместное редактирование в реальном времени (OT/CRDT).
3. Массовый refactor всего сайта одним AI-запросом.
4. Link/index/schedule logic (кроме совместимости с текущим UI).

---

## Изменения в API / интерфейсах / типах

## 1) Backend File API (additive, без ломающего удаления v1)
1. `POST /api/domains/:id/files`
- Создать файл/папку.
- Body: `{ path, kind: "file"|"dir", content?, mime_type? }`.

2. `PATCH /api/domains/:id/files/:path`
- Операции rename/move.
- Body: `{ op: "move", new_path }`.

3. `POST /api/domains/:id/files/upload`
- Multipart upload для бинарных файлов (изображения и т.д.).

4. `GET /api/domains/:id/files/:path/meta`
- Метаданные (size, mime, updated_at, is_editable, dimensions для image).

5. `GET /api/domains/:id/files/:path/history`
- История по path (новый канон), включая revision info.
- Старый `.../:fileId/history` оставить как legacy-compatible.

6. `POST /api/domains/:id/files/:path/revert`
- Body: `{ revision_id, description? }`.
- Создает новую ревизию и возвращает новый `version`.

7. `PUT /api/domains/:id/files/:path` (расширение)
- Добавить optimistic locking: `expected_version`.
- На конфликт: `409` + `{ current_version, current_hash, updated_by, updated_at }`.

## 2) AI API (editor assistant)
1. `POST /api/domains/:id/files/:path/ai-suggest`
- Body: `{ instruction, model?, selection?, context_files? }`.
- Ответ: `{ suggested_content, diff_summary, warnings, prompt_trace, token_usage }`.
- Ничего не сохраняет.

2. `POST /api/domains/:id/editor/ai-create-page`
- Body: `{ instruction, target_path, with_assets: boolean, model? }`.
- Ответ: предложенный пакет файлов:
  `{ files: [{path, content, mime_type}], warnings, prompt_trace }`.
- Ничего не сохраняет.

3. Применение результатов AI — только через обычные file endpoints (явно пользователем).

## 3) Frontend типы
1. Расширить `FileListItem`:
- `version`, `isEditable`, `isBinary`, `width?`, `height?`.
2. Новый `FileRevisionDTO`:
- `id, file_id, version, edited_by, source(manual|ai|revert), description, created_at, content_hash`.
3. Новый `AIEditorSuggestionDTO` / `AIPageSuggestionDTO`.

---

## Изменения в БД (миграции)

1. `site_files`
- добавить `version INT NOT NULL DEFAULT 1`.
- добавить `last_edited_by TEXT NULL`.
- индекс `(domain_id, path)` уже есть, сохраняем.

2. Новая таблица `file_revisions`
- `id TEXT PK`
- `file_id TEXT FK -> site_files(id) ON DELETE CASCADE`
- `version INT NOT NULL`
- `content BYTEA NOT NULL`
- `content_hash TEXT NOT NULL`
- `size_bytes BIGINT NOT NULL`
- `mime_type TEXT NOT NULL`
- `source TEXT NOT NULL` (`manual|ai|revert|import`)
- `description TEXT NULL`
- `edited_by TEXT NOT NULL REFERENCES users(email)`
- `created_at TIMESTAMPTZ NOT NULL`
- `UNIQUE(file_id, version)`

3. Backfill migration/CLI
- для существующих `site_files` создать baseline revision `version=1` из файлов на диске.
- для старых `file_edits` без snapshot пометить как legacy events (без diff target).

---

## Спринты (decision-complete)

## Sprint V2.1 — File Manager UX + операции файлов
**Цель:** сделать редактор удобным для ежедневной работы с деревом и медиа.

1. Frontend (`/domains/[id]/editor`)
- левый сайдбар: дерево + quick actions (`New file`, `New folder`, `Rename`, `Delete`, `Upload`).
- для image/*: thumbnail preview + размеры + open full.
- для non-text: карточка с preview/download/meta вместо “только нельзя редактировать”.
- вынести live-preview util из domain-page в общий модуль (`lib/livePreview.ts`) для единой логики.

2. Backend
- реализовать `POST files`, `PATCH move`, `POST upload`, `GET meta`.
- права:
  - viewer: read-only,
  - owner/editor/admin: create/update/move/upload,
  - delete: owner/editor/admin (вместо admin-only для редактора v2).

3. Guard-правила
- запрет path traversal,
- deny-list на системные пути/скрытые служебные файлы,
- ограничения размера upload.

4. Acceptance
- можно создать `about.html`, переименовать, удалить, загрузить изображение и увидеть preview в editor.

---

## Sprint V2.2 — Versioning + Diff/Revert
**Цель:** полноценная история изменений с откатом.

1. Backend
- при каждом save:
  - сохранять snapshot в `file_revisions`,
  - инкрементировать `site_files.version`.
- `PUT` с `expected_version` и `409` на конфликт.
- `GET history by path`.
- `POST revert` создает новую revision и обновляет файл на диске + metadata.

2. Frontend
- `FileHistory`:
  - список ревизий,
  - `View diff` (Monaco Diff Editor),
  - `Revert to this version`.
- conflict modal:
  - показать “файл изменился кем-то еще”,
  - кнопки: `Reload`, `Overwrite`, `Cancel`.

3. UX
- unsaved-changes guard при смене файла/уходе.
- для binary revisions diff скрыт, доступен только revert/download.

4. Acceptance
- можно откатить любой text-файл к прошлой версии.
- конкурентное редактирование не перетирает молча изменения.

---

## Sprint V2.3 — AI Edit + AI New Page
**Цель:** ускорить редактирование через AI без риска “тихой порчи”.

1. AI suggest pipeline
- endpoint `ai-suggest` формирует предложение по текущему файлу.
- UI: чат-панель справа (`instruction`, модель, контекст-файлы).
- отображение результата:
  - diff preview,
  - summary изменений,
  - кнопки `Apply` / `Discard`.

2. AI create-page
- endpoint `ai-create-page` возвращает предложенный комплект:
  - `new-page.html`,
  - опционально `new-page.css`, `new-page.js`.
- UI wizard:
  - шаг 1: prompt,
  - шаг 2: review файлов,
  - шаг 3: apply в файловую структуру.

3. Prompt/model/variables
- использовать существующий контур prompt overrides + модель.
- добавить stage `editor_assistant` в системные prompts.
- в UI показывать source badge: `domain|project|global`.
- variables help расширить под editor-контекст (`current_file_path`, `current_file_content`, `domain_url`, `language`, `keyword`).

4. Безопасность AI
- apply только после явного подтверждения.
- hard cap на размер ответа.
- валидация mime/path.
- журналировать source=`ai` в revisions.

5. Acceptance
- AI предлагает корректный патч для текущего файла.
- AI создаёт новую страницу и связанные файлы с подтверждением перед записью.

---

## Sprint V2.4 — Live site preview + polish + release hardening
**Цель:** финальная практичность и стабильность.

1. Unified Preview
- split view в editor: `Code | Preview`.
- preview использует текущий buffer (несохраненные изменения) + runtime asset rewrite.
- toggle:
  - `Buffer preview` (локальный),
  - `Published preview` (с диска),
  - `Open domain` (внешняя ссылка).

2. Domain page интеграция
- кнопка “Открыть live сайт” (если published).
- “Просмотр HTML” и editor preview используют общий render engine (без рассинхрона).

3. Производительность
- lazy load для больших файлов,
- debounce preview refresh,
- virtualization в истории ревизий.

4. Документация
- `README` раздел “Editor V2”.
- API docs (`frontend/openapi.yaml`) для новых endpoints.
- runbook rollback/recovery для file revisions.

5. Release criteria
- ноль TS type errors,
- backend и frontend verify scripts green,
- smoke для import/generation/link не деградируют.

---

## Тесты и сценарии

## Backend
1. `file_api_test.go`
- create/move/upload/delete permissions.
- optimistic lock `409`.
- revert flow и корректный `version++`.

2. `sqlstore` tests
- `file_revisions` create/list/get by version.
- atomic update file + revision transaction.

3. Security tests
- traversal (`../`, encoded traversal),
- invalid mime/path reject.

4. AI endpoint tests
- suggest не пишет на диск/в DB.
- apply через обычный save создает revision with `source=ai`.

## Frontend
1. Verify scripts:
- `verify_file_editor_route_v2.ts`
- `verify_file_preview_images.ts`
- `verify_file_create_rename_delete.ts`
- `verify_file_diff_revert.ts`
- `verify_ai_editor_panel.ts`
- `verify_ai_create_page_wizard.ts`
- `verify_editor_live_preview_modes.ts`

2. `tsc --noEmit` + existing verify scripts regression.

## Smoke (ручной DoD)
1. Импортированный домен: открыть editor, изменить `style.css`, увидеть результат в preview, сохранить, откатить.
2. Сгенерированный домен: создать новую страницу через AI, применить, открыть preview.
3. Viewer: не может писать/удалять, но видит preview/history.
4. Conflict: два редактора, второй получает `409` и корректный UI.

---

## Риски и mitigation
1. Рост БД из-за snapshot
- gzip/compression для `content`,
- retention policy (опционально по количеству ревизий в v2.1+),
- метрики размера таблицы.

2. AI может предложить вредный/ломающий код
- mandatory review before apply,
- размер/тип валидация,
- явный журнал `source=ai`.

3. Рассинхрон preview между страницами
- единый модуль live preview для domain page + editor.

4. Конкурентные правки
- optimistic locking + conflict UX.

---

## Явные допущения и defaults
1. Редактор V2 делаем в ветке `feature/editor-v2`.
2. Full snapshot — для всех сохраняемых файлов через editor API.
3. AI режим по умолчанию: **suggest then apply**, авто-применения нет.
4. Первый AI-релиз: **single-file edit + new page generation**, без full-site refactor.
5. Существующие v1 endpoints сохраняются совместимыми; новые контракты additive.
6. Никаких изменений в generation/link/index scheduler логике в рамках этого плана.

---

## Финальный DoD по V2
1. Editor позволяет работать с текстом и изображениями полноценно.
2. Есть create/move/rename/delete/upload операции из UI.
3. Diff/Revert работает на snapshot-истории.
4. Live preview показывает фактический результат сайта (buffer/published).
5. AI редактирование и создание страниц работают через подтверждение пользователя.
6. Текущие рабочие контуры (import/generation/link/indexing) не ломаются регрессионно.
