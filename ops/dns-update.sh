#!/usr/bin/env bash
set -euo pipefail

DRY_RUN="false"
DOMAIN="${DUCKDNS_DOMAIN:-}"
TOKEN="${DUCKDNS_TOKEN:-}"
IP="${PUBLIC_IP:-}"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --domain)
      DOMAIN="$2"
      shift 2
      ;;
    --token)
      TOKEN="$2"
      shift 2
      ;;
    --ip)
      IP="$2"
      shift 2
      ;;
    --dry-run)
      DRY_RUN="true"
      shift
      ;;
    -h|--help)
      echo "Usage: dns-update.sh --domain <duckdns-subdomain> --token <token> [--ip <ip>] [--dry-run]"
      exit 0
      ;;
    *)
      echo "Unknown flag: $1"
      exit 1
      ;;
  esac
done

if [[ -z "${DOMAIN}" || -z "${TOKEN}" ]]; then
  echo "DUCKDNS_DOMAIN and DUCKDNS_TOKEN required"
  exit 1
fi

URL="https://www.duckdns.org/update?domains=${DOMAIN}&token=<redacted>&ip=${IP}"

if [[ "${DRY_RUN}" == "true" ]]; then
  echo "[DRY-RUN] curl ${URL}"
  exit 0
fi

curl -fsS "https://www.duckdns.org/update?domains=${DOMAIN}&token=${TOKEN}&ip=${IP}"
echo
