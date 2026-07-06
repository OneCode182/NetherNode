# NetherNode Agents Entry

Read this file first. Then load `.agents/AGENTS.md`, `.agents/env.json`, and
`.prompts/orquestacion-dynamic-workflows.md`.

## Repo Contract

- `.agents/` owns harness memory, agents, protocols, tasks, sessions, and Graphify ops.
- `server/`, `ops/`, `infra/`, and `.github/` own implementation.
- Graphify narrows navigation; Markdown docs remain source of truth.
- `terraform apply` and any AWS resource creation need explicit human approval.

## Runtime Save And Backup Rules

- Before backup or shutdown, preserve Minecraft state with
  `rcon-cli save-all flush`.
- Treat `save-all flush` as a disk flush, not a backup. Backups are the
  `.tar.gz` archives created by `ops/backup.sh`.
- Use `ops/stop-safe.sh` for shutdowns; expected order is save, backup, stop
  Minecraft, then `docker compose down`.
- On deployed EC2 hosts, prefer `nethernode save-server` for manual saves and
  `nethernode backup-server` for save + backup + retention.
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
