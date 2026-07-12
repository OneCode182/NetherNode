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
| S4 | done | Create atomic commits, push, sync auxiliary checkout with Git, and verify CI without restarting the server or changing persistent data. | Commits `26b530e` and `27348e0` pushed to `dev`; CI run `29182500797` passed; auxiliary checkout fast-forwarded and integrity evidence recorded below. |

Each step: design -> implementation -> verification -> correction -> harness
update. On failure, return to implementation. Each closed step requires recorded
verification evidence.

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
- 2026-07-12: feature commit `26b530e` and docs commit `27348e0` pushed to
  `dev`.
- 2026-07-12: CI run `29182500797` succeeded: `windows-controller` PowerShell
  and BAT checks, `validate`, and Go checks passed.
- 2026-07-12: auxiliary checkout fast-forwarded from `8849683` to `27348e0`;
  local override `server/runtime.env` was preserved.
- 2026-07-12: container ID
  `d9ee3ab4527fc6363dbb362a800e000b2256cc150dc60792fef4afd8334a2fe6`
  and `StartedAt=2026-07-12T05:08:59.19985823Z` were unchanged after sync;
  `running=true`, proving no container restart.
- 2026-07-12: backup inventory SHA256 remained
  `1f8e567e3ed7ebd43a76ecb117d3b804d1270a24e394356f3a741f3f30a98c1a`.
- 2026-07-12: world `level.dat` remained size `471` with mtime `1783837185`
  before and after sync.
- 2026-07-12: remote Java and Bedrock status online, `0/5` players, 5 backups.
