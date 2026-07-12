#!/usr/bin/env bash
# NetherNode cloud controller. Reads a deliberately small, non-secret dotenv file.
set -Eeuo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CONFIG_FILE="${NETHERNODE_CLOUD_ENV:-${SCRIPT_DIR}/nethernode.local.env}"

readonly -a CONFIG_KEYS=(
  AWS_PROFILE AWS_REGION EC2_INSTANCE_ID SSH_USER SSH_KEY_PATH REMOTE_APP_DIR
  MINECRAFT_STATUS_HOST POLL_INTERVAL_SECONDS SSH_CONNECT_TIMEOUT_SECONDS
)
declare -A PROCESS_ENV=()

if [[ -t 1 && -z "${NO_COLOR:-}" ]]; then
  C_RESET=$'\033[0m'; C_BLUE=$'\033[38;5;39m'; C_GREEN=$'\033[38;5;78m'
  C_YELLOW=$'\033[38;5;220m'; C_RED=$'\033[38;5;203m'; C_DIM=$'\033[2m'
else
  C_RESET=''; C_BLUE=''; C_GREEN=''; C_YELLOW=''; C_RED=''; C_DIM=''
fi

info() { printf '%s%s%s\n' "${C_BLUE}" "$*" "${C_RESET}"; }
ok() { printf '%s%s%s\n' "${C_GREEN}" "$*" "${C_RESET}"; }
warn() { printf '%s%s%s\n' "${C_YELLOW}" "$*" "${C_RESET}" >&2; }
die() { printf '%sError: %s%s\n' "${C_RED}" "$*" "${C_RESET}" >&2; exit 1; }

usage() {
  cat <<'EOF'
NetherNode cloud controller

Usage:
  scripts/nethernode.sh [command] [flags]

Commands:
  help                         Show help.
  status [--once]              Watch cloud/server state; --once prints one poll.
  start [--only-ec2|--only-server] [--no-watch]
  stop [--only-ec2|--only-server] [--no-watch]
  restart [--no-watch]         Backup, then restart server; starts EC2 if stopped.
  save                         Remote nethernode save-server.
  backup                       Remote nethernode backup-server.

Config:
  Default: scripts/nethernode.local.env
  Override: NETHERNODE_CLOUD_ENV=/path/to/file
  Process environment values override dotenv values. AWS_PROFILE is optional.
EOF
}

capture_process_environment() {
  local key
  for key in "${CONFIG_KEYS[@]}"; do
    if declare -p "${key}" >/dev/null 2>&1; then
      PROCESS_ENV["${key}"]=1
    fi
  done
}

known_config_key() {
  local candidate="$1" key
  for key in "${CONFIG_KEYS[@]}"; do
    [[ "${key}" == "${candidate}" ]] && return 0
  done
  return 1
}

