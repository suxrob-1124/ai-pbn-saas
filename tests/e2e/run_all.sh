#!/usr/bin/env bash
if [[ -z "${BASH_VERSION:-}" ]]; then
  exec /usr/bin/env bash "$0" "$@"
fi
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck disable=SC1091
source "${SCRIPT_DIR}/lib.sh"

cleanup() {
  if [[ "${E2E_CLEANUP:-true}" != "false" ]]; then
    "${SCRIPT_DIR}/99_cleanup.sh" || true
  else
    log "[run_all] cleanup skipped (E2E_CLEANUP=false)"
  fi
}
trap cleanup EXIT

# shellcheck disable=SC1091
source "${SCRIPT_DIR}/00_env.sh"

START_STEP="${E2E_START_STEP:-}"
SKIP_STEPS="${E2E_SKIP_STEPS:-}"
steps=(01_up 02_seed 03_generation 04_schedule 05_link_schedule 06_verify)

if [[ "${E2E_RESUME:-}" == "true" && -z "${START_STEP}" ]]; then
  last_success="$(state_get last_success_step)"
  if [[ -n "${last_success}" ]]; then
    next_step=""
    found=false
    for step in "${steps[@]}"; do
      if [[ "${found}" == "true" ]]; then
        next_step="${step}"
        break
      fi
      if [[ "${step}" == "${last_success}" ]]; then
        found=true
      fi
    done
    if [[ -n "${next_step}" ]]; then
      START_STEP="${next_step}"
      log "[run_all] resume from ${START_STEP} (after ${last_success})"
    else
      START_STEP="${last_success}"
      log "[run_all] resume: last step already completed, rerun ${START_STEP}"
    fi
  else
    START_STEP="01_up"
  fi
fi

START_STEP="${START_STEP:-01_up}"

should_run=false
for step in "${steps[@]}"; do
  if [[ "${step}" == "${START_STEP}" ]]; then
    should_run=true
  fi
  if [[ "${should_run}" != "true" ]]; then
    continue
  fi
  if [[ -n "${SKIP_STEPS}" && ",${SKIP_STEPS}," == *",${step},"* ]]; then
    log "[run_all] skip step ${step}"
    continue
  fi
  state_set "current_step" "${step}"
  bash "${SCRIPT_DIR}/${step}.sh"
  state_set "last_success_step" "${step}"
done

log "[run_all] all steps completed"
