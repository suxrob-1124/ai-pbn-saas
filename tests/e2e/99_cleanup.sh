#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck disable=SC1091
source "${SCRIPT_DIR}/lib.sh"

BACKEND_URL="$(state_get backend_url)"
COOKIE_FILE="$(state_get cookie_file)"
LOG_DIR="$(state_get log_dir)"
PROJECT_ID="$(state_get project_id)"
RUN_ID="$(state_get run_id)"
DOCKER_STARTED="$(state_get docker_started)"

log "[99_cleanup] start" | tee -a "${LOG_DIR}/99_cleanup.log"

ensure_login_state

if [[ -n "${PROJECT_ID}" ]]; then
  if [[ "${E2E_KEEP_PROJECT:-}" == "true" ]]; then
    log "project deletion skipped (E2E_KEEP_PROJECT=true)" | tee -a "${LOG_DIR}/99_cleanup.log"
  else
    resp=$(request DELETE "/api/projects/${PROJECT_ID}" "" "" "${COOKIE_FILE}")
    status="$(get_status "${resp}")"
    if [[ "${status}" != "200" ]]; then
      log "project delete failed: $(get_body "${resp}")" | tee -a "${LOG_DIR}/99_cleanup.log"
    else
      log "project deleted" | tee -a "${LOG_DIR}/99_cleanup.log"
    fi
  fi
fi

if [[ -n "${RUN_ID}" ]]; then
  log "[99_cleanup] removing published files" | tee -a "${LOG_DIR}/99_cleanup.log"
  rm -rf "${ROOT_DIR}/server/e2e-${RUN_ID}-"* || true
fi

if [[ "${DOCKER_STARTED}" == "true" ]]; then
  log "[99_cleanup] collecting docker logs" | tee -a "${LOG_DIR}/99_cleanup.log"
  (
    cd "${ROOT_DIR}"
    docker compose logs --no-color backend > "${LOG_DIR}/docker_backend.log" 2>&1 || true
    docker compose logs --no-color worker > "${LOG_DIR}/docker_worker.log" 2>&1 || true
    docker compose logs --no-color scheduler > "${LOG_DIR}/docker_scheduler.log" 2>&1 || true
  )
fi

if [[ "${DOCKER_STARTED}" == "true" && "${E2E_KEEP_SERVICES:-}" != "true" ]]; then
  log "[99_cleanup] docker compose down" | tee -a "${LOG_DIR}/99_cleanup.log"
  (
    cd "${ROOT_DIR}"
    if [[ "${E2E_CLEAN_VOLUMES:-}" == "true" ]]; then
      docker compose down -v
    else
      docker compose down
    fi
  ) >>"${LOG_DIR}/99_cleanup.log" 2>&1
fi

restore_env_backup
log "[99_cleanup] done" | tee -a "${LOG_DIR}/99_cleanup.log"
