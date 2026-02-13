# Sprint 4: Редактор импортированных и сгенерированных сайтов v1

## Цель
Запустить рабочий UI-редактор домена (`/domains/[id]/editor`) поверх существующего File API, чтобы редактировать как импортированные (`import_legacy`), так и сгенерированные сайты.

## Scope

### In Scope
- Backend: `my_role` в `GET /api/projects/:id/summary` и `GET /api/domains/:id/summary`.
- Frontend: новая страница редактора `/domains/[id]/editor`.
- Компоненты редактора:
  - `FileTree`
  - `MonacoEditor`
  - `EditorToolbar`
  - `FileHistory`
- Ролевая модель UI:
  - `viewer` — только чтение.
  - `owner/editor/admin` — чтение и сохранение.
- Интеграция переходов:
  - из логов ссылок на странице домена;
  - из карточек доменов на странице проекта.
- Тесты backend/frontend под editor-flow.

### Out of Scope
- `/admin/server` и `ServerFolderList`.
- Snapshot-based diff/revert.
- Автосохранение.
- Массовые файловые операции (create/rename/move folder).

## Изменения API/контрактов
- `GET /api/projects/:id/summary`:
  - добавлено поле `my_role: admin|owner|editor|viewer`.
- `GET /api/domains/:id/summary`:
  - добавлено поле `my_role: admin|owner|editor|viewer`.
- Файловые endpoints без breaking changes:
  - `GET /api/domains/:id/files`
  - `GET /api/domains/:id/files/*path`
  - `PUT /api/domains/:id/files/*path`
  - `GET /api/domains/:id/files/:fileId/history`

## Реализация (потоки)

### Поток 1. Backend role-awareness
- Добавить вычисление роли текущего пользователя в project/domain summary.
- Возвращать `my_role` в DTO summary.
- Сохранить текущую авторизацию file API без изменений:
  - `viewer+` для чтения;
  - `editor+` для `PUT`;
  - `admin` для `DELETE`.

### Поток 2. Страница `/domains/[id]/editor`
- Загрузка:
  1. `GET /api/domains/:id/summary`;
  2. `listFiles(domainId)`;
  3. открыть файл по query `path`, иначе первый editable, иначе первый в списке.
- Query support:
  - `path=<relative/path>`;
  - `line=<N>` для позиционирования в редакторе.
- Monaco:
  - языки: html/css/js/ts/json/xml/markdown/plaintext.
- MIME политика v1:
  - editable: `text/*`, `application/json`, `application/javascript`, `application/xml`, `image/svg+xml`;
  - остальные только metadata + download.
- Toolbar:
  - `Save` (manual);
  - `Revert` (локально);
  - `Download`.
- History:
  - показать metadata из `file_edits`;
  - заметка: diff/revert в v2.
- UX:
  - предупреждение при несохраненных изменениях (`beforeunload`);
  - подтверждение при переключении файла с dirty state;
  - явный toast на 403 сохранения.

### Поток 3. Интеграция навигации
- Страница домена:
  - активировать переход "Открыть в редакторе" в логах ссылок;
  - передавать `path` и `line`.
- Страница проекта:
  - добавить action "Редактор" в карточке домена;
  - для непубликованных доменов показать disabled-state с подсказкой.

### Поток 4. Тесты и верификация
- Backend:
  - `my_role` в summary;
  - file API permissions: owner/editor can `PUT`, viewer cannot.
- Frontend verify scripts:
  - `verify:file-editor-route`
  - `verify:file-editor-permissions`
  - `verify:file-editor-query-open`
- Ручной smoke:
  - импортированный домен редактируется и сохраняется;
  - сгенерированный домен редактируется и сохраняется;
  - viewer не может сохранить;
  - переход из link logs открывает нужный файл/строку.

## Definition of Done
- Работает `/domains/[id]/editor` для published доменов.
- `owner/editor/admin` сохраняют изменения через UI.
- `viewer` открывает страницу в read-only режиме.
- История файла отображается через `file_edits`.
- Интеграционные переходы в editor работают из домена и проекта.
- Нет breaking changes в существующих File API и страницах.

## Риски и mitigation
- Большие файлы и Monaco:
  - warning + fallback в read-only/plaintext для тяжелых кейсов.
- Ожидания по diff/revert:
  - явное сообщение в UI: "Diff/Revert в v2".
- Конкурентные правки:
  - v1: last-write-wins + понятные уведомления об ошибках.

## Принятые допущения
1. Sprint 4 ограничен core editor + интеграцией.
2. История v1 только metadata (`file_edits`), без snapshot.
3. Доступ к странице editor — все участники проекта.
4. Право сохранения — только owner/editor/admin.
5. Сохранение — только manual save.
