#!/usr/bin/env bash
set -euo pipefail

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ENV_FILE="${ENV_FILE:-${PROJECT_ROOT}/.env}"
RUNTIME_ENV="${RUNTIME_ENV:-${PROJECT_ROOT}/server/runtime.env}"

set_env_value() {
  local key="$1"
  local value="$2"
  local file="$3"
  local tmp
  tmp="$(mktemp)"

  if grep -q "^${key}=" "${file}"; then
    awk -v key="${key}" -v value="${value}" '
      BEGIN { FS = "=" }
      $1 == key { print key "=" value; next }
      { print }
    ' "${file}" >"${tmp}"
  else
    cp "${file}" "${tmp}"
    printf '%s=%s\n' "${key}" "${value}" >>"${tmp}"
  fi

  # Preserve the target's owner/mode: mktemp files are 600 and, run via sudo,
  # would flip .env to root-only, breaking the CLI's .env fallback for the
  # invoking user.
  chown --reference="${file}" "${tmp}" 2>/dev/null || true
  chmod --reference="${file}" "${tmp}" 2>/dev/null || true
  mv "${tmp}" "${file}"
}

if [[ ! -f "${ENV_FILE}" ]]; then
  echo "Env file missing: ${ENV_FILE}"
  exit 1
fi

if [[ ! -f "${RUNTIME_ENV}" ]]; then
  echo "Runtime env missing: ${RUNTIME_ENV}"
  exit 1
fi

while IFS='=' read -r key value; do
  [[ -z "${key}" || "${key}" == \#* ]] && continue
  case "${key}" in
    MINECRAFT_*)
      set_env_value "${key}" "${value}" "${ENV_FILE}"
      ;;
  esac
done <"${RUNTIME_ENV}"

echo "Synced ${RUNTIME_ENV} -> ${ENV_FILE}"
