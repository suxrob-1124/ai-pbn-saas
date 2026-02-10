#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck disable=SC1091
source "${SCRIPT_DIR}/lib.sh"

RUN_ID="$(state_get run_id)"
BACKEND_URL="$(state_get backend_url)"
COOKIE_FILE="$(state_get cookie_file)"
LOG_DIR="$(state_get log_dir)"
EXISTING_PROJECT_ID="$(state_get project_id)"
EXISTING_DOMAIN_A_ID="$(state_get domain_a_id)"
EXISTING_DOMAIN_B_ID="$(state_get domain_b_id)"
EXISTING_LINK_SCHEDULE_ID="$(state_get link_schedule_id)"

E2E_EMAIL="${E2E_EMAIL:-e2e-${RUN_ID}@example.com}"
E2E_PASSWORD="${E2E_PASSWORD:-E2ePassw0rd!!}"
E2E_DATASET="${E2E_DATASET:-surstrem}"

SURSTREM_PROJECT_NAME="surstrem"
SURSTREM_COUNTRY="se"
SURSTREM_LANG="sv"
SURSTREM_DOMAINS=(
  "profitnesscamps.se||Insättning och uttag på utländska casinon: kort, e-plånböcker, banköverföring och krypto"
  "elinloe.se||Snabba uttag hos utländska casinon: vad påverkar utbetalningstiderna?"
  "kundservice.net||Bonuserbjudanden på utländska casinon: hur du läser omsättningskrav och maxuttag"
)

XBET_PROJECT_NAME="1xbet-ru"
XBET_COUNTRY="ru"
XBET_LANG="ru"
XBET_DOMAINS=(
  "скважина61.рф||бонус за регистрацию 1xBet"
  "dialog-c.ru||1хБет вывод средств"
  "autogornostay.ru||регистрация в 1хБет"
)

PROJECT_NAME="${PROJECT_NAME:-}"
PROJECT_COUNTRY="${PROJECT_COUNTRY:-}"
PROJECT_LANG="${PROJECT_LANG:-}"

login_or_register() {
  local email="$1"
  local password="$2"
  local login_payload
  login_payload=$(printf '{"email":"%s","password":"%s"}' "${email}" "${password}")

  local resp
  resp=$(request POST "/api/login" "${login_payload}" "${COOKIE_FILE}")
  if [[ "$(get_status "${resp}")" == "200" ]]; then
    return 0
  fi

  local reg_payload
  reg_payload=$(printf '{"email":"%s","password":"%s"}' "${email}" "${password}")
  resp=$(request POST "/api/register" "${reg_payload}")
  local reg_status
  reg_status="$(get_status "${resp}")"
  if [[ "${reg_status}" != "201" && "${reg_status}" != "400" ]]; then
    die "register failed: $(get_body "${resp}")"
  fi

  resp=$(request POST "/api/login" "${login_payload}" "${COOKIE_FILE}")
  if [[ "$(get_status "${resp}")" != "200" ]]; then
    die "login failed: $(get_body "${resp}")"
  fi
}

log "[02_seed] login/register" | tee -a "${LOG_DIR}/02_seed.log"
login_or_register "${E2E_EMAIL}" "${E2E_PASSWORD}"

if [[ "${E2E_RESUME:-}" == "true" ]]; then
  if [[ -n "${EXISTING_PROJECT_ID}" && -n "${EXISTING_DOMAIN_A_ID}" && -n "${EXISTING_DOMAIN_B_ID}" && -n "${EXISTING_LINK_SCHEDULE_ID}" ]]; then
    log "[02_seed] resume: using existing project/domains" | tee -a "${LOG_DIR}/02_seed.log"
    exit 0
  fi
fi

