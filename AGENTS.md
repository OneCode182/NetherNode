# NetherNode Agents Entry

Read this file first. Then load `.agents/AGENTS.md`, `.agents/env.json`, and
`.prompts/orquestacion-dynamic-workflows.md`.

## Repo Contract

- `.agents/` owns harness memory, agents, protocols, tasks, sessions, and Graphify ops.
- `server/`, `ops/`, `infra/`, and `.github/` own implementation.
- Graphify narrows navigation; Markdown docs remain source of truth.
- `terraform apply` and any AWS resource creation need explicit human approval.

## Local Cloud Controller

- `scripts/nethernode.sh` is Bash and may be called directly from fish;
  Windows uses `scripts/nethernode.ps1` or `scripts/nethernode.bat`.
- Controller config is copied from `scripts/nethernode.env.example` to ignored
  `scripts/nethernode.local.env`. Require AWS CLI authentication, OpenSSH, and
  an OS SSH agent with the private key unlocked.
- Never store Git or SSH key passphrases. Controller performs no Git work; key
  passphrase ownership remains with `ssh-agent` or Windows OpenSSH agent.
- SSH is temporary local-operator convenience. CI/CD authority remains GitHub
  OIDC + SSM. SSH must be enabled and security-group restricted before use.
- Stop order is `backup-server` -> `stop --no-backup` -> EC2 stop. Never
  terminate EC2; `--only-ec2` must refuse while the container runs.
- Remote sync/pull work must never mutate `/opt/nethernode/data` or
  `/opt/nethernode/backups`.

## Runtime Save And Backup Rules

- Before backup or shutdown, preserve Minecraft state with
  `rcon-cli save-all flush`.
- Treat `save-all flush` as a disk flush, not a backup. Backups are the
  `.tar.gz` archives created by `ops/backup.sh`.
- Use `ops/stop-safe.sh` for shutdowns; expected order is save, backup, stop
  Minecraft, then `docker compose down`.
- On deployed EC2 hosts, prefer `nethernode save-server` for manual saves and
  `nethernode backup-server` for save + backup + retention.
- Built-in Paper plugins are copied from the runtime image's `/plugins` path
  into the persistent `/data/plugins` path on startup. They may store their
  own state there, but must never mutate world data or backup archives.
- For low-disk EC2 operation, use `BACKUP_RETENTION=1` to keep only the newest
  local backup. Do not use `BACKUP_RETENTION=0` to mean "keep one" with the
  current script.
- Stop/start of EC2 preserves the world only while the EBS volume remains
  attached and undeleted. Termination can destroy the world if the root volume
  is configured with delete-on-termination.

## Required Flow

1. Read `.prompts/orquestacion-dynamic-workflows.md`.
2. Read `.agents/workflows/init-session.workflow.md`.
3. For each step, follow `.agents/workflows/nethernode-step.workflow.md`.
4. On failure, follow `.agents/protocols/verification-retry.protocol.md`.
5. For commits, follow `.agents/protocols/atomic-commit.protocol.md`.
