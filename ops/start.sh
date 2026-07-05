#!/usr/bin/env bash
set -euo pipefail

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
COMPOSE_FILE="${COMPOSE_FILE:-${PROJECT_ROOT}/compose.yaml}"
ENV_FILE="${ENV_FILE:-${PROJECT_ROOT}/.env}"

get_env_value() {
  local key="$1"
  awk -F= -v key="${key}" '$1 == key { sub(/^[^=]*=/, ""); print; exit }' "${ENV_FILE}"
}

if [[ ! -f "${ENV_FILE}" ]]; then
  cp "${PROJECT_ROOT}/.env.example" "${ENV_FILE}"
  echo "created .env from .env.example; set MINECRAFT_EULA=TRUE before first real start"
fi

bash "${PROJECT_ROOT}/ops/sync-runtime-env.sh"

if [[ "$(get_env_value MINECRAFT_EULA)" != "TRUE" ]]; then
  echo "Minecraft EULA not accepted. Set MINECRAFT_EULA=TRUE in .env after accepting the EULA."
  exit 1
fi

docker compose -f "${COMPOSE_FILE}" pull minecraft
docker compose -f "${COMPOSE_FILE}" up -d minecraft

DUCKDNS_DOMAIN_VALUE="$(get_env_value DUCKDNS_DOMAIN)"
DUCKDNS_TOKEN_VALUE="$(get_env_value DUCKDNS_TOKEN)"

if [[ -n "${DUCKDNS_DOMAIN_VALUE}" && -n "${DUCKDNS_TOKEN_VALUE}" ]]; then
  DUCKDNS_DOMAIN="${DUCKDNS_DOMAIN_VALUE}" DUCKDNS_TOKEN="${DUCKDNS_TOKEN_VALUE}" \
    bash "${PROJECT_ROOT}/ops/dns-update.sh"
fi
