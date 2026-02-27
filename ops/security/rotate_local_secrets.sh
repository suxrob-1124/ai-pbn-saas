#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
ENV_FILE="${1:-${ROOT_DIR}/.env}"

if [[ ! -f "${ENV_FILE}" ]]; then
  echo "[rotate] env file not found: ${ENV_FILE}" >&2
  exit 1
fi

rand() {
  local n="${1:-48}"
  openssl rand -base64 96 | tr -d '\n' | tr '+/' '-_' | cut -c1-"${n}"
}

JWT_SECRET_NEW="$(rand 64)"
CAPTCHA_SECRET_NEW="$(rand 48)"
API_KEY_SECRET_NEW="$(rand 64)"
GF_SECURITY_ADMIN_PASSWORD_NEW="$(rand 32)"

python3 - <<'PY' "${ENV_FILE}" "${JWT_SECRET_NEW}" "${CAPTCHA_SECRET_NEW}" "${API_KEY_SECRET_NEW}" "${GF_SECURITY_ADMIN_PASSWORD_NEW}"
from pathlib import Path
import re
import sys

path = Path(sys.argv[1])
jwt = sys.argv[2]
captcha = sys.argv[3]
api = sys.argv[4]
graf = sys.argv[5]

text = path.read_text(encoding='utf-8')
repl = {
    'JWT_SECRET': jwt,
    'CAPTCHA_SECRET': captcha,
    'API_KEY_SECRET': api,
    'GF_SECURITY_ADMIN_PASSWORD': graf,
}
for key, value in repl.items():
    pat = re.compile(rf'^(\s*{re.escape(key)}\s*=).*$' , re.MULTILINE)
    if pat.search(text):
        text = pat.sub(rf'\1{value}', text)
    else:
        text += f"\n{key}={value}\n"
path.write_text(text, encoding='utf-8')
PY

echo "[rotate] rotated keys in ${ENV_FILE}: JWT_SECRET, CAPTCHA_SECRET, API_KEY_SECRET, GF_SECURITY_ADMIN_PASSWORD"
echo "[rotate] note: rotate GEMINI_API_KEY and SMTP_PASSWORD at provider side manually if leaked"
