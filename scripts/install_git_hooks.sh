#!/usr/bin/env bash
set -euo pipefail

ROOT="$(git rev-parse --show-toplevel)"
HOOKS_DIR="$ROOT/.githooks"

if [[ ! -d "$HOOKS_DIR" ]]; then
  echo "hooks directory not found: $HOOKS_DIR" >&2
  exit 1
fi

chmod +x "$ROOT/scripts/check_no_secrets.sh"
chmod +x "$HOOKS_DIR/pre-commit" "$HOOKS_DIR/pre-push"

git config core.hooksPath "$HOOKS_DIR"
echo "Git hooks installed. core.hooksPath=$HOOKS_DIR"
