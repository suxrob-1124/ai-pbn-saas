#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'EOF'
Usage:
  scripts/check_no_secrets.sh --staged
  scripts/check_no_secrets.sh --range <from> <to>
  scripts/check_no_secrets.sh --diff-file <path>

Checks added lines in git diff and blocks obvious secrets.
EOF
}

MODE="staged"
RANGE_FROM=""
RANGE_TO=""
DIFF_FILE=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    --staged)
      MODE="staged"
      shift
      ;;
    --range)
      MODE="range"
      RANGE_FROM="${2:-}"
      RANGE_TO="${3:-}"
      if [[ -z "$RANGE_FROM" || -z "$RANGE_TO" ]]; then
        usage
        exit 2
      fi
      shift 3
      ;;
    --diff-file)
      MODE="diff-file"
      DIFF_FILE="${2:-}"
      if [[ -z "$DIFF_FILE" ]]; then
        usage
        exit 2
      fi
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "Unknown argument: $1" >&2
      usage
      exit 2
      ;;
  esac
done

ROOT="$(git rev-parse --show-toplevel 2>/dev/null || pwd)"
cd "$ROOT"

collect_diff() {
  case "$MODE" in
    staged)
      git diff --cached --unified=0 --no-color --no-ext-diff
      ;;
    range)
      git diff --unified=0 --no-color --no-ext-diff "$RANGE_FROM" "$RANGE_TO"
      ;;
    diff-file)
      cat "$DIFF_FILE"
      ;;
    *)
      echo "unsupported mode: $MODE" >&2
      return 2
      ;;
  esac
}

collect_paths() {
  case "$MODE" in
    staged)
      git diff --cached --name-only --diff-filter=ACMR
      ;;
    range)
      git diff --name-only --diff-filter=ACMR "$RANGE_FROM" "$RANGE_TO"
      ;;
    diff-file)
      grep -E '^\+\+\+ b/' "$DIFF_FILE" | sed -E 's#^\+\+\+ b/##' | sort -u
      ;;
  esac
}

is_allowed_env_file() {
  local path="$1"
  local base
  base="$(basename "$path")"
  [[ "$base" == ".env.example" || "$base" == ".env.sample" || "$base" == ".env.template" ]]
}

is_prohibited_path() {
  local path="$1"
  local base
  base="$(basename "$path")"

  if [[ "$base" == ".env" || "$base" == .env.* ]]; then
    if ! is_allowed_env_file "$path"; then
      return 0
    fi
  fi

  if [[ "$base" =~ ^id_(rsa|dsa|ecdsa|ed25519)$ ]]; then
    return 0
  fi

  if [[ "$path" =~ \.(pem|p12|pfx|jks|keystore|key)$ && ! "$path" =~ \.pub$ ]]; then
    return 0
  fi

  return 1
}

is_placeholder_line() {
  local line="$1"
  printf '%s' "$line" | grep -Eqi -- '(example|localhost|127\.0\.0\.1|changeme|your[_-]?|dummy|sample|test|admin@example\.com|manager@example\.com|user@example\.com|admin123!!|manager123!!|user123!!!)'
}

matches_secret_pattern() {
  local line="$1"
  if printf '%s' "$line" | grep -Eq -- '-----BEGIN ([A-Z ]+)?PRIVATE KEY-----'; then
    return 0
  fi
  if printf '%s' "$line" | grep -Eq -- 'AKIA[0-9A-Z]{16}'; then
    return 0
  fi
  if printf '%s' "$line" | grep -Eq -- 'ghp_[A-Za-z0-9]{36}'; then
    return 0
  fi
  if printf '%s' "$line" | grep -Eq -- 'github_pat_[A-Za-z0-9_]{40,}'; then
    return 0
  fi
  if printf '%s' "$line" | grep -Eq -- 'xox[baprs]-[A-Za-z0-9-]{10,}'; then
    return 0
  fi
  if printf '%s' "$line" | grep -Eq -- 'sk-[A-Za-z0-9]{20,}'; then
    return 0
  fi
  if printf '%s' "$line" | grep -Eqi -- 'bearer[[:space:]]+[A-Za-z0-9._=-]{20,}'; then
    return 0
  fi
  if printf '%s' "$line" | grep -Eqi -- '(^|[^A-Za-z])(api[_-]?key|access[_-]?token|refresh[_-]?token|jwt[_-]?secret|secret|password|passwd)[[:space:]]*[:=][[:space:]]*["'\'']?[A-Za-z0-9+/=_-]{16,}'; then
    return 0
  fi
  return 1
}

extract_added_entries() {
  local diff_input="$1"
  awk '
    BEGIN { file=""; line=0 }
    /^\+\+\+ b\// {
      file=$0
      sub(/^\+\+\+ b\//, "", file)
      next
    }
    /^@@ / {
      if (match($0, /\+[0-9]+/)) {
        start = substr($0, RSTART+1, RLENGTH-1)
        line = start - 1
      }
      next
    }
    /^\+/ && $0 !~ /^\+\+\+/ {
      line++
      content = substr($0, 2)
      printf "%s\t%d\t%s\n", file, line, content
      next
    }
    /^ / {
      line++
      next
    }
    /^-/ { next }
  ' <<< "$diff_input"
}

DIFF_CONTENT="$(collect_diff || true)"

if [[ -z "$DIFF_CONTENT" ]]; then
  exit 0
fi

had_error=0

while IFS= read -r path; do
  [[ -z "$path" ]] && continue
  if is_prohibited_path "$path"; then
    echo "SECRET-CHECK: запрещено коммитить потенциально секретный файл: $path" >&2
    had_error=1
  fi
done < <(collect_paths || true)

while IFS=$'\t' read -r file line text; do
  [[ -z "$text" ]] && continue
  if is_placeholder_line "$text"; then
    continue
  fi
  if matches_secret_pattern "$text"; then
    echo "SECRET-CHECK: найден потенциальный секрет в $file:$line" >&2
    echo "  > $text" >&2
    had_error=1
  fi
done < <(extract_added_entries "$DIFF_CONTENT")

if [[ "$had_error" -ne 0 ]]; then
  cat >&2 <<'EOF'

Коммит/пуш остановлен: обнаружены потенциальные секреты.
Если это ложное срабатывание, замените значение на placeholder или скорректируйте проверку в scripts/check_no_secrets.sh.
EOF
  exit 1
fi

exit 0
