#!/usr/bin/env bash
set -Eeuo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
CONTROLLER="${REPO_ROOT}/scripts/nethernode.sh"
TEMPLATE="${REPO_ROOT}/scripts/nethernode.env.example"
TMP_DIR="$(mktemp -d)"
MOCK_BIN="${TMP_DIR}/bin"
MOCK_LOG="${TMP_DIR}/calls.log"
MOCK_STATE_FILE="${TMP_DIR}/ec2-state"
CONFIG_FILE="${TMP_DIR}/nethernode.local.env"
KEY_FILE="${TMP_DIR}/test-key"

cleanup() {
  rm -rf "${TMP_DIR}"
}
trap cleanup EXIT

fail() {
  printf 'FAIL: %s\n' "$*" >&2
  exit 1
}

pass() {
  printf 'PASS: %s\n' "$*"
}

assert_contains() {
  local needle="$1" file="$2"
  grep -F -- "${needle}" "${file}" >/dev/null || fail "missing ${needle} in ${file}"
}

assert_not_contains() {
  local needle="$1" file="$2"
  if grep -F -- "${needle}" "${file}" >/dev/null; then
    fail "unexpected ${needle} in ${file}"
  fi
}

line_number() {
  local needle="$1" file="$2"
  grep -n -F -- "${needle}" "${file}" | sed -n '1s/:.*//p'
}

reset_mocks() {
  local state="$1"
  printf '%s\n' "${state}" > "${MOCK_STATE_FILE}"
  : > "${MOCK_LOG}"
}

run_controller() {
  PATH="${MOCK_BIN}:${PATH}" \
    MOCK_LOG="${MOCK_LOG}" \
    MOCK_STATE_FILE="${MOCK_STATE_FILE}" \
    NETHERNODE_CLOUD_ENV="${CONFIG_FILE}" \
    NO_COLOR=1 \
    bash "${CONTROLLER}" "$@"
}

mkdir -p "${MOCK_BIN}"
printf 'test key\n' > "${KEY_FILE}"
chmod 600 "${KEY_FILE}"
cat > "${CONFIG_FILE}" <<EOF
AWS_REGION=us-east-1
EC2_INSTANCE_ID=i-controller-test
SSH_USER=ubuntu
SSH_KEY_PATH=${KEY_FILE}
REMOTE_APP_DIR=/srv/nethernode
MINECRAFT_STATUS_HOST=127.0.0.1
POLL_INTERVAL_SECONDS=1
SSH_CONNECT_TIMEOUT_SECONDS=1
EOF

cat > "${MOCK_BIN}/aws" <<'EOF'
#!/usr/bin/env bash
set -Eeuo pipefail
printf 'aws %s\n' "$*" >> "${MOCK_LOG}"

case " $* " in
  *" ec2 describe-instances "*)
    state="$(cat "${MOCK_STATE_FILE}")"
    if [[ "${state}" == 'running' ]]; then
      printf 'running\t203.0.113.10\n'
    else
      printf '%s\tNone\n' "${state}"
    fi
    ;;
  *" ec2 start-instances "*) printf 'running\n' > "${MOCK_STATE_FILE}" ;;
  *" ec2 wait instance-status-ok "*) : ;;
  *" ec2 stop-instances "*) printf 'stopped\n' > "${MOCK_STATE_FILE}" ;;
  *" ec2 wait instance-stopped "*) : ;;
  *) printf 'unexpected aws command: %s\n' "$*" >&2; exit 1 ;;
esac
EOF

cat > "${MOCK_BIN}/ssh" <<'EOF'
#!/usr/bin/env bash
set -Eeuo pipefail
command="${*: -1}"
readable_command="${command//\\/}"
printf 'ssh %s\n' "${readable_command}" >> "${MOCK_LOG}"
case "${readable_command}" in
  *'nethernode status'*) printf 'remote status: online\n' ;;
  *"docker inspect -f"*)
    [[ "${MOCK_CONTAINER_LEADING_BLANK:-false}" == 'true' ]] && printf '\n'
    printf '%s\n' "${MOCK_CONTAINER_STATE:-false}"
    ;;
  *'nethernode backup-server'*) : ;;
  *'nethernode stop --no-backup'*) : ;;
  *'docker exec nethernode-minecraft rcon-cli list'*) printf 'There are 0 of a max of 5 players online:\n' ;;
  *'bash ops/start.sh'*) : ;;
  *'bash -lc true'*) : ;;
  *) printf 'unexpected ssh command: %s\n' "${command}" >&2; exit 1 ;;
