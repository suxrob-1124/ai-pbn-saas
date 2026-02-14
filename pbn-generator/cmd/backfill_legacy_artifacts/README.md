# backfill_legacy_artifacts

CLI для декодирования уже импортированных сайтов и записи synthetic generation (`legacy_decode_v2`) в `generations.artifacts`.

## Что делает

1. Читает тот же CSV-манифест, что и `import_legacy`.
2. Для каждой строки находит домен в БД по `domain_url`.
3. Проверяет наличие папки `server/<domain>`.
4. Декодирует артефакты сайта (`final_html`, `css_content`, `js_content`, `404_html`, `logo_svg`, `favicon_tag`, `generated_files`).
5. Создает или обновляет synthetic generation с `prompt_id=legacy_decode_v2`.
6. Пишет JSON-отчет по батчу.

## Флаги

- `--manifest <path>` — путь к CSV (обязательно)
- `--mode dry-run|apply` — режим запуска (обязательно)
- `--batch-size <N>` — размер батча, default `50`
- `--batch-number <K>` — номер батча, default `1`
- `--server-dir <path>` — корень сайтов, default `server`
- `--report <path>` — путь к JSON-отчету
- `--force` — разрешить rewrite synthetic decode даже если у домена есть non-legacy generation

## Пример dry-run

```bash
cd pbn-generator
go run ./cmd/backfill_legacy_artifacts \
  --manifest ../scripts/legacy_sites_manifest.csv \
  --mode dry-run \
  --batch-size 50 \
  --batch-number 1 \
  --server-dir ../server \
  --report ../scripts/legacy_backfill_report.json
```

## Пример apply

```bash
cd pbn-generator
go run ./cmd/backfill_legacy_artifacts \
  --manifest ../scripts/legacy_sites_manifest.csv \
  --mode apply \
  --batch-size 50 \
  --batch-number 1 \
  --server-dir ../server \
  --report ../scripts/legacy_backfill_report_apply.json
```

## Отчет

`summary` содержит:
- `processed`
- `success`
- `failed`
- `warned`
- `decoded`
- `updated`
- `skipped`
- `unchanged`

