#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck disable=SC1091
source "${SCRIPT_DIR}/lib.sh"

BACKEND_URL="$(state_get backend_url)"
COOKIE_FILE="$(state_get cookie_file)"
LOG_DIR="$(state_get log_dir)"
PROJECT_ID="$(state_get project_id)"
DOMAIN_A_ID="$(state_get domain_a_id)"
LINKS_FILE="${LOG_DIR}/05_links_a.json"

ensure_login_state

log "[05_link_schedule] trigger link schedule manually" | tee -a "${LOG_DIR}/05_link_schedule.log"
resp=$(request POST "/api/projects/${PROJECT_ID}/link-schedule/trigger" "" "" "${COOKIE_FILE}")
status="$(get_status "${resp}")"
if [[ "${status}" == "401" ]]; then
  ensure_login_state
  resp=$(request POST "/api/projects/${PROJECT_ID}/link-schedule/trigger" "" "" "${COOKIE_FILE}")
  status="$(get_status "${resp}")"
fi
if [[ "${status}" != "202" ]]; then
  die "link schedule trigger failed: $(get_body "${resp}")"
fi
body="$(get_body "${resp}")"
printf '%s' "${body}" > "${LOG_DIR}/05_link_trigger.json"
created="$(json_get "@${LOG_DIR}/05_link_trigger.json" created)"
updated="$(json_get "@${LOG_DIR}/05_link_trigger.json" updated)"
eligible="$(json_get "@${LOG_DIR}/05_link_trigger.json" eligible)"
if [[ -z "${created}" ]]; then created=0; fi
if [[ -z "${updated}" ]]; then updated=0; fi
if (( created + updated <= 0 )); then
  log "[05_link_schedule] no new tasks created by trigger, verifying existing tasks" | tee -a "${LOG_DIR}/05_link_schedule.log"
  resp=$(request GET "/api/links?project_id=${PROJECT_ID}&limit=50" "" "" "${COOKIE_FILE}")
  status="$(get_status "${resp}")"
  if [[ "${status}" == "401" ]]; then
    ensure_login_state
    resp=$(request GET "/api/links?project_id=${PROJECT_ID}" "" "" "${COOKIE_FILE}")
    status="$(get_status "${resp}")"
  fi
  if [[ "${status}" != "200" ]]; then
    die "link tasks list failed: $(get_body "${resp}")"
  fi
  body="$(get_body "${resp}")"
  printf '%s' "${body}" > "${LOG_DIR}/05_links_project.json"
  existing_count="$(json_len "@${LOG_DIR}/05_links_project.json")"
  if [[ "${existing_count}" -le 0 ]]; then
    log "[05_link_schedule] no project links yet, checking domain A links" | tee -a "${LOG_DIR}/05_link_schedule.log"
    resp=$(request GET "/api/links?domain_id=${DOMAIN_A_ID}&limit=50" "" "" "${COOKIE_FILE}")
    status="$(get_status "${resp}")"
    if [[ "${status}" == "401" ]]; then
      ensure_login_state
      resp=$(request GET "/api/links?domain_id=${DOMAIN_A_ID}&limit=50" "" "" "${COOKIE_FILE}")
      status="$(get_status "${resp}")"
    fi
    if [[ "${status}" != "200" ]]; then
      die "link tasks list failed: $(get_body "${resp}")"
    fi
    body="$(get_body "${resp}")"
    printf '%s' "${body}" > "${LOG_DIR}/05_links_domain_a.json"
    existing_count="$(json_len "@${LOG_DIR}/05_links_domain_a.json")"
  fi
  if [[ "${existing_count}" -le 0 ]]; then
    die "expected created/updated link tasks, got created=${created} updated=${updated} eligible=${eligible}"
  fi
fi

log "[05_link_schedule] manual link run for domain A" | tee -a "${LOG_DIR}/05_link_schedule.log"
resp=$(request POST "/api/domains/${DOMAIN_A_ID}/link/run" "" "" "${COOKIE_FILE}")
status="$(get_status "${resp}")"
if [[ "${status}" == "401" ]]; then
  ensure_login_state
  resp=$(request POST "/api/domains/${DOMAIN_A_ID}/link/run" "" "" "${COOKIE_FILE}")
  status="$(get_status "${resp}")"
fi
if [[ "${status}" != "200" && "${status}" != "201" ]]; then
  die "link run failed: $(get_body "${resp}")"
fi

log "[05_link_schedule] wait for link task processing" | tee -a "${LOG_DIR}/05_link_schedule.log"
start_ts="$(date +%s)"
while true; do
  resp=$(request GET "/api/links?domain_id=${DOMAIN_A_ID}" "" "" "${COOKIE_FILE}")
  status="$(get_status "${resp}")"
  if [[ "${status}" == "401" ]]; then
    ensure_login_state
    sleep 2
    continue
  fi
  if [[ "${status}" != "200" ]]; then
    die "link tasks fetch failed: $(get_body "${resp}")"
  fi
  body="$(get_body "${resp}")"
  printf '%s' "${body}" > "${LINKS_FILE}"
  if [[ "$(json_len "@${LINKS_FILE}")" -gt 0 ]]; then
    status_val="$(json_get "@${LINKS_FILE}" "0.status")"
    if [[ "${status_val}" != "pending" && "${status_val}" != "searching" && "${status_val}" != "removing" ]]; then
      break
    fi
  fi
  if (( $(date +%s) - start_ts > 3600 )); then
    die "timeout waiting for link task to complete"
  fi
  sleep 20
 done

log "[05_link_schedule] link schedule checks OK" | tee -a "${LOG_DIR}/05_link_schedule.log"
