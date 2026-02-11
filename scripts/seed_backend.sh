#!/usr/bin/env bash
set -euo pipefail

API_URL="${API_URL:-http://localhost:8080}"
ADMIN_EMAIL="${ADMIN_EMAIL:-admin@example.com}"
ADMIN_PASSWORD="${ADMIN_PASSWORD:-Admin123!!}"
MANAGER_EMAIL="${MANAGER_EMAIL:-manager@example.com}"
MANAGER_PASSWORD="${MANAGER_PASSWORD:-Manager123!!}"
MANAGER2_EMAIL="${MANAGER2_EMAIL:-manager2@example.com}"
MANAGER2_PASSWORD="${MANAGER2_PASSWORD:-Manager123!!}"
USER_EMAIL="${USER_EMAIL:-user@example.com}"
USER_PASSWORD="${USER_PASSWORD:-User123!!!}"

SURSTREM_PROJECT_NAME="${SURSTREM_PROJECT_NAME:-surstrem}"
SURSTREM_COUNTRY="${SURSTREM_COUNTRY:-se}"
SURSTREM_LANG="${SURSTREM_LANG:-sv}"
SURSTREM_DOMAINS=(
  "profitnesscamps.se||Insättning och uttag på utländska casinon: kort, e-plånböcker, banköverföring och krypto"
  "elinloe.se||Snabba uttag hos utländska casinon: vad påverkar utbetalningstiderna?"
  "kundservice.net||Bonuserbjudanden på utländska casinon: hur du läser omsättningskrav och maxuttag"
)

XBET_PROJECT_NAME="${XBET_PROJECT_NAME:-1xbet-ru}"
XBET_COUNTRY="${XBET_COUNTRY:-ru}"
XBET_LANG="${XBET_LANG:-ru}"
XBET_DOMAINS=(
  "скважина61.рф||бонус за регистрацию 1xBet"
  "dialog-c.ru||1хБет вывод средств"
  "autogornostay.ru||регистрация в 1хБет"
)

WORK_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TMP_DIR="${TMP_DIR:-/tmp}"
ADMIN_COOKIE="${TMP_DIR}/obz_seed_admin.cookie"
MANAGER_COOKIE="${TMP_DIR}/obz_seed_manager.cookie"
MANAGER2_COOKIE="${TMP_DIR}/obz_seed_manager2.cookie"
ENV_FILE="${ENV_FILE:-${WORK_DIR}/../.env}"

log() {
  echo "[seed] $*"
}

wait_for_backend() {
  local attempts=60
  local i=0
  while [[ $i -lt $attempts ]]; do
    if curl -sS "${API_URL}/healthz" >/dev/null 2>&1; then
      return 0
    fi
    i=$((i + 1))
    sleep 2
  done
  return 1
}

request() {
  local method="$1"
  local path="$2"
  local data="${3:-}"
  local cookie_out="${4:-}"
  local cookie_in="${5:-}"

  local args=(-sS -w "\n%{http_code}" -H "Content-Type: application/json" -X "$method")
  if [[ -n "$data" ]]; then
    args+=(-d "$data")
  fi
  if [[ -n "$cookie_out" ]]; then
    args+=(-c "$cookie_out")
  fi
  if [[ -n "$cookie_in" ]]; then
    args+=(-b "$cookie_in")
  fi

  curl "${args[@]}" "${API_URL}${path}"
}

get_status() {
  echo "${1##*$'\n'}"
}

get_body() {
  echo "${1%$'\n'*}"
}

