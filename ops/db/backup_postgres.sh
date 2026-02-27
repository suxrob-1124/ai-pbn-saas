#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "${ROOT_DIR}"

SERVICE_NAME="${BACKUP_DB_SERVICE:-db}"
CONTAINER_BACKUP_DIR="${BACKUP_CONTAINER_DIR:-/var/lib/postgresql/backups}"
LOCAL_BACKUP_DIR="${BACKUP_LOCAL_DIR:-${ROOT_DIR}/backups/postgres}"
TIMESTAMP="$(date -u +%Y%m%dT%H%M%SZ)"
FILE_NAME="postgres_${TIMESTAMP}.sql.gz"
CONTAINER_FILE="${CONTAINER_BACKUP_DIR}/${FILE_NAME}"
LOCAL_FILE="${LOCAL_BACKUP_DIR}/${FILE_NAME}"

mkdir -p "${LOCAL_BACKUP_DIR}"

if ! docker compose ps --status running "${SERVICE_NAME}" >/dev/null 2>&1; then
  echo "[backup] service '${SERVICE_NAME}' is not running. Start it first: docker compose up -d ${SERVICE_NAME}" >&2
  exit 1
fi

echo "[backup] creating gzip dump in container and local file"
docker compose exec -T "${SERVICE_NAME}" sh -lc \
  "set -euo pipefail; \
   mkdir -p '${CONTAINER_BACKUP_DIR}'; \
   export PGPASSWORD=\"\${POSTGRES_PASSWORD:-}\"; \
   pg_dump -U \"\${POSTGRES_USER}\" -d \"\${POSTGRES_DB}\" | gzip -c | tee '${CONTAINER_FILE}'" \
  > "${LOCAL_FILE}"

if [[ ! -s "${LOCAL_FILE}" ]]; then
  echo "[backup] local backup file is empty: ${LOCAL_FILE}" >&2
  exit 1
fi

ln -sfn "${FILE_NAME}" "${LOCAL_BACKUP_DIR}/latest.sql.gz"

echo "[backup] done"
echo "  local:     ${LOCAL_FILE}"
echo "  container: ${CONTAINER_FILE}"
