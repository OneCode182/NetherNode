# Dynamic Workflow - NetherNode V2 Paper Crossplay + Go CLI

## Authority

This workflow specializes `dynamic-workflows-claude-code.workflow.md` (leader
`Fable 5`; subagents default `Sonnet 5` `xhigh`, `Haiku` for simple tasks,
`Opus`/`max` only on documented strong technical block; never `Fable 5`
subagents). For Codex sessions use `dynamic-workflows-codex.workflow.md` as the
equivalent contract. Active V2 task:

`tasks/active/nethernode-v2-paper-crossplay-go-cli.task.md`

Markdown remains source of truth. Graphify is a router and compression layer.
If Graphify conflicts with Markdown, trust Markdown and log the conflict.

## Mandatory Mode

- Leader and subagents use `$caveman` Ultra.
- Every phase ends with `atomic-commit-helper`.
- No deploy.
- No push.
- No `terraform apply`.
- No AWS/Azure resource creation.
- No secrets printed or committed.

## Start Of Every Phase

Before touching files in each phase:

1. Read `AGENTS.md`.
2. Read `.agents/AGENTS.md`.
3. Read `.agents/env.json`.
4. Read `.prompts/orquestacion-dynamic-workflows.md`.
5. Read `.agents/workflows/dynamic-workflows-claude-code.workflow.md` (or the
   Codex contract when running under Codex).
6. Read `.agents/workflows/nethernode-step.workflow.md`.
7. Read this workflow.
8. Read `.agents/tasks/active/nethernode-v2-paper-crossplay-go-cli.task.md`.
9. Use Graphify:

```bash
python .agents/tools/build_graphify_focus_graphs.py --check
```

If `semantic_backend_available=false`, proceed with Markdown fallback and record
that fallback in the task verification log.

## Phase Loop

Each phase must close:

1. Idea + Diseno Base
2. Implementacion
3. Testeo + Verificacion
4. Evaluacion + Correcciones
5. Documentacion + Harness update
6. Commit Atomico

Failure routing:

- Test fails -> return to Implementacion.
- Design mismatch -> return to Idea + Diseno Base.
- Docs/harness stale -> return to Documentacion.
- Same failure twice -> write escalation in the task file.
- No next phase until verification passes or skip is documented.

## Commit Gate

Run before each commit:

```bash
git status --short --branch
git diff --stat
git log -n 5 --oneline --decorate
git diff --check
```

Then stage exact files and commit one atomic phase:

```bash
git add <exact-files>
git commit -m "<phase commit message>"
```

Never push.

## Subagent Rules

- Spawn subagents only for bounded, independent work.
- Prefer `explorer` for read-only audits.
- Prefer `worker` only for disjoint write scopes.
- Do not duplicate leader work.
- Do not assign blocking immediate work to subagents.
- Leader reviews subagent evidence before integrating.
- Subagents never decide final architecture.

## Global Subagent Matrix

| Role | Default phase use | Scope |
|---|---|---|
| Runtime explorer/worker | S1, S2, S8 | PaperMC, Geyser, Floodgate, ViaVersion, ViaBackwards, migration safety |
| Go CLI explorer/worker | S3, S4, S5 | `cmd/nethernode`, `internal/**`, RCON, backup, admin/settings |
| CI/CD explorer/worker | S6, S7 | Dockerfile, GHCR, GitHub Actions, install flow, no-reset policy |
| Infra explorer/worker | S9, S10 | AWS intact, Azure scaffold, local SSH key docs |
| QA explorer | Every phase | Verification commands, stale docs, no secrets, acceptance criteria |
| Docs/harness worker | Every phase | README, AGENTS, `.agents/**`, task log |

## Current Resume Point

Known task status (2026-07-06): S0-S11 done; V2 task closed with one atomic
commit per step (see task Verification Log). This workflow stays as the
contract for reruns, corrections, and future V2.x phases. Before any rerun,
audit repo evidence (`git log`, task file) instead of trusting this list.

## Phases

### S0 - Baseline + Task Harness

Goal: create V2 task plan and register it in harness.

Subagents:

- Docs/harness explorer optional.

Required work:

- Create `tasks/active/nethernode-v2-paper-crossplay-go-cli.task.md`.
- Update `tasks/_.index.md`.
- Record Graphify check and harness check.

Verification:

```bash
python .agents/tools/build_graphify_focus_graphs.py --check
python .agents/tools/check_harness.py
git diff --check
```

