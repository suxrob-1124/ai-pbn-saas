#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
E2E_DIR="${ROOT_DIR}/tests/e2e"
STATE_FILE="${E2E_DIR}/state.json"
COOKIE_DIR="${E2E_DIR}/.cookies"
LOG_ROOT="${E2E_DIR}/logs"

mkdir -p "${COOKIE_DIR}" "${LOG_ROOT}"

log() {
  local msg="$*"
  printf '[%s] %s\n' "$(date -u '+%Y-%m-%dT%H:%M:%SZ')" "${msg}"
}

die() {
  log "ERROR: $*"
  exit 1
}

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || die "missing command: $1"
}

require_env() {
  local name="$1"
  if [[ -z "${!name:-}" ]]; then
    die "missing env: ${name}"
  fi
}

state_init() {
  if [[ ! -f "${STATE_FILE}" ]]; then
    echo '{}' > "${STATE_FILE}"
  fi
}

state_set() {
  local key="$1"
  local value="$2"
  state_init
  python3 - "${STATE_FILE}" "${key}" "${value}" <<'PY'
import json, sys
path = sys.argv[1]
key = sys.argv[2]
value = sys.argv[3]
with open(path, 'r', encoding='utf-8') as f:
    data = json.load(f)
data[key] = value
with open(path, 'w', encoding='utf-8') as f:
    json.dump(data, f)
PY
}

state_get() {
  local key="$1"
  state_init
  python3 - "${STATE_FILE}" "${key}" <<'PY'
import json, sys
path = sys.argv[1]
key = sys.argv[2]
with open(path, 'r', encoding='utf-8') as f:
    data = json.load(f)
print(data.get(key, ""))
PY
}

ensure_env_key() {
  local file="$1"
  local key="$2"
  local value="$3"
  if grep -q "^${key}=" "${file}"; then
    return 0
  fi
  local backup="${file}.e2e.bak"
  if [[ ! -f "${backup}" ]]; then
    cp "${file}" "${backup}"
  fi
  printf '\n%s=%s\n' "${key}" "${value}" >> "${file}"
  state_set "env_backup" "${backup}"
}

restore_env_backup() {
  local backup
  backup="$(state_get env_backup)"
  if [[ -n "${backup}" && -f "${backup}" ]]; then
    local target="${backup%.e2e.bak}"
    mv "${backup}" "${target}"
    log "restored ${target} from backup"
  fi
}

request() {
  local method="$1"
  local path="$2"
  local data="${3:-}"
  local cookie_out="${4:-}"
  local cookie_in="${5:-}"

  local args=(-sS -w "\n%{http_code}" -H "Content-Type: application/json" -X "${method}")
  if [[ -n "${data}" ]]; then
    args+=(-d "${data}")
  fi
  if [[ -n "${cookie_out}" ]]; then
    args+=(-c "${cookie_out}")
  fi
  if [[ -n "${cookie_in}" ]]; then
    args+=(-b "${cookie_in}")
  fi

  curl "${args[@]}" "${BACKEND_URL}${path}"
}

get_status() {
  echo "${1##*$'\n'}"
}

get_body() {
  echo "${1%$'\n'*}"
}

json_get() {
  local input="$1"
  local key="$2"
  python3 - "${input}" "${key}" <<'PY'
import json, sys
src = sys.argv[1]
key = sys.argv[2]
if src == "-":
    payload = json.load(sys.stdin)
elif src.startswith("@"):
    with open(src[1:], "r", encoding="utf-8") as f:
        payload = json.load(f)
else:
    payload = json.loads(src)
path = key.split('.')
cur = payload
for part in path:
    if isinstance(cur, list):
        idx = int(part)
        cur = cur[idx]
    else:
        cur = cur.get(part)
        if cur is None:
            break
print(cur if cur is not None else "")
PY
}

json_len() {
  local input="$1"
  python3 - "${input}" <<'PY'
import json, sys
src = sys.argv[1]
if src == "-":
    payload = json.load(sys.stdin)
elif src.startswith("@"):
    with open(src[1:], "r", encoding="utf-8") as f:
        payload = json.load(f)
else:
    payload = json.loads(src)
print(len(payload) if isinstance(payload, list) else 0)
PY
}

wait_for_http() {
  local url="$1"
  local timeout="$2"
  local start
  start="$(date +%s)"
  while true; do
    if curl -sS "${url}" >/dev/null 2>&1; then
      return 0
    fi
    if (( $(date +%s) - start > timeout )); then
      return 1
    fi
    sleep 2
  done
}

wait_for_condition() {
  local timeout="$1"
  local interval="$2"
  local description="$3"
  local start
  start="$(date +%s)"
  while true; do
    if eval "$4"; then
      return 0
    fi
    if (( $(date +%s) - start > timeout )); then
      die "timeout waiting for ${description}"
    fi
    sleep "${interval}"
  done
}

login_user() {
  local email="$1"
  local password="$2"
  local cookie="$3"
  local payload
  payload=$(printf '{"email":"%s","password":"%s"}' "${email}" "${password}")
  local resp
  resp=$(request POST "/api/login" "${payload}" "${cookie}")
  if [[ "$(get_status "${resp}")" != "200" ]]; then
    die "login failed: $(get_body "${resp}")"
  fi
}

ensure_login_state() {
  local email
  local password
  local cookie
  email="$(state_get user_email)"
  password="$(state_get user_password)"
  cookie="$(state_get cookie_file)"
  if [[ -z "${email}" || -z "${password}" || -z "${cookie}" ]]; then
    return 0
  fi
  log "[auth] refreshing session"
  login_user "${email}" "${password}" "${cookie}"
}
