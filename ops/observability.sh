#!/usr/bin/env bash
set -euo pipefail

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

DRY_RUN="false"
COMPOSE_FILE="${COMPOSE_FILE:-${PROJECT_ROOT}/compose.yaml}"
DATA_DIR="${MINECRAFT_DATA_DIR:-${PROJECT_ROOT}/data/minecraft}"
BACKUP_DIR="${BACKUP_DEST:-${PROJECT_ROOT}/backups}"

print_help() {
  cat <<'EOF'
Usage:
  observability.sh [--compose-file <path>] [--dry-run]
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --compose-file)
      COMPOSE_FILE="$2"
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

echo "Compose: ${COMPOSE_FILE}"
echo "Dry-run: ${DRY_RUN}"
echo

if command -v docker >/dev/null 2>&1; then
  if [[ "${DRY_RUN}" == "true" ]]; then
    echo "[DRY-RUN] docker compose -f ${COMPOSE_FILE} ps"
    echo "[DRY-RUN] docker compose -f ${COMPOSE_FILE} exec -T minecraft rcon-cli list"
    echo "[DRY-RUN] docker stats --no-stream"
  else
    docker compose -f "${COMPOSE_FILE}" ps
    if docker compose -f "${COMPOSE_FILE}" ps --services --filter status=running | grep -qx "minecraft"; then
      docker compose -f "${COMPOSE_FILE}" exec -T minecraft rcon-cli list || true
      docker stats --no-stream || true
    fi
  fi
else
  echo "docker not found; skip container status"
fi

if [[ "${DRY_RUN}" == "true" ]]; then
  echo "[DRY-RUN] du -sh ${DATA_DIR} ${BACKUP_DIR}"
  echo "[DRY-RUN] df -h ${DATA_DIR}"
  echo "[DRY-RUN] backup count + sizes in ${BACKUP_DIR}"
  exit 0
fi

if command -v du >/dev/null 2>&1; then
  du -sh "${DATA_DIR}" "${BACKUP_DIR}" 2>/dev/null || true
fi

echo "--- Disk free (target: >20% free) ---"
df -h "${DATA_DIR}" 2>/dev/null || df -h "${PROJECT_ROOT}" || true

echo "--- Backups ---"
backup_count=0
if [[ -d "${BACKUP_DIR}" ]]; then
  backup_count="$(find "${BACKUP_DIR}" -maxdepth 1 -name '*.tar.gz' | wc -l | tr -d ' ')"
fi
echo "Archive count: ${backup_count}"
if [[ "${backup_count}" -gt 0 ]]; then
  find "${BACKUP_DIR}" -maxdepth 1 -name '*.tar.gz' -exec ls -1t {} + | head -n 5 | xargs -r du -h
else
  echo "No backups yet"
fi