Commit:

```text
docs: add NetherNode v2 task plan
```

### S1 - Runtime Paper

Goal: switch default runtime from Fabric to PaperMC `26.2`.

Subagents:

- Runtime explorer checks Paper runtime vars and itzg compatibility.
- QA explorer checks stale Fabric refs.

Required work:

- Set runtime default `MINECRAFT_TYPE=PAPER`.
- Keep `MINECRAFT_VERSION=26.2`.
- Keep Java 25 image.
- Preserve `/data` volume and `online-mode=false`.
- Remove active Fabric narrative from docs/harness.

Verification:

```bash
docker compose -f compose.yaml config -q
docker build -f server/Dockerfile .
rg -n "FABRIC|Fabric" README.md .agents server compose.yaml
python .agents/tools/check_harness.py
```

Commit:

```text
feat(runtime): switch default server to Paper crossplay
```

### S2 - Plugins Crossplay

Goal: manage Geyser, Floodgate, ViaVersion, and ViaBackwards.

Subagents:

- Runtime explorer checks plugin source APIs and latest compatibility.
- QA explorer checks dry-run, manifest, docs.

Required work:

- Add managed plugin manifest.
- Add plugin sync/list command path.
- Configure Geyser for `0.0.0.0:19132`, remote Java `127.0.0.1:25565`,
  `auth-type=floodgate`.
- Document Switch BedrockConnect/GeyserConnect workaround.

Verification:

```bash
nethernode plugins sync --dry-run
nethernode plugins list
rg -n "Geyser|Floodgate|ViaVersion|ViaBackwards" README.md .agents server ops
make validate
```

Commit:

```text
feat(runtime): add managed Paper crossplay plugins
```

### S3 - Go CLI Core

Goal: create Go module and core packages.

Subagents:

- Go CLI explorer reviews package boundaries.
- QA explorer checks source files are actually staged in commit.

Required work:

- Add `go.mod`.
- Add `cmd/nethernode`.
- Add core internal packages for RCON, compose, backup, mcstatus.
- Add tests for offline behavior.

Verification:

```bash
go test ./...
go build ./cmd/nethernode
git status --short --branch
```

Commit:

```text
feat(cli): add Go nethernode core commands
```

### S4 - CLI Lifecycle

Goal: implement lifecycle commands.

Subagents:

- Go CLI explorer reviews dry-run and mutation boundaries.
- QA explorer verifies no runtime data touched by tests.

Required work:

- Implement `start`, `stop`, `restart`, `status`, `save-server`,
  `backup-server`.
- `status` aggregates Docker, RCON, mcstatus, uptime, backups, disk.
- Dry-run must avoid RCON/docker/filesystem mutations.

Verification:

```bash
go test ./...
go build ./cmd/nethernode
nethernode status --dry-run
nethernode backup-server --dry-run
```

Commit:

```text
feat(cli): add server lifecycle commands
```

### S5 - CLI Admin + Settings

Goal: implement admin and settings commands.

Subagents:

- Go CLI explorer audits existing dirty S5 work first.
- QA explorer checks admin/settings tests and atomic write behavior.

Required work:

- Implement `nethernode admin list`.
- Implement `nethernode admin add <player> [--level 4]`.
- Implement `nethernode admin remove <player>`.
- Implement `nethernode settings get <key>`.
- Implement `nethernode settings set <key> <value> [--apply]`.
- Use RCON for live `op`, `deop`, and applicable settings when online.
- Use atomic file writes for `ops.json` and `server.properties`.
- If setting requires restart, print `restart-required`.

Verification:

```bash
go test ./...
go build ./cmd/nethernode
nethernode admin list --dry-run
nethernode settings get difficulty --dry-run
nethernode settings set difficulty hard --apply --dry-run
python .agents/tools/check_harness.py
```

Commit:

```text
feat(cli): manage admins and server settings
```

### S6 - Image + Install

Goal: package Go CLI in Minecraft image and install host CLI from image.

Subagents:

- CI/CD explorer checks Dockerfile, image workflow, install script.
- QA explorer checks binary path and no Go runtime needed on EC2.

Required work:

- Make Dockerfile multi-stage.
- Build Go binary into `/usr/local/bin/nethernode`.
- Change image workflow context if needed.
- Update `ops/install-server-cli.sh` to copy binary from image.
- Keep fallback shell scripts only as fallback.

Verification:

