#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
WORKFLOW_DIR="${ROOT}/.github/workflows"

FORBIDDEN_REGEX='aws[[:space:]]+ec2[[:space:]]+(start-instances|stop-instances)|aws[[:space:]]+ssm[[:space:]]+send-command|docker[[:space:]]+compose[[:space:]]+(up|down|restart)|ops/(start|stop-safe)\.sh|/opt/nethernode/data/minecraft'

failed=0

for workflow in "${WORKFLOW_DIR}"/*.yml "${WORKFLOW_DIR}"/*.yaml; do
  [[ -e "${workflow}" ]] || continue
  case "$(basename "${workflow}")" in
    start-server.yml|start-server.yaml|stop-server.yml|stop-server.yaml)
      continue
      ;;
  esac

  if grep -En "${FORBIDDEN_REGEX}" "${workflow}"; then
    echo "Forbidden runtime mutation command found in non-lifecycle workflow: ${workflow}" >&2
    failed=1
  fi
done

if [[ "${failed}" -ne 0 ]]; then
  exit 1
fi

echo "ci_no_reset_ok"
