#!/usr/bin/env bash
set -euo pipefail

WORK_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

export SEED_USERS="${SEED_USERS:-true}"
export SEED_PROJECTS="${SEED_PROJECTS:-true}"
export SEED_DOMAINS="${SEED_DOMAINS:-false}"

exec "${WORK_DIR}/seed_backend.sh" "$@"
