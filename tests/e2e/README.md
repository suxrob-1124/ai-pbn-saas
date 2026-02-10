# E2E Tests

Этот набор e2e-скриптов поднимает инфраструктуру, создает тестовые данные, запускает генерации и проверяет расписания/линкбилдинг в условиях, максимально близких к продакшену.

## Требования
- Docker + Docker Compose
- `curl`, `python3`, `bash`
- Действующий ключ Gemini (`GEMINI_API_KEY` в окружении)

## Быстрый запуск
```bash
export GEMINI_API_KEY="<your-key>"
./tests/e2e/run_all.sh
```

## Переменные окружения
- `BACKEND_URL` (default: `http://localhost:8080`)
- `FRONTEND_URL` (default: `http://localhost:3000`)
- `GEMINI_API_KEY` (обязателен)
- `E2E_DATASET` (`surstrem` по умолчанию, можно `xbet`) — набор реалистичных доменов/ключевых слов для SERP
- `E2E_DOMAIN_PREFIX` (default: `e2e-<run_id>`) — префикс поддомена для уникальности URL
- `E2E_GEN_RETRIES` (default: `3`) — число повторов генерации при transient ошибках SERP
- `E2E_GEN_RETRY_DELAY` (default: `60`) — задержка между повторами (сек)
- `E2E_APIKEY_RETRIES` (default: `3`) — повторы сохранения API ключа при сетевых сбоях
- `E2E_APIKEY_RETRY_DELAY` (default: `10`) — задержка между повторами (сек)
- `E2E_EMAIL` / `E2E_PASSWORD` (опционально)
- `PROJECT_NAME`, `PROJECT_COUNTRY`, `PROJECT_LANG`
- `E2E_KEEP_SERVICES=true` (не останавливать контейнеры в cleanup)
- `E2E_CLEAN_VOLUMES=true` (docker compose down -v)
- `E2E_DB_SEED=true` (по умолчанию, запуск seed контейнера для системных промптов)
- `E2E_CLEANUP=false` (не запускать `99_cleanup.sh` после прогона)
- `E2E_KEEP_PROJECT=true` (не удалять проект в cleanup)
- `E2E_START_STEP` (например: `04_schedule`) — старт с нужного шага
- `E2E_SKIP_STEPS` (например: `03_generation,05_link_schedule`) — пропуск шагов
- `E2E_RESUME=true` — продолжить с последнего успешного шага из `tests/e2e/state.json`

## Что делает прогон
1. `00_env.sh` — проверка окружения, генерация `run_id`.
2. `01_up.sh` — старт docker-инфры + миграции.
3. `02_seed.sh` — регистрация/логин, сохранение API ключа, создание проекта/доменов, линк-настройки.
4. `03_generation.sh` — ручная генерация домена A, проверка публикации.
5. `04_schedule.sh` — создание расписаний, ожидание автогенерации домена B, ожидание link-тасков.
6. `05_link_schedule.sh` — ручной trigger link-schedule + ручной link run.
7. `06_verify.sh` — финальная проверка.
8. `99_cleanup.sh` — удаление тестовых данных и cleanup.

Логи находятся в `tests/e2e/logs/<run_id>/`.

## Resume
Если прогон упал, можно продолжить без полного пересоздания:
```bash
E2E_RESUME=true E2E_CLEANUP=false ./tests/e2e/run_all.sh
```
По умолчанию будет выбран следующий шаг после `last_success_step`.
```
E2E_RESUME=true E2E_START_STEP=04_schedule ./tests/e2e/run_all.sh
```
