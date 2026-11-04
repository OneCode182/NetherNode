#!/usr/bin/env bash
set -euo pipefail

CONTAINER_NAME="${MINECRAFT_CONTAINER_NAME:-nethernode-minecraft}"
DATA_DIR="${MINECRAFT_DATA_DIR:-/opt/nethernode/data/minecraft}"

print_help() {
  cat <<'EOF'
Usage:
  save-server.sh

Environment:
  MINECRAFT_CONTAINER_NAME  Container name. Default: nethernode-minecraft
  MINECRAFT_DATA_DIR        Minecraft data path. Default: /opt/nethernode/data/minecraft
EOF
}

log() {
  printf '[%s] %s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "$*"
}

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  print_help
  exit 0
fi

if docker ps >/dev/null 2>&1; then
  DOCKER=(docker)
else
  DOCKER=(sudo docker)
fi

rcon() {
  "${DOCKER[@]}" exec -i "${CONTAINER_NAME}" rcon-cli "$@"
}

if [[ ! -d "${DATA_DIR}" ]]; then
  echo "Minecraft data dir missing: ${DATA_DIR}" >&2
  exit 1
fi

if ! "${DOCKER[@]}" inspect -f '{{.State.Running}}' "${CONTAINER_NAME}" 2>/dev/null | grep -qx true; then
  echo "Minecraft container is not running: ${CONTAINER_NAME}" >&2
  exit 1
fi

LEVEL_NAME="world"
if [[ -f "${DATA_DIR}/server.properties" ]]; then
  parsed_level="$(awk -F= '$1 == "level-name" {print $2}' "${DATA_DIR}/server.properties" | tail -n 1)"
  if [[ -n "${parsed_level}" ]]; then
    LEVEL_NAME="${parsed_level}"
  fi
fi
WORLD_DIR="${DATA_DIR}/${LEVEL_NAME}"

log "Saving full Minecraft server state with RCON: save-all flush"
rcon save-all flush
sync

log "Save flushed to disk."
log "Data dir : ${DATA_DIR}"
log "World dir: ${WORLD_DIR}"

if [[ -d "${WORLD_DIR}" ]]; then
  world_size="$(du -sh "${WORLD_DIR}" 2>/dev/null | awk '{print $1}')"
  player_files="$(find "${WORLD_DIR}" \( -path "${WORLD_DIR}/playerdata/*.dat" -o -path "${WORLD_DIR}/players/data/*.dat" \) -type f 2>/dev/null | wc -l | tr -d ' ')"
  log "World size: ${world_size:-unknown}"
  log "Player data files: ${player_files:-0}"
else
  log "Warning: world dir not found after save: ${WORLD_DIR}"
fi
