#!/usr/bin/env bash
set -euo pipefail

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SCRIPT_DIR="${NETHERNODE_SCRIPT_DIR:-/opt/nethernode/scripts}"
BIN_PATH="${NETHERNODE_BIN_PATH:-/usr/local/bin/nethernode}"

install_file() {
  local source="$1"
  local target="$2"

  install -D -m 0755 "${source}" "${target}"
}

mkdir -p "${SCRIPT_DIR}"
install_file "${PROJECT_ROOT}/ops/save-server.sh" "${SCRIPT_DIR}/save-server.sh"
install_file "${PROJECT_ROOT}/ops/backup-server.sh" "${SCRIPT_DIR}/backup-server.sh"
install_file "${PROJECT_ROOT}/ops/plugins-sync.sh" "${SCRIPT_DIR}/plugins-sync.sh"
install_file "${PROJECT_ROOT}/ops/nethernode" "${BIN_PATH}"

echo "Installed NetherNode CLI: ${BIN_PATH}"
echo "Installed scripts: ${SCRIPT_DIR}"
