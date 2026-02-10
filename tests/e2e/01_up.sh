#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck disable=SC1091
source "${SCRIPT_DIR}/lib.sh"

RUN_ID="$(state_get run_id)"
BACKEND_URL="$(state_get backend_url)"
LOG_DIR="$(state_get log_dir)"
ENV_FILE="${ROOT_DIR}/.env"

require_env GEMINI_API_KEY

log "[01_up] ensuring GEMINI_API_KEY is available for containers" | tee -a "${LOG_DIR}/01_up.log"
ensure_env_key "${ENV_FILE}" "GEMINI_API_KEY" "${GEMINI_API_KEY}"

log "[01_up] starting docker services" | tee -a "${LOG_DIR}/01_up.log"
(
  cd "${ROOT_DIR}"
  docker compose up -d db redis migrate backend worker scheduler frontend
) >>"${LOG_DIR}/01_up.log" 2>&1

log "[01_up] waiting for migrate to finish" | tee -a "${LOG_DIR}/01_up.log"
MIGRATE_ID="$(cd "${ROOT_DIR}" && docker compose ps -q migrate)"
if [[ -n "${MIGRATE_ID}" ]]; then
  docker wait "${MIGRATE_ID}" >>"${LOG_DIR}/01_up.log" 2>&1 || true
  MIGRATE_STATUS="$(docker inspect -f '{{.State.ExitCode}}' "${MIGRATE_ID}")"
  if [[ "${MIGRATE_STATUS}" != "0" ]]; then
    die "migrate failed with exit code ${MIGRATE_STATUS}"
  fi
fi

E2E_DB_SEED="${E2E_DB_SEED:-true}"
if [[ "${E2E_DB_SEED}" == "true" ]]; then
  log "[01_up] seeding database (system_prompts)" | tee -a "${LOG_DIR}/01_up.log"
  (
    cd "${ROOT_DIR}"
    docker compose run --rm seed
  ) >>"${LOG_DIR}/01_up.log" 2>&1 || die "db seed failed"
fi

log "[01_up] waiting for backend health" | tee -a "${LOG_DIR}/01_up.log"
if ! wait_for_http "${BACKEND_URL}/healthz" 180; then
  die "backend health check failed"
fi

state_set "docker_started" "true"
log "[01_up] services are up" | tee -a "${LOG_DIR}/01_up.log"
