#!/usr/bin/env bash
set -euo pipefail

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DRY_RUN="false"

SOURCE_PATH="${BACKUP_SOURCE:-${PROJECT_ROOT}/data/minecraft}"
DEST_PATH="${BACKUP_DEST:-${PROJECT_ROOT}/backups}"
COMPOSE_FILE="${COMPOSE_FILE:-${PROJECT_ROOT}/compose.yaml}"
RETENTION="${BACKUP_RETENTION:-5}"
LABEL="${BACKUP_LABEL:-minecraft}"

print_help() {
  cat <<'EOF'
Usage:
  backup.sh [--source <path>] [--dest <path>] [--retention <n>] [--label <name>] [--dry-run]

Example:
  ./ops/backup.sh --source ./data/minecraft --dest ./backups --retention 5 --label mc
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --source)
      SOURCE_PATH="$2"
      shift 2
      ;;
    --dest)
      DEST_PATH="$2"
      shift 2
      ;;
    --retention)
      RETENTION="$2"
      shift 2
      ;;
    --label)
      LABEL="$2"
      shift 2
      ;;
    --dry-run)
      DRY_RUN="true"
      shift
      ;;
    -h|--help)
      print_help
      exit 0
      ;;
    *)
      echo "Unknown flag: $1"
      print_help
      exit 1
      ;;
  esac
done

if [[ ! "$RETENTION" =~ ^[0-9]+$ ]]; then
  echo "Invalid retention: $RETENTION"
  exit 1
fi

TIMESTAMP="$(date -u +%Y%m%dT%H%M%SZ)"
ARCHIVE_NAME="${LABEL}-${TIMESTAMP}.tar.gz"
ARCHIVE_PATH="${DEST_PATH}/${ARCHIVE_NAME}"

echo "Backup source : ${SOURCE_PATH}"
echo "Backup target : ${DEST_PATH}"
echo "Retention    : ${RETENTION} backups"
echo "Label        : ${LABEL}"
echo "Dry-run      : ${DRY_RUN}"

if [[ "${DRY_RUN}" == "true" ]]; then
  if [[ ! -d "${SOURCE_PATH}" ]]; then
    echo "[DRY-RUN] mkdir -p ${SOURCE_PATH}"
  fi
  echo "[DRY-RUN] mkdir -p ${DEST_PATH}"
  echo "[DRY-RUN] docker compose -f ${COMPOSE_FILE} exec -T minecraft rcon-cli save-all flush"
  echo "[DRY-RUN] tar -czf ${ARCHIVE_PATH} ${SOURCE_PATH}"
  echo "[DRY-RUN] prune old backups in ${DEST_PATH} keeping ${RETENTION}"
  exit 0
fi

if [[ ! -d "${SOURCE_PATH}" ]]; then
  echo "Source missing: ${SOURCE_PATH}"
  exit 1
fi

mkdir -p "${DEST_PATH}"
if command -v docker >/dev/null 2>&1 && docker compose -f "${COMPOSE_FILE}" ps --services --filter status=running | grep -qx "minecraft"; then
  docker compose -f "${COMPOSE_FILE}" exec -T minecraft rcon-cli save-all flush || true
fi
tar -czf "${ARCHIVE_PATH}" -C "${SOURCE_PATH}" .
echo "Created ${ARCHIVE_PATH}"

if [[ "${RETENTION}" -eq 0 ]]; then
  echo "Retention set to 0; keeping archive only."
  exit 0
fi

mapfile -t BACKUPS < <(ls -1t "${DEST_PATH}/${LABEL}-"*.tar.gz 2>/dev/null || true)
COUNT="${#BACKUPS[@]}"

if [[ "${COUNT}" -le "${RETENTION}" ]]; then
  echo "Retention check passed: ${COUNT}/${RETENTION}"
  exit 0
fi

TO_REMOVE=$((COUNT - RETENTION))
for ((i = RETENTION; i < COUNT; i++)); do
  rm -f "${BACKUPS[$i]}"
  echo "Removed old backup: ${BACKUPS[$i]}"
done
echo "Retention complete: removed ${TO_REMOVE}"