```bash
docker build -f server/Dockerfile .
docker run --rm <built-image> nethernode help
bash -n ops/install-server-cli.sh
make validate
```

Commit:

```text
ci: package Go CLI in Minecraft image
```

### S7 - CI/CD No-Reset

Goal: guarantee PR/merge does not stop/reset running server.

Subagents:

- CI/CD explorer audits workflows for SSM, stop, restart, compose down.
- QA explorer checks no data-path mutation in automatic workflows.

Required work:

- CI validates only.
- Image workflow builds/publishes only.
- Lifecycle workflows remain manual.
- No automatic workflow mutates `/opt/nethernode/data/minecraft`.
- No automatic workflow runs `docker compose down` or `stop-instances`.

Verification:

```bash
rg -n "stop-instances|compose down|ssm send-command|docker compose restart|docker compose up" .github/workflows
go test ./...
make validate
```

Commit:

```text
ci: protect running server from automatic resets
```

### S8 - Migration Runbook

Goal: document safe Fabric-like world to Paper migration.

Subagents:

- Runtime explorer checks Paper migration risks.
- Docs/harness worker updates docs/harness.

Required work:

- Add backup -> staging restore -> Paper verify runbook.
- Document copy list: `world/`, `ops.json`, whitelist, bans, usercache,
  `server.properties`.
- Document do-not-copy list: `mods/`, Fabric loader/libs/configs.
- Document UUID and `online-mode=false` risk.
- Document rollback from backup.

Verification:

```bash
rg -n "Paper migration|UUID|online-mode|Fabric" README.md .agents docs
python .agents/tools/check_harness.py
```

Commit:

```text
docs: add Paper migration safety runbook
```

### S9 - Azure Scaffold

Goal: add Azure extension base without deploy.

Subagents:

- Infra explorer checks AWS remains unchanged.
- Infra worker may own `infra/azure/**` only.

Required work:

- Add `infra/azure` Terraform skeleton.
- Add variables for location, VM size, SSH key, ports, disk size, repo URL,
  repo branch.
- Add README mapping AWS concepts to Azure.
- No Azure secrets.
- No Azure workflow.

Verification:

```bash
terraform -chdir=infra/azure init -backend=false
terraform -chdir=infra/azure validate
terraform -chdir=infra init -backend=false
terraform -chdir=infra validate
```

Commit:

```text
chore(infra): add Azure extension scaffold
```

### S10 - SSH Key Local-Only

Goal: create local v2 SSH key materials outside repo.

Subagents:

- Infra explorer optional, read-only.

Required work:

```bash
mkdir -p /home/onecode/lab/ec2-nethernode-v2
ssh-keygen -t ed25519 \
  -f /home/onecode/lab/ec2-nethernode-v2/nethernode-v2 \
  -C "nethernode-v2" \
  -N ""
chmod 700 /home/onecode/lab/ec2-nethernode-v2
chmod 600 /home/onecode/lab/ec2-nethernode-v2/nethernode-v2
chmod 644 /home/onecode/lab/ec2-nethernode-v2/nethernode-v2.pub
```

Rules:

- Never commit private key.
- Never print private key.
- Do not import into AWS/Azure in this repo-only phase.

Verification:

```bash
stat -c "%a %n" /home/onecode/lab/ec2-nethernode-v2/nethernode-v2*
git status --short --branch
```

Commit:

- No commit unless docs/harness change.

### S11 - Final QA

Goal: prove full V2 repo-only objective.

Subagents:

- QA explorer runs acceptance matrix.
- Docs/harness explorer checks stale docs.

Required work:

- Full test suite.
- Full docs/harness alignment.
- No secrets.
- No generated binary tracked.
- Commit history atomic.
- Goal completion audit.

Verification:

```bash
git status --short --branch
go test ./...
go build ./cmd/nethernode
docker compose -f compose.yaml config -q
docker build -f server/Dockerfile .
make validate
terraform -chdir=infra init -backend=false
terraform -chdir=infra validate
terraform -chdir=infra/azure init -backend=false
terraform -chdir=infra/azure validate
python .agents/tools/check_harness.py
python -m json.tool .agents/env.json >/dev/null
git diff --check
```

Commit:

```text
docs: finalize NetherNode v2 operating docs
```

## Completion Criteria

The goal is complete only when S0-S11 are done or explicitly skipped with
evidence, all verification commands pass or have documented non-blocking skips,
and no required repo-only deliverable remains.