trim() {
  local value="$1"
  value="${value#"${value%%[![:space:]]*}"}"
  value="${value%"${value##*[![:space:]]}"}"
  printf '%s' "${value}"
}

load_dotenv() {
  [[ -r "${CONFIG_FILE}" ]] || die "config missing: ${CONFIG_FILE} (copy scripts/nethernode.env.example)"

  local line line_no=0 key value
  while IFS= read -r line || [[ -n "${line}" ]]; do
    ((line_no += 1))
    line="${line%$'\r'}"
    [[ -z "$(trim "${line}")" || "${line}" =~ ^[[:space:]]*# ]] && continue

    if [[ ! "${line}" =~ ^[[:space:]]*(export[[:space:]]+)?([A-Za-z_][A-Za-z0-9_]*)[[:space:]]*=(.*)$ ]]; then
      die "invalid dotenv line ${line_no} in ${CONFIG_FILE}"
    fi
    key="${BASH_REMATCH[2]}"
    value="$(trim "${BASH_REMATCH[3]}")"
    known_config_key "${key}" || die "unsupported dotenv key ${key} on line ${line_no}"

    if [[ ( "${value}" == \"*\" && "${value}" == *\" ) || ( "${value}" == \'*\' && "${value}" == *\' ) ]]; then
      value="${value:1:${#value}-2}"
    fi

    # Never source dotenv: values remain literal data, never shell code.
    if [[ -z "${PROCESS_ENV[$key]+x}" ]]; then
      printf -v "${key}" '%s' "${value}"
    fi
  done < "${CONFIG_FILE}"
}

is_positive_integer() { [[ "$1" =~ ^[1-9][0-9]*$ ]]; }

validate_config() {
  local key
  for key in AWS_REGION EC2_INSTANCE_ID SSH_USER SSH_KEY_PATH REMOTE_APP_DIR MINECRAFT_STATUS_HOST \
    POLL_INTERVAL_SECONDS SSH_CONNECT_TIMEOUT_SECONDS; do
    [[ -n "${!key:-}" ]] || die "missing ${key} in environment or ${CONFIG_FILE}"
  done
  is_positive_integer "${POLL_INTERVAL_SECONDS}" || die "POLL_INTERVAL_SECONDS must be positive integer"
  is_positive_integer "${SSH_CONNECT_TIMEOUT_SECONDS}" || die "SSH_CONNECT_TIMEOUT_SECONDS must be positive integer"
  [[ -r "${SSH_KEY_PATH}" ]] || die "SSH_KEY_PATH not readable: ${SSH_KEY_PATH}"
  command -v aws >/dev/null 2>&1 || die "aws CLI not found"
  command -v ssh >/dev/null 2>&1 || die "ssh not found"
}

init_config() {
  capture_process_environment
  load_dotenv
  validate_config
}

aws_cmd() {
  local -a options=(--no-cli-pager --region "${AWS_REGION}")
  [[ -n "${AWS_PROFILE:-}" ]] && options+=(--profile "${AWS_PROFILE}")
  aws "${options[@]}" "$@"
}

ec2_snapshot() {
  local state ip
  read -r state ip < <(aws_cmd ec2 describe-instances --instance-ids "${EC2_INSTANCE_ID}" \
    --query 'Reservations[0].Instances[0].[State.Name,PublicIpAddress]' --output text)
  [[ -n "${state:-}" && "${state}" != "None" ]] || die "EC2 instance not found: ${EC2_INSTANCE_ID}"
  [[ "${ip:-}" == "None" ]] && ip=''
  printf '%s\t%s\n' "${state}" "${ip:-}"
}

ec2_state() {
  local state ignored
  IFS=$'\t' read -r state ignored < <(ec2_snapshot)
  printf '%s\n' "${state}"
}

shell_quote() { printf '%q' "$1"; }

remote_exec_at() {
  local ip="$1" command="$2"
  ssh -i "${SSH_KEY_PATH}" \
    -o BatchMode=yes \
    -o ConnectTimeout="${SSH_CONNECT_TIMEOUT_SECONDS}" \
    -o StrictHostKeyChecking=accept-new \
    "${SSH_USER}@${ip}" "bash -lc $(shell_quote "${command}")"
}

remote_cli_at() {
  local ip="$1"
  shift
  local command="cd -- $(shell_quote "${REMOTE_APP_DIR}") && sudo -n nethernode" arg
  for arg in "$@"; do
    command+=" $(shell_quote "${arg}")"
  done
  remote_exec_at "${ip}" "${command}"
}

CURRENT_IP=''
wait_for_ssh() {
  local state ip
  info "Waiting for SSH..." >&2
  while :; do
    IFS=$'\t' read -r state ip < <(ec2_snapshot)
    if [[ "${state}" == "running" && -n "${ip}" ]] \
      && remote_exec_at "${ip}" 'true' >/dev/null 2>&1; then
      CURRENT_IP="${ip}"
      ok "SSH ready: ${ip}" >&2
      return
    fi
    sleep "${POLL_INTERVAL_SECONDS}"
  done
}

ensure_ec2_running() {
  local state
  state="$(ec2_state)"
  [[ "${state}" == "running" ]] || die "EC2 is ${state}; start it first"
}

start_ec2() {
  local state
  state="$(ec2_state)"
  case "${state}" in
    stopped)
      info "Starting EC2..."
      aws_cmd ec2 start-instances --instance-ids "${EC2_INSTANCE_ID}" >/dev/null
      ;;
    running) info "EC2 already running." ;;
    pending) info "EC2 pending." ;;
    *) die "cannot start EC2 from state: ${state}" ;;
  esac
  info "Waiting for EC2 status checks..."
  aws_cmd ec2 wait instance-status-ok --instance-ids "${EC2_INSTANCE_ID}"
  ok "EC2 status checks passed."
}

stop_ec2() {
  info "Stopping EC2..."
  aws_cmd ec2 stop-instances --instance-ids "${EC2_INSTANCE_ID}" >/dev/null
  info "Waiting for EC2 to stop..."
  aws_cmd ec2 wait instance-stopped --instance-ids "${EC2_INSTANCE_ID}"
  ok "EC2 stopped."
}

remote_start_server() {
  remote_exec_at "${CURRENT_IP}" "cd -- $(shell_quote "${REMOTE_APP_DIR}") && sudo -n bash ops/start.sh"
}

wait_for_server_ready() {
  local attempt output

  info 'Waiting for Minecraft readiness...'
  for ((attempt = 1; attempt <= 60; attempt++)); do
    if output="$(remote_exec_at "${CURRENT_IP}" \
      "sudo -n docker exec nethernode-minecraft rcon-cli list" 2>/dev/null)"; then
      ok "Minecraft ready: ${output}"
      return
    fi
    sleep "${POLL_INTERVAL_SECONDS}"
  done
  die 'Minecraft did not become RCON-ready after 60 attempts'
}

remote_container_state() {
  local output line state=''
  output="$(remote_exec_at "${CURRENT_IP}" \
    "sudo -n docker info >/dev/null && { sudo -n docker inspect -f '{{.State.Running}}' nethernode-minecraft 2>/dev/null || printf 'missing\\n'; }")" \
    || return

  while IFS= read -r line || [[ -n "${line}" ]]; do
    line="$(trim "${line}")"
    [[ -n "${line}" ]] && state="${line}"
  done <<< "${output}"
  printf '%s\n' "${state}"
}

status_once() {
  local state ip remote_status
  if ! IFS=$'\t' read -r state ip < <(ec2_snapshot); then
    warn "AWS status unavailable."
    return 0
  fi

  printf '%sNetherNode status%s\n' "${C_BLUE}" "${C_RESET}"
  printf '%sUpdated:%s %s\n' "${C_DIM}" "${C_RESET}" "$(date '+%Y-%m-%d %H:%M:%S %Z')"
  printf 'EC2: %s\n' "${state}"
  printf 'Public IP: %s\n' "${ip:-unassigned}"

  if [[ "${state}" != 'running' || -z "${ip}" ]]; then
    printf 'Server: unavailable (EC2 not running)\n'
    return 0
  fi

  if remote_status="$(remote_cli_at "${ip}" status --host "${MINECRAFT_STATUS_HOST}" 2>&1)"; then
    printf 'Server:\n%s\n' "${remote_status}"
  else
    printf 'Server: unavailable\n%s\n' "${remote_status}"
  fi
}

restore_cursor() { [[ -t 1 ]] && printf '\033[?25h'; }

watch_status() {
  [[ -t 1 ]] || die "status watch needs terminal; use status --once"
  trap 'restore_cursor; exit 130' INT TERM
  trap 'restore_cursor' EXIT
  printf '\033[?25l'
  while :; do
    printf '\033[H\033[2J'
    status_once
    printf '%sCtrl+C exits. Refresh: %ss%s\n' "${C_DIM}" "${POLL_INTERVAL_SECONDS}" "${C_RESET}"
    sleep "${POLL_INTERVAL_SECONDS}"
  done
}

watch_after() {
  local no_watch="$1"
  (( no_watch )) && return
  watch_status
}

cmd_status() {
  case "${1:-}" in
    '') watch_status ;;
    --once) [[ $# -eq 1 ]] || die 'status accepts only --once'; status_once ;;
    *) die "unknown status option: $1" ;;
  esac
}

cmd_start() {
  local only_ec2=0 only_server=0 no_watch=0 option
  for option in "$@"; do
    case "${option}" in
      --only-ec2) only_ec2=1 ;;
      --only-server) only_server=1 ;;
      --no-watch) no_watch=1 ;;
      *) die "unknown start option: ${option}" ;;
    esac
  done
  (( only_ec2 && only_server )) && die '--only-ec2 and --only-server are mutually exclusive'

  if (( only_server )); then
    ensure_ec2_running
  else
    start_ec2
  fi

  if (( ! only_ec2 )); then
    wait_for_ssh
    info "Starting Minecraft server..."
    remote_start_server
    wait_for_server_ready
  fi
  watch_after "${no_watch}"
}

