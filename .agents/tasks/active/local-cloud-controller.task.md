# Task: Local Cloud Controller

## Objective

Implement and verify manual Linux/fish and Windows control of existing
NetherNode EC2, then complete auxiliary Git sync and CI verification.

## Files

- `README.md`
- `AGENTS.md`
- `.agents/architecture/observability.architecture.md`
- `.agents/memory/{decisions,module-status,patterns}.md`
- `scripts/{nethernode.sh,nethernode.ps1,nethernode.bat,nethernode.env.example}`
- `scripts/tests/nethernode-controller-test.sh`

## Step Loop

| Step | Status | Work | Evidence |
|---|---|---|---|
| S0 | done | Inspect controller, config, ignore rule, and repo boundaries. | Graphify graph missing; Markdown fallback used. |
| S1 | done | Document command UX, auth/key ownership, SSH boundary, stop safety, IP/domain behavior, and data boundaries. | Owned docs + memory updated. |
| S2 | done | Verify controller behavior with mock AWS/SSH commands. | `make controller-test` passed 7 checks. |
| S3 | done | Run local, harness, Graphify, CI safety, IaC, and read-only auxiliary verification. | All local gates passed; live status and data-integrity evidence recorded below. |
| S4 | pending | Create atomic commits, push, sync auxiliary checkout with Git, and verify CI. | Commit/push/sync/CI evidence pending. |

Each step: design -> implementation -> verification -> correction -> harness
update. On failure, return to implementation. Do not close S4 without recorded
commit, push, auxiliary sync, and CI results.

## Acceptance

- Bash works directly from fish as `./scripts/nethernode.sh`; Windows supports
  PowerShell and BAT launchers.
- Local config comes from ignored `scripts/nethernode.local.env`, copied from
  `scripts/nethernode.env.example`; AWS CLI auth and OpenSSH are explicit.
- No Git/SSH passphrase storage. Controller does no Git; OS SSH agent owns key
  unlock.
- `status/start/stop/restart/save/backup` and flags documented: `status` watch
  with `Ctrl+C`, `--once`, `--only-ec2`, `--only-server`, `--no-watch`.
- Stop sequence is exact: `nethernode backup-server` -> `nethernode stop
  --no-backup` -> EC2 stop. `--only-ec2` refuses when container runs. Never
  terminate.
- Public IP is queried from AWS per poll; remote status owns domain behavior.
- SSH is restricted auxiliary convenience; canonical CI/CD remains OIDC + SSM.
- Sync/pull never touches `/opt/nethernode/data` or `/opt/nethernode/backups`.

## Current Evidence

- 2026-07-12: `make controller-test` passed all 7 checks.
- 2026-07-12: `make validate`, `go test ./...`, `go vet ./...`,
  `go build ./...`, harness check, YAML parse, and `ci_no_reset` all passed.
- 2026-07-12: Graphify check passed with Markdown fallback.
- 2026-07-12: AWS infra validation passed when excluding ignored, unformatted
  local-only `aux.tfvars`; recursive format check reported that local file.
  Azure validation passed.
- 2026-07-12: live read-only auxiliary status passed: EC2 `running` at
  `34.231.129.179`; Java Paper `26.1.2`; Bedrock `26.33`; `0/5` players; 5
  backups.
- 2026-07-12: before/after backup inventory SHA256 was identical:
  `1f8e567e3ed7ebd43a76ecb117d3b804d1270a24e394356f3a741f3f30a98c1a`.
  World `level.dat` size and mtime were unchanged.
- S4 pending: atomic commits, push, auxiliary Git sync, and CI verification.
