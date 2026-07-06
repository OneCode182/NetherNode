#!/usr/bin/env bash
set -euo pipefail

CONTAINER_NAME="${MINECRAFT_CONTAINER_NAME:-nethernode-minecraft}"
DATA_DIR="${MINECRAFT_DATA_DIR:-/opt/nethernode/data/minecraft}"
BACKUP_DIR="${BACKUP_DEST:-/opt/nethernode/backups}"
RETENTION="${BACKUP_RETENTION:-5}"
LABEL="${BACKUP_LABEL:-minecraft}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SAVE_SCRIPT="${SAVE_SCRIPT:-${SCRIPT_DIR}/save-server.sh}"

print_help() {
  cat <<'EOF'
Usage:
  backup-server.sh

Environment:
  MINECRAFT_CONTAINER_NAME  Container name. Default: nethernode-minecraft
  MINECRAFT_DATA_DIR        Minecraft data path. Default: /opt/nethernode/data/minecraft
  BACKUP_DEST               Backup dir. Default: /opt/nethernode/backups
  BACKUP_RETENTION          Backups to keep. Default: 5
  BACKUP_LABEL              Backup filename prefix. Default: minecraft
  SAVE_SCRIPT               Save script path. Default: sibling save-server.sh
EOF
}

log() {
  printf '[%s] %s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "$*"
}

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  print_help
  exit 0
fi

if [[ ! "${RETENTION}" =~ ^[1-9][0-9]*$ ]]; then
  echo "BACKUP_RETENTION must be a positive integer. Current: ${RETENTION}" >&2
  exit 1
fi

if [[ ! -x "${SAVE_SCRIPT}" ]]; then
  echo "Save script missing or non-executable: ${SAVE_SCRIPT}" >&2
  exit 1
fi

if docker ps >/dev/null 2>&1; then
  DOCKER=(docker)
else
  DOCKER=(sudo docker)
fi

rcon() {
  "${DOCKER[@]}" exec -i "${CONTAINER_NAME}" rcon-cli "$@"
}

SAVE_DISABLED="false"
cleanup() {
  if [[ "${SAVE_DISABLED}" == "true" ]]; then
    log "Re-enabling Minecraft autosave after interrupted backup."
    rcon save-on >/dev/null 2>&1 || true
  fi
}
trap cleanup EXIT

if [[ ! -d "${DATA_DIR}" ]]; then
  echo "Minecraft data dir missing: ${DATA_DIR}" >&2
  exit 1
fi

if ! "${DOCKER[@]}" inspect -f '{{.State.Running}}' "${CONTAINER_NAME}" 2>/dev/null | grep -qx true; then
  echo "Minecraft container is not running: ${CONTAINER_NAME}" >&2
  exit 1
fi

log "Step 1/4: force-save complete Minecraft state."
"${SAVE_SCRIPT}"

log "Step 2/4: pause autosave for a consistent on-disk archive."
rcon save-off
SAVE_DISABLED="true"
rcon save-all flush
sync

mkdir -p "${BACKUP_DIR}"
TIMESTAMP="$(date -u +%Y%m%dT%H%M%SZ)"
ARCHIVE_NAME="${LABEL}-${TIMESTAMP}.tar.gz"
ARCHIVE_PATH="${BACKUP_DIR}/${ARCHIVE_NAME}"
TMP_ARCHIVE="${BACKUP_DIR}/.${ARCHIVE_NAME}.tmp"

log "Step 3/4: create backup archive: ${ARCHIVE_PATH}"
rm -f "${TMP_ARCHIVE}"
tar -czf "${TMP_ARCHIVE}" -C "${DATA_DIR}" .
mv "${TMP_ARCHIVE}" "${ARCHIVE_PATH}"
sync

rcon save-on
SAVE_DISABLED="false"

archive_size="$(du -h "${ARCHIVE_PATH}" | awk '{print $1}')"
log "Backup created: ${ARCHIVE_PATH} (${archive_size})"

log "Step 4/4: prune old backups, keeping newest ${RETENTION}."
mapfile -t BACKUPS < <(find "${BACKUP_DIR}" -maxdepth 1 -type f -name "${LABEL}-*.tar.gz" -printf '%T@ %p\n' | sort -nr | cut -d' ' -f2-)
COUNT="${#BACKUPS[@]}"

if (( COUNT <= RETENTION )); then
  log "Retention OK: ${COUNT}/${RETENTION} backups."
  exit 0
fi

REMOVED=0
for ((i = RETENTION; i < COUNT; i++)); do
  rm -f "${BACKUPS[$i]}"
  log "Removed old backup: ${BACKUPS[$i]}"
  REMOVED=$((REMOVED + 1))
done

log "Retention complete: removed ${REMOVED}; kept ${RETENTION}."
