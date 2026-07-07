#!/usr/bin/env bash
set -euo pipefail

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
MANIFEST="${PLUGINS_MANIFEST:-${PROJECT_ROOT}/server/plugins.manifest}"
DATA_DIR="${MINECRAFT_DATA_DIR:-${PROJECT_ROOT}/data/minecraft}"
PLUGINS_DIR="${MINECRAFT_PLUGINS_DIR:-${DATA_DIR}/plugins}"
RUNTIME_ENV="${RUNTIME_ENV:-${PROJECT_ROOT}/server/runtime.env}"
# Plugin config templates installed only when the live config is missing.
# Format: <template path>|<target path>
CONFIG_TEMPLATES=(
  "${PROJECT_ROOT}/server/config/geyser/config.yml|${PLUGINS_DIR}/Geyser-Spigot/config.yml"
  "${PROJECT_ROOT}/server/config/tab/config.yml|${PLUGINS_DIR}/TAB/config.yml"
)

# PlaceholderAPI eCloud expansions installed when missing.
# Format: <jar name>|<download url>
PAPI_EXPANSIONS=(
  "player.jar|https://api.extendedclip.com/v2/download/player/latest/"
)
STATE_FILE="${PLUGINS_DIR}/.nethernode-plugins.state"
MODRINTH_API="${MODRINTH_API:-https://api.modrinth.com/v2}"
GEYSERMC_API="${GEYSERMC_API:-https://download.geysermc.org/v2}"

DRY_RUN="false"
MODE="sync"

print_help() {
  cat <<'EOF'
Usage:
  plugins-sync.sh [--dry-run]
  plugins-sync.sh --list

Sync managed Paper crossplay plugins (Geyser, Floodgate, ViaVersion,
ViaBackwards) into the Minecraft plugins dir, from server/plugins.manifest.
Installs the Geyser config template when the plugin config is missing.
Never touches world data.

Options:
  --dry-run   Resolve versions and print planned actions; write nothing.
  --list      Offline: print manifest entries and installed managed jars.
  -h, --help  This help.

Environment:
  PLUGINS_MANIFEST        Manifest path. Default: <repo>/server/plugins.manifest
  MINECRAFT_DATA_DIR      Data dir. Default: <repo>/data/minecraft
  MINECRAFT_PLUGINS_DIR   Plugins dir. Default: $MINECRAFT_DATA_DIR/plugins
  MINECRAFT_VERSION       Minecraft version. Default: from server/runtime.env
  RUNTIME_ENV             runtime.env path. Default: <repo>/server/runtime.env
EOF
}

log() {
  printf '[%s] %s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "$*"
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --dry-run) DRY_RUN="true" ;;
    --list) MODE="list" ;;
    -h|--help)
      print_help
      exit 0
      ;;
    *)
      echo "Unknown argument: $1" >&2
      print_help >&2
      exit 1
      ;;
  esac
  shift
done

if [[ ! -f "${MANIFEST}" ]]; then
  echo "Plugins manifest missing: ${MANIFEST}" >&2
  exit 1
fi

