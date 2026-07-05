#!/usr/bin/env bash
set -euo pipefail

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
COMPOSE_FILE="${COMPOSE_FILE:-${PROJECT_ROOT}/compose.yaml}"
DRY_RUN="false"
BACKUP_BEFORE_STOP="true"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --dry-run)
      DRY_RUN="true"
      shift
      ;;
    --skip-backup)
      BACKUP_BEFORE_STOP="false"
      shift
      ;;
    -h|--help)
      echo "Usage: stop-safe.sh [--dry-run] [--skip-backup]"
      exit 0
      ;;
    *)
      echo "Unknown flag: $1"
      exit 1
      ;;
  esac
done

if [[ "${DRY_RUN}" == "true" ]]; then
  echo "[DRY-RUN] docker compose -f ${COMPOSE_FILE} exec -T minecraft rcon-cli save-all flush"
  echo "[DRY-RUN] bash ${PROJECT_ROOT}/ops/backup.sh"
  echo "[DRY-RUN] docker compose -f ${COMPOSE_FILE} exec -T minecraft rcon-cli stop"
  echo "[DRY-RUN] docker compose -f ${COMPOSE_FILE} down"
  exit 0
fi

if docker compose -f "${COMPOSE_FILE}" ps --services --filter status=running | grep -qx "minecraft"; then
  docker compose -f "${COMPOSE_FILE}" exec -T minecraft rcon-cli save-all flush || true
  if [[ "${BACKUP_BEFORE_STOP}" == "true" ]]; then
    bash "${PROJECT_ROOT}/ops/backup.sh"
  fi
  docker compose -f "${COMPOSE_FILE}" exec -T minecraft rcon-cli stop || true
  sleep 10
fi

docker compose -f "${COMPOSE_FILE}" down
