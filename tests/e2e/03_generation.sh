#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck disable=SC1091
source "${SCRIPT_DIR}/lib.sh"

BACKEND_URL="$(state_get backend_url)"
COOKIE_FILE="$(state_get cookie_file)"
LOG_DIR="$(state_get log_dir)"
DOMAIN_ID="$(state_get domain_a_id)"
E2E_GEN_RETRIES="${E2E_GEN_RETRIES:-3}"
E2E_GEN_RETRY_DELAY="${E2E_GEN_RETRY_DELAY:-60}"
GEN_BODY_FILE="${LOG_DIR}/03_generation_last.json"
DOMAIN_BODY_FILE="${LOG_DIR}/03_domain.json"
FILES_BODY_FILE="${LOG_DIR}/03_files.json"

is_retryable_generation_error() {
  local msg="$1"
  msg="$(echo "${msg}" | tr '[:upper:]' '[:lower:]')"
  if [[ "${msg}" == *"serp"* && ( "${msg}" == *"eof"* || "${msg}" == *"status 5"* || "${msg}" == *"timeout"* ) ]]; then
    return 0
  fi
  if [[ "${msg}" == *"eof"* || "${msg}" == *"timeout"* || "${msg}" == *"connection reset"* || "${msg}" == *"temporary"* ]]; then
    return 0
  fi
  return 1
}

attempt=1
while (( attempt <= E2E_GEN_RETRIES )); do
  log "[03_generation] trigger generation (attempt ${attempt}/${E2E_GEN_RETRIES})" | tee -a "${LOG_DIR}/03_generation.log"
  resp=$(request POST "/api/domains/${DOMAIN_ID}/generate" "{}" "" "${COOKIE_FILE}")
  status="$(get_status "${resp}")"
  if [[ "${status}" != "202" ]]; then
    die "generation trigger failed: $(get_body "${resp}")"
  fi
  GEN_ID="$(json_get "$(get_body "${resp}")" "id")"
  if [[ -z "${GEN_ID}" ]]; then
    die "generation id missing"
  fi
  state_set "gen_a_id" "${GEN_ID}"

  log "[03_generation] wait for generation ${GEN_ID}" | tee -a "${LOG_DIR}/03_generation.log"

  start_ts="$(date +%s)"
  last_body=""
  while true; do
    resp=$(request GET "/api/generations/${GEN_ID}" "" "" "${COOKIE_FILE}")
    body="$(get_body "${resp}")"
    last_body="${body}"
    printf '%s' "${body}" > "${GEN_BODY_FILE}"
    st="$(json_get "@${GEN_BODY_FILE}" status)"
    if [[ "${st}" == "success" ]]; then
      break
    fi
    if [[ "${st}" == "error" || "${st}" == "cancelled" ]]; then
      echo "${body}" > "${LOG_DIR}/03_generation_error.json"
      err_msg="$(json_get "@${GEN_BODY_FILE}" error)"
      log "[03_generation] generation failed: status=${st} error=${err_msg}" | tee -a "${LOG_DIR}/03_generation.log"
      if is_retryable_generation_error "${err_msg}" && (( attempt < E2E_GEN_RETRIES )); then
        log "[03_generation] retrying after ${E2E_GEN_RETRY_DELAY}s due to transient error" | tee -a "${LOG_DIR}/03_generation.log"
        sleep "${E2E_GEN_RETRY_DELAY}"
        attempt=$((attempt + 1))
        break
      fi
      die "generation failed with status ${st}"
    fi
    if (( $(date +%s) - start_ts > 5400 )); then
      if [[ -n "${last_body}" ]]; then
        echo "${last_body}" > "${LOG_DIR}/03_generation_timeout.json"
      fi
      die "timeout waiting for generation ${GEN_ID}"
    fi
    sleep 30
  done

  if [[ "${st}" == "success" ]]; then
    break
  fi
done

log "[03_generation] verify domain status" | tee -a "${LOG_DIR}/03_generation.log"
resp=$(request GET "/api/domains/${DOMAIN_ID}" "" "" "${COOKIE_FILE}")
if [[ "$(get_status "${resp}")" != "200" ]]; then
  die "domain fetch failed: $(get_body "${resp}")"
fi
body="$(get_body "${resp}")"
printf '%s' "${body}" > "${DOMAIN_BODY_FILE}"
status_field="$(json_get "@${DOMAIN_BODY_FILE}" status)"
if [[ "${status_field}" != "published" ]]; then
  die "domain status expected published, got ${status_field}"
fi

log "[03_generation] verify files" | tee -a "${LOG_DIR}/03_generation.log"
resp=$(request GET "/api/domains/${DOMAIN_ID}/files" "" "" "${COOKIE_FILE}")
if [[ "$(get_status "${resp}")" != "200" ]]; then
  die "files list failed: $(get_body "${resp}")"
fi
files_body="$(get_body "${resp}")"
printf '%s' "${files_body}" > "${FILES_BODY_FILE}"
file_count="$(json_len "@${FILES_BODY_FILE}")"
if [[ "${file_count}" -le 0 ]]; then
  die "expected published files, got ${file_count}"
fi

log "[03_generation] generation OK" | tee -a "${LOG_DIR}/03_generation.log"