read_manifest() {
  local line
  while IFS= read -r line; do
    [[ -z "${line}" || "${line}" == \#* ]] && continue
    printf '%s\n' "${line}"
  done <"${MANIFEST}"
}

state_get() {
  local name="$1"
  [[ -f "${STATE_FILE}" ]] || return 0
  awk -F'|' -v name="${name}" '$1 == name { print $2; exit }' "${STATE_FILE}"
}

state_set() {
  local name="$1"
  local filename="$2"
  local tmp
  tmp="$(mktemp)"
  if [[ -f "${STATE_FILE}" ]]; then
    awk -F'|' -v name="${name}" '$1 != name { print }' "${STATE_FILE}" >"${tmp}"
  fi
  printf '%s|%s\n' "${name}" "${filename}" >>"${tmp}"
  mv "${tmp}" "${STATE_FILE}"
}

if [[ "${MODE}" == "list" ]]; then
  echo "Managed plugin manifest: ${MANIFEST}"
  while IFS='|' read -r name source project channel pin; do
    printf '  %-13s source=%-9s project=%-13s channel=%-7s pin=%s\n' \
      "${name}" "${source}" "${project}" "${channel}" "${pin}"
  done < <(read_manifest)
  echo "Plugins dir: ${PLUGINS_DIR}"
  if [[ -d "${PLUGINS_DIR}" ]]; then
    found="false"
    while IFS= read -r jar; do
      found="true"
      printf '  installed: %s\n' "$(basename "${jar}")"
    done < <(find "${PLUGINS_DIR}" -maxdepth 1 -type f -name '*.jar' | sort)
    [[ "${found}" == "false" ]] && echo "  installed: none"
  else
    echo "  installed: none (plugins dir absent)"
  fi
  exit 0
fi

for tool in curl python3; do
  if ! command -v "${tool}" >/dev/null 2>&1; then
    echo "Required tool missing: ${tool}" >&2
    exit 1
  fi
done

if [[ -z "${MINECRAFT_VERSION:-}" && -f "${RUNTIME_ENV}" ]]; then
  MINECRAFT_VERSION="$(awk -F= '$1 == "MINECRAFT_VERSION" { print $2; exit }' "${RUNTIME_ENV}")"
fi
if [[ -z "${MINECRAFT_VERSION:-}" ]]; then
  echo "MINECRAFT_VERSION not set and not found in ${RUNTIME_ENV}" >&2
  exit 1
fi

# Resolvers print: version|url|checksum(algo:hex or empty)|filename
resolve_modrinth() {
  local slug="$1"
  local loader="$2"
  local mcver="$3"
  curl -fsSL \
    "${MODRINTH_API}/project/${slug}/version?loaders=%5B%22${loader}%22%5D&game_versions=%5B%22${mcver}%22%5D" |
    python3 -c '
import json, sys
versions = json.load(sys.stdin)
if not versions:
    sys.exit(3)
v = versions[0]
f = next((x for x in v["files"] if x.get("primary")), v["files"][0])
sha = f.get("hashes", {}).get("sha512", "")
checksum = f"sha512:{sha}" if sha else ""
print("|".join([v["version_number"], f["url"], checksum, f["filename"]]))
'
}

resolve_geysermc() {
  local project="$1"
  local platform="$2"
  local version build
  version="$(curl -fsSL "${GEYSERMC_API}/projects/${project}" |
    python3 -c 'import json, sys; print(json.load(sys.stdin)["versions"][-1])')"
  build="$(curl -fsSL "${GEYSERMC_API}/projects/${project}/versions/${version}" |
    python3 -c 'import json, sys; print(json.load(sys.stdin)["builds"][-1])')"
  curl -fsSL "${GEYSERMC_API}/projects/${project}/versions/${version}/builds/${build}" |
    python3 -c "
import json, sys
data = json.load(sys.stdin)
dl = data['downloads']['${platform}']
sha = dl.get('sha256', '')
checksum = f'sha256:{sha}' if sha else ''
url = '${GEYSERMC_API}/projects/${project}/versions/${version}/builds/${build}/downloads/${platform}'
print('|'.join(['${version} b${build}', url, checksum, dl['name']]))
"
}

verify_checksum() {
  local file="$1"
  local checksum="$2"
  [[ -z "${checksum}" ]] && return 0
  local algo="${checksum%%:*}"
  local expected="${checksum#*:}"
  local actual
  case "${algo}" in
    sha256) actual="$(sha256sum "${file}" | awk '{print $1}')" ;;
    sha512) actual="$(sha512sum "${file}" | awk '{print $1}')" ;;
    *)
      echo "Unknown checksum algorithm: ${algo}" >&2
      return 1
      ;;
  esac
  if [[ "${actual}" != "${expected}" ]]; then
    echo "Checksum mismatch for ${file}: expected ${expected}, got ${actual}" >&2
    return 1
  fi
}

