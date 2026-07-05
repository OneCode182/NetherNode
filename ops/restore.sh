#!/usr/bin/env bash
set -euo pipefail

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DRY_RUN="false"
FORCE="false"

ARCHIVE_PATH=""
TARGET_PATH="${BACKUP_SOURCE:-${PROJECT_ROOT}/data/minecraft}"

print_help() {
  cat <<'EOF'
Usage:
  restore.sh --archive <path> [--target <path>] [--force] [--dry-run]

Options:
  --force   remove existing target directory before restore
  --dry-run show actions without touching files
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --archive)
      ARCHIVE_PATH="$2"
      shift 2
      ;;
    --target)
      TARGET_PATH="$2"
      shift 2
      ;;
    --force)
      FORCE="true"
      shift
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

if [[ -z "${ARCHIVE_PATH}" ]]; then
  echo "Archive required: --archive <path>"
  exit 1
fi

if [[ ! -f "${ARCHIVE_PATH}" ]]; then
  echo "Archive missing: ${ARCHIVE_PATH}"
  exit 1
fi

if [[ -d "${TARGET_PATH}" && "${FORCE}" != "true" ]]; then
  echo "Target exists: ${TARGET_PATH}. Use --force to replace."
  exit 1
fi

echo "Archive: ${ARCHIVE_PATH}"
echo "Target : ${TARGET_PATH}"
echo "Force  : ${FORCE}"
echo "Dry-run: ${DRY_RUN}"

if [[ "${DRY_RUN}" == "true" ]]; then
  echo "[DRY-RUN] mkdir -p ${TARGET_PATH}"
  echo "[DRY-RUN] tar -xzf ${ARCHIVE_PATH} -C ${TARGET_PATH}"
  exit 0
fi

mkdir -p "${TARGET_PATH}"
if [[ "${FORCE}" == "true" ]]; then
  rm -rf "${TARGET_PATH:?}"/*
fi
tar -xzf "${ARCHIVE_PATH}" -C "${TARGET_PATH}"
echo "Restore done."
