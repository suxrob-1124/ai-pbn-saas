# import_legacy

CLI для импорта уже опубликованных сайтов из `server/<domain>` в БД.

## Что делает

1. Читает CSV-манифест.
2. Резолвит/создает проекты.
3. Резолвит/создает домены (при необходимости переносит домен в проект из манифеста).
4. Синхронизирует файлы из `server/<domain>` в `site_files` (включая удаление stale-записей).
5. Ставит домен в `published`.
6. Пытается декодировать внешнюю `https://` ссылку из `index.html` (`body a[href]`) и записывает baseline link state.
7. Декодирует файлы сайта и создает/обновляет synthetic generation с артефактами (`legacy_decode_v2`).
8. Пишет JSON-отчет.

## Важный момент: `dry-run` ничего не записывает

Если запускали с `--mode dry-run`, в UI изменений не будет.  
В отчете это видно по действиям с префиксом `would`/`preview`, например:
- `domain_would_create`
- `files_sync_preview`
- `domain_publish_preview`
- `link_decode_preview`

Для фактического импорта нужен `--mode apply`.

## Требования

- Доступная PostgreSQL.
- Корректные `DB_DRIVER` и `DB_DSN`.

Если запускаете CLI с хоста (не из docker-контейнера), обычно нужен DSN с `localhost`, например:

```bash
export DB_DRIVER=pgx
export DB_DSN='postgres://auth:auth@localhost:5432/auth?sslmode=disable'
```

## Формат CSV (v1)

Обязательные колонки:

```csv
project_name,owner_email,project_country,project_language,domain_url,main_keyword
```

Опциональные:

```csv
exclude_domains,server_id
```

Пример:

```csv
project_name,owner_email,project_country,project_language,domain_url,main_keyword,exclude_domains,server_id
surstrem,manager@example.com,se,sv,profitnesscamps.se,"Insättning och uttag på utländska casinon","","seotech-web-media1"
1xbet-ru,manager2@example.com,ru,ru,dialog-c.ru,"1хБет вывод средств","","seotech-web-media1"
```

## Команды запуска

Из директории `pbn-generator`:

### 1) Проверка без записи (dry-run)

```bash
go run ./cmd/import_legacy \
  --manifest ../scripts/legacy_sites_manifest.csv \
  --mode dry-run \
  --batch-size 50 \
  --batch-number 1 \
  --server-dir ../server \
  --decode-source import_legacy \
  --report ../scripts/legacy_import_report.json
```

### 2) Реальный импорт (apply)

```bash
go run ./cmd/import_legacy \
  --manifest ../scripts/legacy_sites_manifest.csv \
  --mode apply \
  --batch-size 50 \
  --batch-number 1 \
  --server-dir ../server \
  --decode-source import_legacy \
  --report ../scripts/legacy_import_report_apply.json
```

### 3) Импорт батчами (пример для 500 строк)

```bash
for n in $(seq 1 10); do
  go run ./cmd/import_legacy \
    --manifest ../scripts/legacy_sites_manifest.csv \
    --mode apply \
    --batch-size 50 \
    --batch-number "$n" \
    --server-dir ../server \
    --decode-source import_legacy \
    --report "../scripts/legacy_import_batch_${n}.json"
done
```

### 4) Принудительное обновление synthetic artifacts

Если у домена уже есть не-legacy генерации, importer по умолчанию не перезаписывает synthetic decode.
Для принудительного режима добавьте `--force`:

```bash
go run ./cmd/import_legacy \
  --manifest ../scripts/legacy_sites_manifest.csv \
  --mode apply \
  --batch-size 50 \
  --batch-number 1 \
  --server-dir ../server \
  --decode-source import_legacy \
  --force \
  --report ../scripts/legacy_import_force_report.json
```

## Как понять результат после запуска

Смотрите `summary` и `rows[*].actions` в JSON-отчете.

- `success` = строка обработана без предупреждений.
- `warned` = импорт прошел, но есть warning (например не найдена внешняя `https://` ссылка).
- `failed` = строка не импортирована.

Ключевые действия в `apply`:

- `project_created` / `project_updated` / `project_reused`
- `domain_created` / `domain_moved` / `domain_reused`
- `files_synced`
- `domain_published`
- `link_baseline_created` (если ссылка найдена)
- `legacy_artifacts_created` / `legacy_artifacts_updated` / `legacy_artifacts_unchanged`
- `legacy_artifacts_skipped_non_legacy_exists` (если есть обычные генерации и не указан `--force`)

## Почему “добавления не видно на сайте” (чеклист)

1. Запуск был в `dry-run`, а не `apply`.
2. CLI писал в другую БД, чем backend/frontend (часто проблема `DB_DSN`).
3. Открыт не тот проект/пользователь в UI.
4. Данные ушли в другой батч (`batch-number`).

Быстрая проверка в БД:

```sql
SELECT p.name AS project, d.url, d.status, d.published_at
FROM domains d
JOIN projects p ON p.id = d.project_id
ORDER BY p.name, d.url;
```