load_gemini_key() {
  if [[ -n "${GEMINI_API_KEY:-}" ]]; then
    return 0
  fi
  if [[ ! -f "$ENV_FILE" ]]; then
    return 0
  fi
  local line value
  line="$(grep -E '^[[:space:]]*GEMINI_API_KEY=' "$ENV_FILE" | tail -n 1 || true)"
  if [[ -z "$line" ]]; then
    return 0
  fi
  value="${line#*=}"
  value="${value%$'\r'}"
  if [[ "$value" == \"*\" && "$value" == *\" ]]; then
    value="${value:1:${#value}-2}"
  elif [[ "$value" == \'*\' && "$value" == *\' ]]; then
    value="${value:1:${#value}-2}"
  fi
  if [[ -n "$value" ]]; then
    GEMINI_API_KEY="$value"
  fi
}

ensure_login() {
  local email="$1"
  local password="$2"
  local cookie="$3"

  local login_payload
  login_payload=$(printf '{"email":"%s","password":"%s"}' "$email" "$password")

  local resp
  resp=$(request POST "/api/login" "$login_payload" "$cookie")
  if [[ "$(get_status "$resp")" == "200" ]]; then
    return 0
  fi

  local reg_payload
  reg_payload=$(printf '{"email":"%s","password":"%s"}' "$email" "$password")
  resp=$(request POST "/api/register" "$reg_payload")
  local reg_status
  reg_status="$(get_status "$resp")"
  if [[ "$reg_status" != "201" && "$reg_status" != "400" ]]; then
    log "register failed for ${email}: $(get_body "$resp")"
    return 1
  fi

  resp=$(request POST "/api/login" "$login_payload" "$cookie")
  if [[ "$(get_status "$resp")" == "200" ]]; then
    return 0
  fi

  if [[ -f "$ADMIN_COOKIE" && "$email" != "$ADMIN_EMAIL" ]]; then
    local reset_payload
    reset_payload=$(printf '{"newPassword":"%s"}' "$password")
    local reset_resp
    reset_resp=$(request POST "/api/admin/users/${email}/password" "$reset_payload" "" "$ADMIN_COOKIE")
    if [[ "$(get_status "$reset_resp")" != "200" ]]; then
      log "admin password reset failed for ${email}: $(get_body "$reset_resp")"
      return 1
    fi
    resp=$(request POST "/api/login" "$login_payload" "$cookie")
    if [[ "$(get_status "$resp")" == "200" ]]; then
      return 0
    fi
  fi

  log "login failed for ${email}: $(get_body "$resp")"
  return 1
}

save_api_key() {
  local cookie="$1"
  if [[ -z "${GEMINI_API_KEY:-}" ]]; then
    return 0
  fi
  local payload
  payload=$(printf '{"apiKey":"%s"}' "$GEMINI_API_KEY")
  local resp
  resp=$(request POST "/api/profile/api-key" "$payload" "" "$cookie")
  if [[ "$(get_status "$resp")" != "200" ]]; then
    log "api key save failed: $(get_body "$resp")"
    return 1
  fi
}

approve_user() {
  local email="$1"
  local role="$2"
  local payload
  if [[ -n "$role" ]]; then
    payload=$(printf '{"role":"%s","isApproved":true}' "$role")
  else
    payload='{"isApproved":true}'
  fi
  local resp
  resp=$(request PATCH "/api/admin/users/${email}" "$payload" "" "$ADMIN_COOKIE")
  if [[ "$(get_status "$resp")" != "200" ]]; then
    log "admin approve failed for ${email}: $(get_body "$resp")"
    return 1
  fi
}

find_project_id() {
  local projects_json="$1"
  local name="$2"
  python3 - "$projects_json" "$name" <<'PY'
import json, sys
data = json.loads(sys.argv[1])
name = sys.argv[2]
for p in data:
  if p.get("name") == name:
    print(p.get("id", ""))
    break
PY
}

create_project() {
  local cookie="$1"
  local name="$2"
  local country="$3"
  local lang="$4"

  local resp
  resp=$(request GET "/api/projects" "" "" "$cookie")
  if [[ "$(get_status "$resp")" != "200" ]]; then
    log "failed to list projects: $(get_body "$resp")"
    return 1
  fi
  local body
  body="$(get_body "$resp")"
  local id
  id="$(find_project_id "$body" "$name")"
  if [[ -n "$id" ]]; then
    echo "$id"
    return 0
  fi

  local payload
  payload=$(printf '{"name":"%s","country":"%s","language":"%s","status":"draft"}' "$name" "$country" "$lang")
  resp=$(request POST "/api/projects" "$payload" "" "$cookie")
  if [[ "$(get_status "$resp")" != "201" ]]; then
    log "failed to create project ${name}: $(get_body "$resp")"
    return 1
  fi
  body="$(get_body "$resp")"
  python3 - "$body" <<'PY'
import json, sys
print(json.loads(sys.argv[1]).get("id", ""))
PY
}

import_domains() {
  local cookie="$1"
  local project_id="$2"
  shift 2
  local items=()
  for entry in "$@"; do
    local url="$entry"
    local keyword=""
    if [[ "$entry" == *"||"* ]]; then
      url="${entry%%||*}"
      keyword="${entry#*||}"
    fi
    if [[ -n "$keyword" ]]; then
      items+=("{\"url\":\"${url}\",\"keyword\":\"${keyword}\"}")
    else
      items+=("{\"url\":\"${url}\"}")
    fi
  done
  local payload
  payload=$(printf '{"items":[%s]}' "$(IFS=,; echo "${items[*]}")")
  local resp
  resp=$(request POST "/api/projects/${project_id}/domains/import" "$payload" "" "$cookie")
  if [[ "$(get_status "$resp")" != "201" ]]; then
    log "failed to import domains for ${project_id}: $(get_body "$resp")"
    return 1
  fi
}

main() {
  log "waiting for backend at ${API_URL}"
  if ! wait_for_backend; then
    log "backend not ready"
    exit 1
  fi

  log "ensure admin user"
  ensure_login "$ADMIN_EMAIL" "$ADMIN_PASSWORD" "$ADMIN_COOKIE"

  load_gemini_key

  log "ensure manager users"
  ensure_login "$MANAGER_EMAIL" "$MANAGER_PASSWORD" "$MANAGER_COOKIE"
  ensure_login "$MANAGER2_EMAIL" "$MANAGER2_PASSWORD" "$MANAGER2_COOKIE"
  ensure_login "$USER_EMAIL" "$USER_PASSWORD" "${TMP_DIR}/obz_seed_user.cookie"

  log "approve users via admin"
  approve_user "$ADMIN_EMAIL" ""
  approve_user "$MANAGER_EMAIL" "manager"
  approve_user "$MANAGER2_EMAIL" "manager"
  approve_user "$USER_EMAIL" "manager"

  if [[ -n "${GEMINI_API_KEY:-}" ]]; then
    log "save Gemini API key for users"
    save_api_key "$ADMIN_COOKIE"
    save_api_key "$MANAGER_COOKIE"
    save_api_key "$MANAGER2_COOKIE"
    save_api_key "${TMP_DIR}/obz_seed_user.cookie"
  else
    log "GEMINI_API_KEY is empty, skipping api key setup"
  fi

  log "create projects"
  local surstrem_id
  surstrem_id="$(create_project "$MANAGER_COOKIE" "$SURSTREM_PROJECT_NAME" "$SURSTREM_COUNTRY" "$SURSTREM_LANG")"
  local xbet_id
  xbet_id="$(create_project "$MANAGER2_COOKIE" "$XBET_PROJECT_NAME" "$XBET_COUNTRY" "$XBET_LANG")"

  log "import domains"
  import_domains "$MANAGER_COOKIE" "$surstrem_id" "${SURSTREM_DOMAINS[@]}"
  import_domains "$MANAGER2_COOKIE" "$xbet_id" "${XBET_DOMAINS[@]}"

  log "done"
}

main "$@"
