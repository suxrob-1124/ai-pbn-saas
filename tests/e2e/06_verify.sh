#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck disable=SC1091
source "${SCRIPT_DIR}/lib.sh"

BACKEND_URL="$(state_get backend_url)"
COOKIE_FILE="$(state_get cookie_file)"
LOG_DIR="$(state_get log_dir)"
PROJECT_ID="$(state_get project_id)"
SUMMARY_FILE="${LOG_DIR}/06_summary.json"
LINKS_FILE="${LOG_DIR}/06_links.json"

ensure_login_state

log "[06_verify] project summary" | tee -a "${LOG_DIR}/06_verify.log"
resp=$(request GET "/api/projects/${PROJECT_ID}/summary" "" "" "${COOKIE_FILE}")
status="$(get_status "${resp}")"
if [[ "${status}" == "401" ]]; then
  ensure_login_state
  resp=$(request GET "/api/projects/${PROJECT_ID}/summary" "" "" "${COOKIE_FILE}")
  status="$(get_status "${resp}")"
fi
if [[ "${status}" != "200" ]]; then
  die "project summary failed: $(get_body "${resp}")"
fi
body="$(get_body "${resp}")"
printf '%s' "${body}" > "${SUMMARY_FILE}"
domains_json="$(json_get "@${SUMMARY_FILE}" domains)"
if [[ -z "${domains_json}" ]]; then
  die "expected domains in project summary"
fi

log "[06_verify] link tasks list" | tee -a "${LOG_DIR}/06_verify.log"
resp=$(request GET "/api/links?project_id=${PROJECT_ID}" "" "" "${COOKIE_FILE}")
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
printf '%s' "${body}" > "${LINKS_FILE}"
count="$(json_len "@${LINKS_FILE}")"
if [[ "${count}" -le 0 ]]; then
  die "expected link tasks, got ${count}"
fi

log "[06_verify] verification OK" | tee -a "${LOG_DIR}/06_verify.log"
