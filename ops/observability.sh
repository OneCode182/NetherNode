#!/usr/bin/env bash
set -euo pipefail

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

DRY_RUN="false"
COMPOSE_FILE="${COMPOSE_FILE:-${PROJECT_ROOT}/compose.yaml}"
WORKER_URL="${WORKER_URL:-http://127.0.0.1:8080}"

print_help() {
  cat <<'EOF'
Usage:
  observability.sh [--compose-file <path>] [--worker-url <url>] [--dry-run]
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --compose-file)
      COMPOSE_FILE="$2"
      shift 2
      ;;
    --worker-url)
      WORKER_URL="$2"
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
echo "Worker:  ${WORKER_URL}"
echo "Dry-run: ${DRY_RUN}"
echo

if command -v docker >/dev/null 2>&1; then
  if [[ "${DRY_RUN}" == "true" ]]; then
    echo "[DRY-RUN] docker compose -f ${COMPOSE_FILE} ps"
  else
    docker compose -f "${COMPOSE_FILE}" ps
  fi
else
  echo "docker not found; skip container status"
fi

if [[ "${DRY_RUN}" == "true" ]]; then
  echo "[DRY-RUN] curl ${WORKER_URL}/health"
  echo "[DRY-RUN] curl ${WORKER_URL}/metrics"
  echo "[DRY-RUN] du -sh ./data/minecraft ./data/workflows ./backups"
  exit 0
fi

if command -v curl >/dev/null 2>&1; then
  echo "--- Worker health ---"
  curl -fsS "${WORKER_URL}/health" | sed 's/^/health: /' || echo "Worker health: unavailable"
  echo "--- Worker metrics ---"
  curl -fsS "${WORKER_URL}/metrics" || echo "Worker metrics: unavailable"
else
  echo "curl not found; skip worker probes"
fi

if command -v du >/dev/null 2>&1; then
  du -sh ./data/minecraft ./data/workflows ./backups 2>/dev/null || true
fi

if command -v ls >/dev/null 2>&1; then
  echo "--- Recent backups ---"
  ls -1t ./backups/*.tar.gz 2>/dev/null | head -n 5 || echo "No backups yet"
fi