install_plugin() {
  local name="$1"
  local url="$2"
  local checksum="$3"
  local filename="$4"
  local dest="${PLUGINS_DIR}/${filename}"
  local tmp="${PLUGINS_DIR}/.${filename}.tmp"

  mkdir -p "${PLUGINS_DIR}"
  rm -f "${tmp}"
  curl -fsSL -o "${tmp}" "${url}"
  verify_checksum "${tmp}" "${checksum}"
  mv "${tmp}" "${dest}"

  local previous
  previous="$(state_get "${name}")"
  if [[ -n "${previous}" && "${previous}" != "${filename}" && -f "${PLUGINS_DIR}/${previous}" ]]; then
    rm -f "${PLUGINS_DIR}/${previous}"
    log "Removed superseded ${name} jar: ${previous}"
  fi
  state_set "${name}" "${filename}"
}

FAILURES=0
log "Syncing managed plugins (Minecraft ${MINECRAFT_VERSION}) -> ${PLUGINS_DIR}"
[[ "${DRY_RUN}" == "true" ]] && log "Dry-run: nothing will be written."

while IFS='|' read -r name source project channel pin; do
  resolved=""
  case "${source}" in
    modrinth)
      resolved="$(resolve_modrinth "${project}" "${channel}" "${MINECRAFT_VERSION}")" || true
      ;;
    geysermc)
      resolved="$(resolve_geysermc "${project}" "${channel}")" || true
      ;;
    *)
      echo "Unknown source '${source}' for plugin '${name}'" >&2
      FAILURES=$((FAILURES + 1))
      continue
      ;;
  esac

  if [[ -z "${resolved}" ]]; then
    echo "Could not resolve ${name} (source=${source}, project=${project}, channel=${channel}, mc=${MINECRAFT_VERSION})" >&2
    FAILURES=$((FAILURES + 1))
    continue
  fi

  IFS='|' read -r version url checksum filename <<<"${resolved}"
  action="install"
  if [[ -f "${PLUGINS_DIR}/${filename}" ]]; then
    if verify_checksum "${PLUGINS_DIR}/${filename}" "${checksum}" 2>/dev/null; then
      action="keep"
    else
      action="upgrade"
    fi
  fi

  log "${name}: ${action} ${version} (${filename})"
  if [[ "${DRY_RUN}" == "true" || "${action}" == "keep" ]]; then
    continue
  fi
  if ! install_plugin "${name}" "${url}" "${checksum}" "${filename}"; then
    echo "Install failed for ${name}" >&2
    FAILURES=$((FAILURES + 1))
  fi
done < <(read_manifest)

for spec in "${CONFIG_TEMPLATES[@]}"; do
  template="${spec%%|*}"
  target="${spec##*|}"
  if [[ -f "${template}" && ! -f "${target}" ]]; then
    log "Config missing: installing template -> ${target}"
    if [[ "${DRY_RUN}" != "true" ]]; then
      mkdir -p "$(dirname "${target}")"
      cp "${template}" "${target}"
    fi
  fi
done

for spec in "${PAPI_EXPANSIONS[@]}"; do
  jar="${spec%%|*}"
  url="${spec##*|}"
  target="${PLUGINS_DIR}/PlaceholderAPI/expansions/${jar}"
  if [[ ! -f "${target}" ]]; then
    log "PAPI expansion missing: installing ${jar}"
    if [[ "${DRY_RUN}" != "true" ]]; then
      mkdir -p "$(dirname "${target}")"
      curl -fsSL -o "${target}.tmp" "${url}"
      mv "${target}.tmp" "${target}"
    fi
  fi
done

# The container runs as UID/GID 1000; when sync runs as root (sudo on EC2),
# root-owned plugin files would block plugins from writing their own configs.
if [[ "${DRY_RUN}" != "true" && "$(id -u)" == "0" && -d "${PLUGINS_DIR}" ]]; then
  chown -R "${MINECRAFT_UID:-1000}:${MINECRAFT_GID:-1000}" "${PLUGINS_DIR}"
fi

if (( FAILURES > 0 )); then
  log "Plugin sync finished with ${FAILURES} failure(s)."
  exit 1
fi
log "Plugin sync complete."
