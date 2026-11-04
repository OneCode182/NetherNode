#!/usr/bin/env bash
set -euo pipefail

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SCRIPT_DIR="${NETHERNODE_SCRIPT_DIR:-/opt/nethernode/scripts}"
BIN_PATH="${NETHERNODE_BIN_PATH:-/usr/local/bin/nethernode}"
CLI_IMAGE="${NETHERNODE_CLI_IMAGE:-}"
GO_BUILDER_IMAGE="${NETHERNODE_GO_BUILDER_IMAGE:-golang:1.26-alpine}"

install_file() {
  local source="$1"
  local target="$2"

  install -D -m 0755 "${source}" "${target}"
}

install_cli_from_image() {
  local image="$1"
  local tmp_dir cid

  if ! command -v docker >/dev/null 2>&1; then
    echo "docker is required to extract CLI from image: ${image}" >&2
    return 1
  fi

  if ! docker image inspect "${image}" >/dev/null 2>&1; then
    docker pull "${image}"
  fi

  tmp_dir="$(mktemp -d)"
  cid="$(docker create --entrypoint /bin/sh "${image}" -c true)"
  trap 'docker rm -f "${cid}" >/dev/null 2>&1 || true; rm -rf "${tmp_dir}"' RETURN

  docker cp "${cid}:/usr/local/bin/nethernode" "${tmp_dir}/nethernode"
  install_file "${tmp_dir}/nethernode" "${BIN_PATH}"
}

install_cli_from_source() {
  local tmp_dir

  if ! command -v go >/dev/null 2>&1; then
    return 1
  fi

  tmp_dir="$(mktemp -d)"
  trap 'rm -rf "${tmp_dir}"' RETURN
  if ! (cd "${PROJECT_ROOT}" && go build -trimpath -ldflags="-s -w" -o "${tmp_dir}/nethernode" ./cmd/nethernode); then
    return 1
  fi
  install_file "${tmp_dir}/nethernode" "${BIN_PATH}"
}

install_cli_from_docker_builder() {
  local tmp_dir

  if ! command -v docker >/dev/null 2>&1; then
    return 1
  fi

  tmp_dir="$(mktemp -d)"
  trap 'rm -rf "${tmp_dir}"' RETURN
  if ! docker run --rm \
    -v "${PROJECT_ROOT}:/src:ro" \
    -v "${tmp_dir}:/out" \
    -w /src \
    "${GO_BUILDER_IMAGE}" \
    sh -ceu 'go build -trimpath -ldflags="-s -w" -o /out/nethernode ./cmd/nethernode'; then
    return 1
  fi
  install_file "${tmp_dir}/nethernode" "${BIN_PATH}"
}

mkdir -p "${SCRIPT_DIR}"
install_file "${PROJECT_ROOT}/ops/save-server.sh" "${SCRIPT_DIR}/save-server.sh"
install_file "${PROJECT_ROOT}/ops/backup-server.sh" "${SCRIPT_DIR}/backup-server.sh"
install_file "${PROJECT_ROOT}/ops/plugins-sync.sh" "${SCRIPT_DIR}/plugins-sync.sh"

if [[ -n "${CLI_IMAGE}" ]]; then
  install_cli_from_image "${CLI_IMAGE}"
elif install_cli_from_source; then
  :
elif install_cli_from_docker_builder; then
  :
else
  echo "warning: Go toolchain unavailable; installing legacy shell wrapper" >&2
  install_file "${PROJECT_ROOT}/ops/nethernode" "${BIN_PATH}"
fi

echo "Installed NetherNode CLI: ${BIN_PATH}"
echo "Installed scripts: ${SCRIPT_DIR}"