log "[02_seed] save Gemini API key" | tee -a "${LOG_DIR}/02_seed.log"
E2E_APIKEY_RETRIES="${E2E_APIKEY_RETRIES:-3}"
E2E_APIKEY_RETRY_DELAY="${E2E_APIKEY_RETRY_DELAY:-10}"
api_payload=$(printf '{"apiKey":"%s"}' "${GEMINI_API_KEY}")
attempt=1
while true; do
  resp=$(request POST "/api/profile/api-key" "${api_payload}" "" "${COOKIE_FILE}")
  status="$(get_status "${resp}")"
  body="$(get_body "${resp}")"
  if [[ "${status}" == "200" ]]; then
    break
  fi
  if echo "${body}" | rg -qi "unexpected EOF|timeout|deadline exceeded|connection reset"; then
    if (( attempt < E2E_APIKEY_RETRIES )); then
      log "[02_seed] api key save transient error, retrying in ${E2E_APIKEY_RETRY_DELAY}s" | tee -a "${LOG_DIR}/02_seed.log"
      sleep "${E2E_APIKEY_RETRY_DELAY}"
      attempt=$((attempt + 1))
      continue
    fi
  fi
  die "api key save failed: ${body}"
done

pick_dataset() {
  local dataset="$1"
  case "${dataset}" in
    surstrem)
      echo "${SURSTREM_PROJECT_NAME}|${SURSTREM_COUNTRY}|${SURSTREM_LANG}"
      return 0
      ;;
    xbet)
      echo "${XBET_PROJECT_NAME}|${XBET_COUNTRY}|${XBET_LANG}"
      return 0
      ;;
    *)
      return 1
      ;;
  esac
}

log "[02_seed] dataset: ${E2E_DATASET}" | tee -a "${LOG_DIR}/02_seed.log"
if [[ -z "${PROJECT_NAME}" || -z "${PROJECT_COUNTRY}" || -z "${PROJECT_LANG}" ]]; then
  if dataset_info="$(pick_dataset "${E2E_DATASET}")"; then
    PROJECT_NAME="${PROJECT_NAME:-$(echo "${dataset_info}" | cut -d'|' -f1)}"
    PROJECT_COUNTRY="${PROJECT_COUNTRY:-$(echo "${dataset_info}" | cut -d'|' -f2)}"
    PROJECT_LANG="${PROJECT_LANG:-$(echo "${dataset_info}" | cut -d'|' -f3)}"
  fi
fi

PROJECT_NAME="${PROJECT_NAME:-e2e-project-${RUN_ID}}"
PROJECT_COUNTRY="${PROJECT_COUNTRY:-us}"
PROJECT_LANG="${PROJECT_LANG:-en}"

log "[02_seed] create project (${PROJECT_NAME}/${PROJECT_COUNTRY}/${PROJECT_LANG})" | tee -a "${LOG_DIR}/02_seed.log"
project_payload=$(printf '{"name":"%s","country":"%s","language":"%s","status":"draft"}' "${PROJECT_NAME}" "${PROJECT_COUNTRY}" "${PROJECT_LANG}")
resp=$(request POST "/api/projects" "${project_payload}" "" "${COOKIE_FILE}")
if [[ "$(get_status "${resp}")" != "201" ]]; then
  die "project create failed: $(get_body "${resp}")"
fi
project_id="$(json_get "$(get_body "${resp}")" "id")"
if [[ -z "${project_id}" ]]; then
  die "project id missing"
fi

state_set "user_email" "${E2E_EMAIL}"
state_set "user_password" "${E2E_PASSWORD}"
state_set "project_id" "${project_id}"

log "[02_seed] create link schedule (delay_minutes=1)" | tee -a "${LOG_DIR}/02_seed.log"
link_payload='{"name":"E2E Link Schedule","config":{"interval":"1m","limit":2,"delay_minutes":1},"isActive":true,"timezone":"UTC"}'
resp=$(request PUT "/api/projects/${project_id}/link-schedule" "${link_payload}" "" "${COOKIE_FILE}")
if [[ "$(get_status "${resp}")" != "200" ]]; then
  die "link schedule upsert failed: $(get_body "${resp}")"
