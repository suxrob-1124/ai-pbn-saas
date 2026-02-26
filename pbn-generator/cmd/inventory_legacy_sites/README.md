# inventory_legacy_sites

CLI для инвентаризации legacy-доменов на удалённом сервере по CSV-манифесту.

## Что делает

- читает `legacy_sites_manifest.csv`;
- фильтрует строки по `server_id` (`--target`);
- по каждому домену ищет `DocumentRoot/root` в vhost-конфигах на target-хосте;
- определяет владельца каталога (`stat -> user:group`);
- в `--apply` обновляет в `domains`:
  - `published_path`
  - `site_owner`
  - `inventory_status` (`ok|partial|failed`)
  - `inventory_checked_at`
  - `inventory_error`
- пишет JSON-отчёт.

## Режимы

- `--mode dry-run`: только проверка и отчёт.
- `--mode apply`: отчёт + запись в БД.

## Пример dry-run

```bash
cd pbn-generator
go run ./cmd/inventory_legacy_sites \
  --manifest ../scripts/legacy_sites_manifest.csv \
  --target seotech-web-media1 \
  --mode dry-run \
  --batch-size 100 \
  --batch-number 1 \
  --concurrency 10 \
  --retries 3 \
  --retry-delay 2s \
  --jitter-min 100ms \
  --jitter-max 500ms \
  --report ../scripts/inventory_report_dry_run.json
```

## Пример apply

```bash
cd pbn-generator
go run ./cmd/inventory_legacy_sites \
  --manifest ../scripts/legacy_sites_manifest.csv \
  --target seotech-web-media1 \
  --mode apply \
  --batch-size 100 \
  --batch-number 1 \
  --concurrency 10 \
  --retries 3 \
  --retry-delay 2s \
  --jitter-min 100ms \
  --jitter-max 500ms \
  --report ../scripts/inventory_report_apply.json
```

## Требования по env

- `DB_DRIVER`, `DB_DSN`
- `DEPLOY_TARGETS_JSON` (рекомендуется) или fallback:
  - `DEPLOY_SSH_HOST`
  - `DEPLOY_SSH_USER`
  - `DEPLOY_SSH_KEY_PATH`
- `DEPLOY_KNOWN_HOSTS_PATH` (для строгой host key проверки)