cmd_stop() {
  local only_ec2=0 only_server=0 no_watch=0 option state container_state
  for option in "$@"; do
    case "${option}" in
      --only-ec2) only_ec2=1 ;;
      --only-server) only_server=1 ;;
      --no-watch) no_watch=1 ;;
      *) die "unknown stop option: ${option}" ;;
    esac
  done
  (( only_ec2 && only_server )) && die '--only-ec2 and --only-server are mutually exclusive'

  state="$(ec2_state)"
  if [[ "${state}" == 'stopped' ]]; then
    info 'EC2 already stopped; nothing to stop.'
    watch_after "${no_watch}"
    return
  fi

  if (( only_ec2 )); then
    [[ "${state}" == 'running' ]] || die "cannot stop EC2 from state: ${state}"
    wait_for_ssh
    container_state="$(remote_container_state)" || die 'cannot inspect remote nethernode-minecraft container'
    if [[ "${container_state}" == 'true' ]]; then
      die 'refusing EC2 stop: nethernode-minecraft is running; stop server first'
    fi
    [[ "${container_state}" == 'false' || "${container_state}" == 'missing' ]] \
      || die "unexpected container state: ${container_state}"
    stop_ec2
  else
    [[ "${state}" == 'running' ]] || die "EC2 is ${state}; remote backup/stop unavailable"
    wait_for_ssh
    info 'Backing up Minecraft server...'
    remote_cli_at "${CURRENT_IP}" backup-server
    info 'Stopping Minecraft server...'
    remote_cli_at "${CURRENT_IP}" stop --no-backup
    ok 'Minecraft server stopped.'
    (( only_server )) || stop_ec2
  fi
  watch_after "${no_watch}"
}

