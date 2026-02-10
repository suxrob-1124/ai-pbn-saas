#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck disable=SC1091
source "${SCRIPT_DIR}/lib.sh"

ENV_FILE_ROOT="${ROOT_DIR}/.env"
ENV_FILE_E2E="${ROOT_DIR}/tests/e2e/.env"

if [[ -f "${ENV_FILE_E2E}" ]]; then
  set -a
  # shellcheck disable=SC1090
  source "${ENV_FILE_E2E}"
  set +a
fi

if [[ -f "${ENV_FILE_ROOT}" ]]; then
  set -a
  # shellcheck disable=SC1090
  source "${ENV_FILE_ROOT}"
  set +a
fi

read_env_key() {
  local file="$1"
  local key="$2"
  if [[ ! -f "${file}" ]]; then
    return 1
  fi
  python3 - "${file}" "${key}" <<'PY'
import os, sys
path = sys.argv[1]
key = sys.argv[2]
value = ""
with open(path, "r", encoding="utf-8") as f:
    for line in f:
        line = line.strip()
        if not line or line.startswith("#") or "=" not in line:
            continue
        k, v = line.split("=", 1)
        if k.strip() == key:
            value = v.strip().strip("\"").strip("'")
            break
print(value)
PY
}

if [[ -z "${GEMINI_API_KEY:-}" ]]; then
  GEMINI_API_KEY="$(read_env_key "${ENV_FILE_E2E}" "GEMINI_API_KEY" || true)"
fi
if [[ -z "${GEMINI_API_KEY:-}" ]]; then
  GEMINI_API_KEY="$(read_env_key "${ENV_FILE_ROOT}" "GEMINI_API_KEY" || true)"
fi
export GEMINI_API_KEY

BACKEND_URL="${BACKEND_URL:-http://localhost:8080}"
FRONTEND_URL="${FRONTEND_URL:-http://localhost:3000}"

require_cmd curl
require_cmd python3
require_cmd docker

require_env GEMINI_API_KEY

RESUME_USED="false"
if [[ "${E2E_RESUME:-}" == "true" && -f "${STATE_FILE}" ]]; then
  prev_run_id="$(state_get run_id)"
  prev_cookie_file="$(state_get cookie_file)"
  prev_log_dir="$(state_get log_dir)"
  prev_backend_url="$(state_get backend_url)"
  prev_frontend_url="$(state_get frontend_url)"
  if [[ -n "${prev_run_id}" && -n "${prev_log_dir}" ]]; then
    RUN_ID="${prev_run_id}"
    COOKIE_FILE="${prev_cookie_file:-${COOKIE_DIR}/e2e_${RUN_ID}.cookie}"
    LOG_DIR="${prev_log_dir}"
    if [[ -n "${prev_backend_url}" ]]; then
      BACKEND_URL="${prev_backend_url}"
    fi
    if [[ -n "${prev_frontend_url}" ]]; then
      FRONTEND_URL="${prev_frontend_url}"
    fi
    RESUME_USED="true"
  fi
fi

if [[ "${RESUME_USED}" != "true" ]]; then
  RUN_ID="${RUN_ID:-$(date +%Y%m%d%H%M%S)-$RANDOM}"
  COOKIE_FILE="${COOKIE_DIR}/e2e_${RUN_ID}.cookie"
  LOG_DIR="${LOG_ROOT}/${RUN_ID}"
fi

mkdir -p "${LOG_DIR}"

state_set "run_id" "${RUN_ID}"
state_set "cookie_file" "${COOKIE_FILE}"
state_set "backend_url" "${BACKEND_URL}"
state_set "frontend_url" "${FRONTEND_URL}"
state_set "log_dir" "${LOG_DIR}"

if [[ "${RESUME_USED}" == "true" ]]; then
  log "e2e env resume: run_id=${RUN_ID}"
else
  log "e2e env ready: run_id=${RUN_ID}"
fi