fi
link_schedule_id="$(json_get "$(get_body "${resp}")" "id")"
state_set "link_schedule_id" "${link_schedule_id}"

log "[02_seed] create domains" | tee -a "${LOG_DIR}/02_seed.log"
DOMAIN_A=""
DOMAIN_B=""
KEYWORD_A=""
KEYWORD_B=""

if [[ "${E2E_DATASET}" == "surstrem" ]]; then
  DOMAIN_A="${SURSTREM_DOMAINS[0]%%||*}"
  KEYWORD_A="${SURSTREM_DOMAINS[0]#*||}"
  DOMAIN_B="${SURSTREM_DOMAINS[1]%%||*}"
  KEYWORD_B="${SURSTREM_DOMAINS[1]#*||}"
elif [[ "${E2E_DATASET}" == "xbet" ]]; then
  DOMAIN_A="${XBET_DOMAINS[0]%%||*}"
  KEYWORD_A="${XBET_DOMAINS[0]#*||}"
  DOMAIN_B="${XBET_DOMAINS[1]%%||*}"
  KEYWORD_B="${XBET_DOMAINS[1]#*||}"
else
  DOMAIN_A="e2e-${RUN_ID}-a.example.com"
  DOMAIN_B="e2e-${RUN_ID}-b.example.com"
  KEYWORD_A="E2E keyword A"
  KEYWORD_B="E2E keyword B"
fi

E2E_DOMAIN_PREFIX="${E2E_DOMAIN_PREFIX:-e2e-${RUN_ID}}"
if [[ -n "${E2E_DOMAIN_PREFIX}" ]]; then
  DOMAIN_A="${E2E_DOMAIN_PREFIX}.${DOMAIN_A}"
  DOMAIN_B="${E2E_DOMAIN_PREFIX}.${DOMAIN_B}"
fi

create_domain() {
  local url="$1"
  local keyword="$2"
  local payload
  payload=$(printf '{"url":"%s","keyword":"%s"}' "${url}" "${keyword}")
  local resp
  resp=$(request POST "/api/projects/${project_id}/domains" "${payload}" "" "${COOKIE_FILE}") || {
    log "[02_seed] curl failed for domain ${url}" | tee -a "${LOG_DIR}/02_seed.log"
    return 1
  }
  if [[ "$(get_status "${resp}")" != "201" ]]; then
    log "[02_seed] domain create failed (${url}): $(get_body "${resp}")" | tee -a "${LOG_DIR}/02_seed.log"
    die "domain create failed"
  fi
  json_get "$(get_body "${resp}")" "id"
}

domain_a_id="$(create_domain "${DOMAIN_A}" "${KEYWORD_A}")"
domain_b_id="$(create_domain "${DOMAIN_B}" "${KEYWORD_B}")"

state_set "domain_a_id" "${domain_a_id}"
state_set "domain_b_id" "${domain_b_id}"
state_set "domain_a_url" "${DOMAIN_A}"
state_set "domain_b_url" "${DOMAIN_B}"

log "[02_seed] set link settings" | tee -a "${LOG_DIR}/02_seed.log"
set_link() {
  local domain_id="$1"
  local anchor="$2"
  local target="$3"
  local payload
  payload=$(printf '{"link_anchor_text":"%s","link_acceptor_url":"%s"}' "${anchor}" "${target}")
  local resp
  resp=$(request PATCH "/api/domains/${domain_id}" "${payload}" "" "${COOKIE_FILE}")
  if [[ "$(get_status "${resp}")" != "200" ]]; then
    die "link settings failed: $(get_body "${resp}")"
  fi
}

set_link "${domain_a_id}" "E2E Anchor A" "https://target.example/${RUN_ID}/a"
set_link "${domain_b_id}" "E2E Anchor B" "https://target.example/${RUN_ID}/b"

log "[02_seed] seed complete" | tee -a "${LOG_DIR}/02_seed.log"
