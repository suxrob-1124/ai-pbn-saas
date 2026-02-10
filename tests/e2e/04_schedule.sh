#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck disable=SC1091
source "${SCRIPT_DIR}/lib.sh"

BACKEND_URL="$(state_get backend_url)"
COOKIE_FILE="$(state_get cookie_file)"
LOG_DIR="$(state_get log_dir)"
PROJECT_ID="$(state_get project_id)"
DOMAIN_B_ID="$(state_get domain_b_id)"
GEN_LIST_FILE="${LOG_DIR}/04_generations_b.json"
GEN_B_FILE="${LOG_DIR}/04_generation_b.json"
LINKS_FILE="${LOG_DIR}/04_links_b.json"

ensure_login_state

log "[04_schedule] create generation schedule" | tee -a "${LOG_DIR}/04_schedule.log"

gen_payload='{"name":"E2E Gen Schedule","strategy":"custom","config":{"interval":"1m","limit":1},"isActive":true,"timezone":"UTC"}'
resp=$(request POST "/api/projects/${PROJECT_ID}/schedules" "${gen_payload}" "" "${COOKIE_FILE}")
if [[ "$(get_status "${resp}")" != "201" ]]; then
  die "generation schedule create failed: $(get_body "${resp}")"
fi
schedule_id="$(json_get "$(get_body "${resp}")" "id")"
state_set "gen_schedule_id" "${schedule_id}"

log "[04_schedule] wait for scheduled generation on domain B" | tee -a "${LOG_DIR}/04_schedule.log"
start_ts="$(date +%s)"
while true; do
  resp=$(request GET "/api/domains/${DOMAIN_B_ID}/generations" "" "" "${COOKIE_FILE}")
  status="$(get_status "${resp}")"
  if [[ "${status}" == "401" ]]; then
    ensure_login_state
    sleep 2
    continue
  fi
  if [[ "${status}" != "200" ]]; then
    die "failed to list generations for domain B: $(get_body "${resp}")"
  fi
  body="$(get_body "${resp}")"
  printf '%s' "${body}" > "${GEN_LIST_FILE}"
  if [[ "$(json_len "@${GEN_LIST_FILE}")" -gt 0 ]]; then
    gen_id="$(json_get "@${GEN_LIST_FILE}" "0.id")"
    if [[ -n "${gen_id}" ]]; then
      state_set "gen_b_id" "${gen_id}"
      break
    fi
  fi
  if (( $(date +%s) - start_ts > 7200 )); then
    die "timeout waiting for scheduled generation on domain B"
  fi
  sleep 30
 done

log "[04_schedule] wait for domain B generation success" | tee -a "${LOG_DIR}/04_schedule.log"
GEN_B_ID="$(state_get gen_b_id)"
start_ts="$(date +%s)"
while true; do
  resp=$(request GET "/api/generations/${GEN_B_ID}" "" "" "${COOKIE_FILE}")
  status="$(get_status "${resp}")"
  if [[ "${status}" == "401" ]]; then
    ensure_login_state
    sleep 2
    continue
  fi
  body="$(get_body "${resp}")"
  printf '%s' "${body}" > "${GEN_B_FILE}"
  st="$(json_get "@${GEN_B_FILE}" status)"
  if [[ "${st}" == "success" ]]; then
    break
  fi
  if [[ "${st}" == "error" || "${st}" == "cancelled" ]]; then
    die "domain B generation failed with status ${st}"
  fi
  if (( $(date +%s) - start_ts > 7200 )); then
    die "timeout waiting for domain B generation success"
  fi
  sleep 30
 done

log "[04_schedule] wait for link tasks after delay" | tee -a "${LOG_DIR}/04_schedule.log"
start_ts="$(date +%s)"
while true; do
  resp=$(request GET "/api/links?domain_id=${DOMAIN_B_ID}" "" "" "${COOKIE_FILE}")
  status="$(get_status "${resp}")"
  if [[ "${status}" == "401" ]]; then
    ensure_login_state
    sleep 2
    continue
  fi
  if [[ "${status}" != "200" ]]; then
    die "failed to list link tasks: $(get_body "${resp}")"
  fi
  body="$(get_body "${resp}")"
  printf '%s' "${body}" > "${LINKS_FILE}"
  if [[ "$(json_len "@${LINKS_FILE}")" -gt 0 ]]; then
    break
  fi
  if (( $(date +%s) - start_ts > 3600 )); then
    die "timeout waiting for link tasks for domain B"
  fi
  sleep 20
 done

log "[04_schedule] schedule checks OK" | tee -a "${LOG_DIR}/04_schedule.log"
