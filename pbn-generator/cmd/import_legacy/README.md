# import_legacy (единая legacy-утилита)

Единая CLI для импорта уже сгенерированных сайтов в БД, включая файлы, synthetic generation/artifacts и inventory-метаданные.

Утилита покрывает оба сценария:
- `local`: источник файлов `server/<domain>`.
- `remote`: источник файлов удаленный сервер по SSH (`DEPLOY_MODE=ssh_remote` или `--source remote`).

## Что делает

1. Читает CSV-манифест.
2. Создает/обновляет `project` и `domain`.
3. Синхронизирует файлы сайта в `site_files`.
4. Публикует домен в `published`.
5. Создает/обновляет synthetic decode artifacts (`legacy_decode_v2`).
6. Пишет JSON-отчет.

Дополнительно для `remote`:
1. Делает SSH probe (published_path/site_owner).
2. Зеркалит удаленные файлы во временный локальный mirror.
3. После `apply` обновляет `published_path`, `site_owner`, `inventory_*`, `deployment_mode=ssh_remote`.

## Ключевые флаги

- `--source auto|local|remote` (по умолчанию `auto`)
- `--target <alias>` fallback server alias, если в CSV нет `server_id`
- `--keep-mirror` не удалять temp mirror после завершения
- `--mode dry-run|apply`
- `--manifest <path>`
- `--batch-size`, `--batch-number`
- `--report <path>`
- `--force` принудительно обновлять synthetic artifacts

## CSV формат

Обязательные колонки:

```csv
project_name,owner_email,project_country,project_language,domain_url,main_keyword
```

Опциональные:

```csv
exclude_domains,server_id
```

## Примеры запуска

Из директории `pbn-generator`.

### 1) Local source (legacy `server/`)

```bash
go run ./cmd/import_legacy \
  --manifest ../tmp/legacy_sites.csv \
  --mode apply \
  --source local \
  --server-dir ../server \
  --batch-size 50 \
  --batch-number 1 \
  --report ../tmp/import_local_report.json
```

### 2) Remote source (ssh_remote)

```bash
go run ./cmd/import_legacy \
  --manifest ../tmp/legacy_sites.csv \
  --mode apply \
  --source remote \
  --target media1 \
  --batch-size 50 \
  --batch-number 1 \
  --report ../tmp/import_remote_report.json
```

### 3) Auto source (по `DEPLOY_MODE`)

```bash
go run ./cmd/import_legacy \
  --manifest ../tmp/legacy_sites.csv \
  --mode dry-run \
  --source auto \
  --batch-size 50 \
  --batch-number 1 \
  --report ../tmp/import_auto_dry.json
```

## Требования для remote

- `DEPLOY_TARGETS_JSON` заполнен и содержит alias из `server_id`/`--target`.
- `DEPLOY_KNOWN_HOSTS_PATH` валиден.
- SSH ключи доступны (`0600`).
- `DB_DRIVER` и `DB_DSN` указывают на целевую БД.