cmd_restart() {
  local no_watch=0 option state
  for option in "$@"; do
    case "${option}" in
      --no-watch) no_watch=1 ;;
      *) die "unknown restart option: ${option}" ;;
    esac
  done

  state="$(ec2_state)"
  if [[ "${state}" == 'stopped' ]]; then
    info 'EC2 stopped; running full start.'
    if (( no_watch )); then
      cmd_start --no-watch
    else
      cmd_start
    fi
    return
  fi
  [[ "${state}" == 'running' ]] || die "cannot restart from EC2 state: ${state}"
  wait_for_ssh
  info 'Backing up Minecraft server...'
  remote_cli_at "${CURRENT_IP}" backup-server
  info 'Restarting Minecraft server...'
  remote_cli_at "${CURRENT_IP}" restart --no-backup
  wait_for_server_ready
  watch_after "${no_watch}"
}

cmd_remote_single() {
  local cli_command="$1"
  ensure_ec2_running
  wait_for_ssh
  remote_cli_at "${CURRENT_IP}" "${cli_command}"
}

interactive_menu() {
  local choice
  while :; do
    printf '%sNetherNode%s\n' "${C_BLUE}" "${C_RESET}"
    printf '1) Status\n2) Start\n3) Stop\n4) Restart\n5) Save\n6) Backup\n7) Help\nq) Quit\n'
    read -r -p 'Select: ' choice
    case "${choice}" in
      1) cmd_status --once ;;
      2) cmd_start --no-watch ;;
      3) cmd_stop --no-watch ;;
      4) cmd_restart --no-watch ;;
      5) cmd_remote_single save-server ;;
      6) cmd_remote_single backup-server ;;
      7) usage ;;
      q|Q) return ;;
      *) warn 'Choose 1-7 or q.' ;;
    esac
    printf '\n'
  done
}

main() {
  case "${1:-}" in
    help|--help|-h) usage; return ;;
  esac

  init_config
  case "${1:-}" in
    '') interactive_menu ;;
    status) shift; cmd_status "$@" ;;
    start) shift; cmd_start "$@" ;;
    stop) shift; cmd_stop "$@" ;;
    restart) shift; cmd_restart "$@" ;;
    save) shift; [[ $# -eq 0 ]] || die 'save accepts no options'; cmd_remote_single save-server ;;
    backup) shift; [[ $# -eq 0 ]] || die 'backup accepts no options'; cmd_remote_single backup-server ;;
    *) die "unknown command: $1 (run: scripts/nethernode.sh help)" ;;
  esac
}

main "$@"