esac
EOF
chmod +x "${MOCK_BIN}/aws" "${MOCK_BIN}/ssh"

help_output="${TMP_DIR}/help.out"
bash "${CONTROLLER}" help > "${help_output}"
assert_contains 'NetherNode cloud controller' "${help_output}"
pass 'help works without config'

stopped_output="${TMP_DIR}/stopped.out"
reset_mocks stopped
run_controller status --once > "${stopped_output}"
assert_contains 'EC2: stopped' "${stopped_output}"
assert_not_contains 'ssh ' "${MOCK_LOG}"
pass 'stopped status skips ssh'

running_output="${TMP_DIR}/running.out"
reset_mocks running
run_controller status --once > "${running_output}"
assert_contains 'remote status: online' "${running_output}"
assert_contains 'ssh ' "${MOCK_LOG}"
pass 'running status renders remote status'

reset_mocks stopped
run_controller start --only-ec2 --no-watch > /dev/null
assert_contains 'ec2 start-instances' "${MOCK_LOG}"
assert_contains 'ec2 wait instance-status-ok' "${MOCK_LOG}"
assert_not_contains 'ssh ' "${MOCK_LOG}"
pass 'ec2-only start starts and waits without ssh'

reset_mocks stopped
run_controller start --no-watch > /dev/null
assert_contains 'bash ops/start.sh' "${MOCK_LOG}"
assert_contains 'docker exec nethernode-minecraft rcon-cli list' "${MOCK_LOG}"
pass 'full start waits for Minecraft RCON readiness'

stopped_stop_output="${TMP_DIR}/stopped-stop.out"
reset_mocks stopped
run_controller stop --no-watch > "${stopped_stop_output}"
assert_contains 'EC2 already stopped; nothing to stop.' "${stopped_stop_output}"
assert_not_contains 'ssh ' "${MOCK_LOG}"
assert_not_contains 'ec2 stop-instances' "${MOCK_LOG}"
pass 'stopped stop exits without ssh or EC2 stop'

refusal_output="${TMP_DIR}/refusal.out"
reset_mocks running
if MOCK_CONTAINER_STATE=true run_controller stop --only-ec2 --no-watch > "${refusal_output}" 2>&1; then
  fail 'ec2-only stop accepted a running remote container'
fi
assert_contains 'refusing EC2 stop' "${refusal_output}"
assert_not_contains 'ec2 stop-instances' "${MOCK_LOG}"
pass 'ec2-only stop refuses running remote container'

reset_mocks running
MOCK_CONTAINER_STATE=missing MOCK_CONTAINER_LEADING_BLANK=true \
  run_controller stop --only-ec2 --no-watch > /dev/null
assert_contains 'ec2 stop-instances' "${MOCK_LOG}"
pass 'ec2-only stop accepts a blank-prefixed missing container state'

reset_mocks running
run_controller stop --no-watch > /dev/null
backup_line="$(line_number 'nethernode backup-server' "${MOCK_LOG}")"
stop_server_line="$(line_number 'nethernode stop --no-backup' "${MOCK_LOG}")"
stop_ec2_line="$(line_number 'ec2 stop-instances' "${MOCK_LOG}")"
[[ -n "${backup_line}" && -n "${stop_server_line}" && -n "${stop_ec2_line}" ]] \
  || fail 'normal stop missed backup, server stop, or EC2 stop'
(( backup_line < stop_server_line && stop_server_line < stop_ec2_line )) \
  || fail 'normal stop order is not backup, server stop, EC2 stop'
pass 'normal stop backs up, stops server, then stops EC2'

if grep -E -n 'GIT_SSH|^[[:space:]]*(export[[:space:]]+)?[A-Za-z_][A-Za-z0-9_]*(PASSPHRASE|PASSWORD)[A-Za-z_]*=' \
  "${TEMPLATE}" "${CONTROLLER}"; then
  fail 'controller template or script contains forbidden SSH passphrase/password field'
fi
pass 'controller template and script contain no forbidden SSH secret fields'

printf 'All controller mock tests passed.\n'
