# Task: NetherNode V2 Paper Crossplay + Go CLI

## Objective

Upgrade NetherNode repo-only, with no deploy, no `terraform apply`, no AWS/Azure
resource creation, no push.

Target state:

- PaperMC `26.2` as default runtime.
- Maximum practical Java + Bedrock compatibility:
  - Java clients on Windows and macOS.
  - Nintendo Switch Bedrock through Geyser/Floodgate and console DNS workaround.
  - ViaVersion for newer Java clients.
  - ViaBackwards for older Java clients.
- Go-based `nethernode` CLI as central server admin surface.
- CI/CD does not stop, restart, reset, or overwrite the running Minecraft world.
- AWS IaC remains intact.
- Azure extension scaffold exists, without deployment.

## Mandatory Mode

- Use `$caveman` Ultra for leader and subagents.
- Use `atomic-commit-helper` at the end of every step.
- Never push.
- Never deploy.
- Never run `terraform apply`.
- Never create AWS/Azure resources.

## Mandatory Step Start

Every step starts by loading:

1. `AGENTS.md`
2. `.agents/AGENTS.md`
3. `.agents/env.json`
4. `.prompts/orquestacion-dynamic-workflows.md`
5. `.agents/workflows/nethernode-step.workflow.md`
6. This task file.
7. Graphify routing docs:
   - `.agents/knowledge/graphify-operations.md`
   - `.agents/knowledge/graphify-corpus-plan.md`
   - `.agents/knowledge/graphify-readiness-audit.md`

Then run:

```bash
python .agents/tools/build_graphify_focus_graphs.py --check
```

If Graphify is not semantically available, use Markdown as source of truth and
record the fallback in this task file or `.agents/memory/mistakes.md` when it
affects execution.

## Harness Update Rule

Every step must update harness when relevant:

- This task file: status, evidence, blockers, next step.
- `.agents/memory/decisions.md`: new durable decisions.
- `.agents/memory/module-status.md`: module status changes.
- `.agents/architecture/*.md`: architecture changes.
- `.agents/memory/patterns.md`: reusable repo pattern learned.
- `.agents/memory/mistakes.md`: repeated failure or bad assumption.

Docs/harness stale means step is not closed.

## Step Loop

Each step must close this loop:

1. Idea + Diseno Base
2. Implementacion
3. Testeo + Verificacion
4. Evaluacion + Correcciones
5. Documentacion + Harness update
6. Commit Atomico with `atomic-commit-helper`

Failure routing:

- Test fails -> return to Implementacion.
- Design mismatch -> return to Idea + Diseno Base.
- Docs/harness stale -> return to Documentacion.
- Same failure twice -> write escalation in this task file before continuing.
- No next step until current step has passing verification or explicit documented skip.

Commit gate:

```bash
git status --short --branch
git diff --stat
git log -n 5 --oneline --decorate
git diff --check
```

Commit rules:

- One atomic commit per step.
- Stage exact files only.
- Commit messages in English.
- Never push.

## Orchestration

Leader:

- Reads harness, repo, CI, runtime, infra before editing.
- Creates implementation plan and commit plan before touching files.
- Integrates subagent findings.
- Makes final architecture decisions.

Subagents:

- Runtime subagent: PaperMC, Geyser, Floodgate, ViaVersion, ViaBackwards.
- Go CLI subagent: `nethernode`, RCON, backup, status, admin/settings.
- CI/CD subagent: no-reset workflows, GHCR/image build, Go build.
- Infra subagent: AWS intact, Azure scaffold.
- QA subagent: Java/Bedrock/version/migration matrix.
- Docs/harness subagent: README, AGENTS, `.agents/` consistency.

Subagents provide evidence. Leader decides.

## Cronograma

| Step | Status | Objective | Scope | Verification | Commit |
|---|---|---|---|---|---|
| S0 | done | Baseline + task harness | Create this task file, update task index, capture V2 scope. | `python .agents/tools/check_harness.py` | `docs: add NetherNode v2 task plan` |
| S1 | done | Runtime Paper | Fabric -> PaperMC `26.2`, Java25, preserve `/data`, keep `online-mode=false` initially. | `docker compose -f compose.yaml config -q`; `docker build -f server/Dockerfile .` | `feat(runtime): switch default server to Paper crossplay` |
| S2 | done | Plugins crossplay | Managed Geyser, Floodgate, ViaVersion, ViaBackwards; Geyser UDP `19132`; Floodgate auth. | `nethernode plugins sync --dry-run`; `rg "Geyser|Floodgate|ViaVersion|ViaBackwards"` | `feat(runtime): add managed Paper crossplay plugins` |
| S3 | done | Go CLI core | `go.mod`, `cmd/nethernode`, RCON client, compose runner, backup tar/gzip, mcstatus client. | `go test ./...`; `go build ./cmd/nethernode` | `feat(cli): add Go nethernode core commands` |
| S4 | done | CLI lifecycle | `start`, `stop`, `restart`, `status`, `save-server`, `backup-server`; status uses Docker/RCON/mcstatus. | `nethernode status --dry-run`; `nethernode backup-server --dry-run` | `feat(cli): add server lifecycle commands` |
| S5 | done | CLI admin/settings | `admin list/add/remove`, `settings get/set --apply`, atomic file writes. | `nethernode admin list --dry-run`; `nethernode settings set difficulty hard --apply --dry-run` | `feat(cli): manage admins and server settings` |
| S6 | done | Image + install | Multi-stage Dockerfile builds Go binary; install `/usr/local/bin/nethernode` from image. | `docker run --rm <image> nethernode help`; `bash -n ops/install-server-cli.sh` | `ci: package Go CLI in Minecraft image` |
| S7 | done | CI/CD no-reset | PR/merge validate/build only; no automatic stop/restart/reset; manual lifecycle intact. | `rg "stop-instances|compose down|ssm send-command" .github/workflows` | `ci: protect running server from automatic resets` |
| S8 | done | Migration runbook | Backup -> staging restore -> Paper verify; UUID/online-mode/Fabric leftovers documented. | `rg "Paper migration|UUID|online-mode" README.md .agents` | `docs: add Paper migration safety runbook` |
| S9 | done | Azure scaffold | `infra/azure` minimal Terraform scaffold + README; no deploy. | `terraform -chdir=infra/azure init -backend=false`; `terraform -chdir=infra/azure validate` | `chore(infra): add Azure extension scaffold` |
| S10 | done | SSH key local-only | Create `/home/onecode/lab/ec2-nethernode-v2/nethernode-v2(.pub)`; never commit private key. | `stat -c "%a %n" /home/onecode/lab/ec2-nethernode-v2/nethernode-v2*` | No commit unless docs change |
| S11 | done | Final QA | Full suite, docs/harness alignment, no secrets, commits atomic. | `go test ./...`; `make validate`; Terraform validate; harness check | `docs: finalize NetherNode v2 operating docs` |

## Runtime Design Criteria

- PaperMC `26.2` is default.
- Existing world data remains in the same volume.
- Do not delete `world/`, `ops.json`, whitelist, bans, usercache, backups, stats, or player data.
- Do not require `server/server.jar`.
- Keep Bedrock UDP `19132` open in compose and infra.
- Keep Java TCP `25565` open.
- Keep `online-mode=false` during first migration to preserve current offline UUIDs.
- Document future `online-mode=true` migration as separate risky step requiring UUID mapping.

## Crossplay Plugin Criteria

Required stack:

- PaperMC
- Geyser-Spigot
- Floodgate-Spigot
- ViaVersion
- ViaBackwards

Expected Geyser config:

- `bedrock.address=0.0.0.0`
- `bedrock.port=19132`
- `remote.address=127.0.0.1`
- `remote.port=25565`
- `remote.auth-type=floodgate`

Compatibility truth:

- ViaVersion helps newer Java clients connect to older server protocol.
- ViaBackwards helps older Java clients connect to newer server protocol.
- No promise of every historical/future version forever.
- Future Mojang protocol changes may require plugin update.
- Nintendo Switch needs BedrockConnect/GeyserConnect style DNS workaround.

## Go CLI Criteria

Public commands:

```bash
nethernode help
nethernode start
nethernode stop [--no-backup]
nethernode restart [--no-backup]
nethernode status [--host <host>] [--json]
nethernode save-server
nethernode backup-server [--retention 5]
nethernode admin list
nethernode admin add <player> [--level 4]
nethernode admin remove <player>
nethernode settings get <key>
nethernode settings set <key> <value> [--apply]
nethernode plugins sync [--dry-run]
nethernode plugins list
```

Go implements directly:

- RCON protocol.
- `save-all flush`.
- `save-off` / `save-on`.
- `op` / `deop`.
- backup tar/gzip.
- retention prune.
- `server.properties` atomic read/write.
- mcstatus.io Java/Bedrock summary.

Go may use shell/system commands only when system operation requires it:

- `docker compose`
- `docker inspect`
- `df`/disk checks if needed

Shell scripts become compatibility wrappers or fallback, not source of truth.

## CI/CD Safety Criteria

- PR and merge do not stop server.
- PR and merge do not run `docker compose down`.
- PR and merge do not run restart.
- PR and merge do not mutate `/opt/nethernode/data/minecraft`.
- Image build/publish is allowed.
- Manual `start-server.yml` and `stop-server.yml` remain lifecycle entrypoints.
- Repo sync on EC2 is allowed only through manual lifecycle/sync workflows.

## Azure Criteria

- Add extension base only.
- Do not deploy Azure.
- Do not add Azure secrets.
- Do not alter AWS default path.
- Keep cloud boundary portable:
  - Docker Compose
  - `.env`
  - persistent volume
  - `nethernode` CLI

## Final Acceptance

Done when:

- PaperMC `26.2` default.
- Geyser + Floodgate + ViaVersion + ViaBackwards managed.
- Java Windows/macOS + Bedrock Switch documented.
- Go `nethernode` root CLI.
- `nethernode status` summarizes Docker/RCON/mcstatus/players/uptime/backups/disk.
- CI/CD cannot auto stop/reset running server on PR/merge.
- AWS IaC intact.
- Azure scaffold ready.
- Harness updated in every step.
- Every step has atomic commit or documented skip.

## Evidence Sources

- Paper Docker type: https://docker-minecraft-server.readthedocs.io/en/latest/types-and-platforms/server-types/paper/
- Paper getting started: https://docs.papermc.io/paper/getting-started/
- Paper plugins: https://docs.papermc.io/paper/adding-plugins/
- Geyser setup: https://geysermc.org/wiki/geyser/setup/
- Floodgate setup: https://geysermc.org/wiki/floodgate/setup/
- Switch BedrockConnect: https://geysermc.org/wiki/geyser/using-geyser-with-consoles/
- ViaVersion/ViaBackwards: https://hangar.papermc.io/ViaVersion/ViaVersion
- mcstatus API: https://mcstatus.io/docs

## Verification Log

Append step evidence here.

### S0 - Baseline + task harness

- Created `.agents/tasks/active/nethernode-v2-paper-crossplay-go-cli.task.md`.
- Updated `.agents/tasks/_.index.md`.
- Graphify check: `graphify_available=true`, `semantic_backend_available=false`, `graphify_check_ok`.
- Harness check: `harness_ok`.

### S1 - Runtime Paper

- Graphify check: `graphify_available=true`, `semantic_backend_available=false` -> Markdown fallback.
- `server/runtime.env`: `MINECRAFT_TYPE=FABRIC` -> `PAPER`; `MINECRAFT_VERSION=26.2` kept; `online-mode=false` kept; same `/data` volume; `server.jar` untracked and unreferenced.
- Docs aligned Fabric -> Paper: README, `.env.example`, `server/Dockerfile` label, `.agents/{project,prompts,architecture,memory,agents}`, `.agents/env.json`, `.claude/workflows/*.js`.
- Dynamic workflow `nethernode-s1-verify` (2 subagents): `docker compose -f compose.yaml config -q` exit 0; `docker build -f server/Dockerfile server/` exit 0 (image built); `python .agents/tools/check_harness.py` -> `harness_ok`; QA sweep: 0 stale Fabric runtime refs left, env consistency clean.
- S2 risks captured by QA: populate `MINECRAFT_MODRINTH_PROJECTS` (or PLUGINS var) and confirm plugin install path `/data/plugins`; Floodgate key must persist on `/data`; check itzg native Geyser support before hand-rolling; ViaVersion/ViaBackwards compat tracks `MINECRAFT_VERSION` bumps.

### S2 - Plugins crossplay

- Dynamic workflow `nethernode-s2-investigate` (2 subagentes Sonnet, evidencia web/API viva):
  - itzg has NO native Geyser toggle; `MODRINTH_PROJECTS` auto-cleans removed entries; `PATCH_DEFINITIONS` exists for config patching.
  - Floodgate has no Spigot artifact on Modrinth (fabric/neoforge only) -> GeyserMC download API required.
  - Geyser latest 2.10.1 b1177 supports up to MC `26.1.x`; no `26.2` yet (Modrinth query empty; download API metadata). ViaVersion/ViaBackwards resolve `26.2` on loader `paper` (SNAPSHOT builds).
- Implemented: `server/plugins.manifest`, `ops/plugins-sync.sh` (resolve/checksum/install/prune + `--dry-run`/`--list`), `nethernode plugins sync|list` dispatch with local script fallback, installer ships `plugins-sync.sh`, Make targets, README crossplay section (incl. Switch BedrockConnect DNS workaround), Geyser config template (`auth-type=floodgate`, bedrock `0.0.0.0:19132`, remote `127.0.0.1:25565`).
- Verification: `NETHERNODE_SCRIPT_DIR=ops bash ops/nethernode plugins sync --dry-run` exit 0, resolvió los 4 jars reales (Geyser 2.10.1 b1177, Floodgate 2.2.5 b138, Via* 5.10.1-SNAPSHOT); `plugins list` offline exit 0; `make validate` exit 0; `rg "Geyser|Floodgate|ViaVersion|ViaBackwards"` matches in README/ops/server config.
- Known upstream gap (documented, not a blocker for repo-only scope): Bedrock join on `26.2` waits for Geyser `26.2` release; re-run `make plugins-sync`.

### S3 - Go CLI core

- `go.mod` (`module github.com/onecode182/nethernode`, `go 1.26`); `cmd/nethernode/main.go` scaffold; `internal/rcon`, `internal/backup`, `internal/compose`, `internal/mcstatus` packages, each with a `_test.go`.
- `go build ./cmd/nethernode`: exit 0.
- `go test ./...`: all four internal packages `ok` (rcon, backup, compose, mcstatus); `cmd/nethernode` reports `[no test files]` (scaffold only, lifecycle commands land in S4).
- Tests are offline: no live RCON socket, Docker daemon, or mcstatus.io network call required to pass.
- Harness check: `git status --short --branch` on branch `dev` still shows `cmd/`, `go.mod`, `internal/` as untracked (`??`) even though `git log` HEAD (`40c9d6a feat(cli): add Go nethernode core commands`) claims the CLI core was added — that commit only staged `.gitignore` (`/nethernode`, `cmd/nethernode/nethernode` build-artifact ignores). Source files were never `git add`ed. Logged as a mistake in `.agents/memory/mistakes.md`; actual `git add`/commit of `go.mod`, `cmd/`, `internal/` is still outstanding and blocks a truthful S3 commit-gate close.

### S4 - CLI lifecycle

- New `internal/cli` package (`config.go`, `app.go`, `lifecycle.go`, `status.go`, `dispatch.go`) implements `start`, `stop [--no-backup]`, `restart [--no-backup]`, `status [--host][--json]`, `save-server`, `backup-server [--retention N]`; `cmd/nethernode/main.go` reduced to `cli.Run(os.Args[1:], os.Stdout, os.Stderr)`.
- Config env defaults match spec: `MINECRAFT_CONTAINER_NAME=nethernode-minecraft`, `COMPOSE_FILE=compose.yaml`, `MINECRAFT_DATA_DIR=./data/minecraft`, `BACKUP_DEST=./backups`, `BACKUP_RETENTION=5`, `BACKUP_LABEL=minecraft`; RCON fixed at `127.0.0.1:${MINECRAFT_RCON_PORT:-25575}`, password from `MINECRAFT_RCON_PASSWORD` env, falling back to `.env` (or `$ENV_FILE`) file parse only when unset.
- `stop`/`restart` default to best-effort RCON `save-all flush` + backup (`internal/backup.Run`, which self-prunes) before `docker compose down`; `--no-backup` skips only the archive step, save still runs. `save-server` requires RCON success (real error on dial failure). `backup-server` sequences `save-all flush` -> `save-off` -> `save-all flush` -> archive -> `save-on` (deferred, runs even if archive fails), degrading to backup-without-pause if RCON is unreachable.
- `status` aggregates docker `ContainerRunning`, RCON `list`, mcstatus.io java/bedrock (`--host` overrides only the mcstatus lookup host, never RCON), local backup count/newest (filesystem scan), and `df -h` on the data dir (new `compose.Runner.Run` exported passthrough); every source degrades independently (`Error` field set, command still exits 0) rather than aborting the report; `--json` emits the `StatusReport` struct.
- Global `--dry-run` extracted regardless of position (`extractDryRun`); every command prints a `[dry-run]` plan and returns before any RCON dial, docker exec, or filesystem write — verified directly by test assertions (dial count 0, exec calls 0, backup dir entry count 0).
- `go build ./...`, `go vet ./...`, `gofmt -l .` (empty): all clean. `go test ./... -count=1`: all packages `ok`, fully offline (`t.TempDir`, fake `Exec`/RCON dialer/mcstatus client, no real socket/docker/network).
- Manual verification (repo root, binary built to scratchpad, not installed): `nethernode status --dry-run` exit 0, printed docker-inspect/rcon/mcstatus/backup-scan/df plan lines, no docker/network/file touch; `nethernode backup-server --dry-run` exit 0, printed save-all/save-off/save-all/archive/save-on plan, no RCON dial or archive written. `git status --short --branch` confirms only `cmd/nethernode/main.go` (modified), `internal/compose/compose.go` (modified, added `Runner.Run`), and new `internal/cli/` are touched; `data/` and world state untouched.
- Harness update: this entry, `S4` row -> `done`, `.agents/memory/module-status.md` Go CLI row updated.

### Dynamic workflow artifact

- Added `.agents/workflows/dynamic-workflows-nethernode-v2-paper-go.workflow.md`
  to orchestrate S0-S11 with Codex dynamic-workflows, Graphify, subagents,
  harness updates, verification gates, and atomic commits.
- Registered workflow in `.agents/workflows/_.index.md` and `.agents/AGENTS.md`.

### S5 - CLI admin/settings

- Graphify check at phase start: `graphify_available=true`, `semantic_backend_available=false`, `graphify_check_ok`; Markdown fallback used as source of truth.
- Implemented `nethernode admin list|add|remove` in Go:
  - `admin list` reads `ops.json` directly.
  - `admin add` runs RCON `op <player>` when live, validates `--level` range `1..4`, and falls back to atomic `ops.json` patch when RCON is unavailable.
  - `admin remove` runs RCON `deop <player>` when live and falls back to atomic `ops.json` removal when RCON is unavailable.
- Implemented `nethernode settings get|set --apply` in Go:
  - reads/writes `server.properties` atomically while preserving comments/order.
  - canonicalizes keys (`whitelist` -> `white-list`, trims/lowercases keys).
  - supports free-form values with spaces, e.g. `motd`.
  - `--apply` maps live settings to RCON where possible (`difficulty`, `white-list`).
- Added `internal/opsjson` and `internal/serverprops` packages with unit tests for offline UUID, atomic writes, parsing, canonical writes, dry-runs, and error paths.
- Verification:
  - `go test -count=1 ./internal/cli -run 'TestCmdSettings|TestCmdAdmin|TestExtract'` -> pass.
  - `go test -count=1 ./internal/opsjson ./internal/serverprops` -> pass.
  - `go test ./...` -> pass.
  - `go vet ./...` -> pass.
  - `go build ./cmd/nethernode` -> pass.
  - Dry-runs passed: `admin list`, `admin add Sirius182 --level 4`, `admin remove Sirius182`, `settings get whitelist`, `settings set whitelist true --apply`.
  - Invalid level check: `admin add Sirius182 --level 5` exits non-zero with range error.
- Design note: manual `ops.json` creation assumes current V2 migration setting `online-mode=false`; future `online-mode=true` migration requires UUID mapping runbook before using offline fallback for new admins.

### S6 - Image + install

- Graphify check at phase start: `graphify_available=true`, `semantic_backend_available=false`, `graphify_check_ok`; Markdown fallback used as source of truth.
- Subagent audit confirmed pre-fix gaps: `server/Dockerfile` was only `itzg/minecraft-server`, image workflow used `context: server`, and `ops/install-server-cli.sh` installed the legacy shell wrapper instead of the Go binary.
- Implemented multi-stage `server/Dockerfile`: `golang:1.26-alpine` builds `cmd/nethernode`; final `itzg/minecraft-server:stable-java25` includes `/usr/local/bin/nethernode`.
- Changed image workflow to use repo-root build context and trigger on `server/**`, `cmd/**`, `internal/**`, `go.mod`, `.dockerignore`, compose, env example, and workflow changes.
- Added root `.dockerignore` so root-context Docker builds exclude secrets, world data, backups, Terraform state, generated graphs, jars, logs, and local binaries.
- `ops/install-server-cli.sh` now installs scripts plus a Go CLI binary:
  - preferred: extract `/usr/local/bin/nethernode` from `NETHERNODE_CLI_IMAGE`.
  - development fallback: local `go build`.
  - last fallback: legacy `ops/nethernode` shell wrapper.
- Added Go `plugins sync|list` delegation to `plugins-sync.sh` so replacing the shell wrapper with the Go binary does not lose managed crossplay plugin commands.
- Verification:
  - `go test ./...` -> pass.
  - `go vet ./...` -> pass.
  - `go build ./cmd/nethernode` -> pass.
  - `docker build -f server/Dockerfile -t nethernode:s6 .` -> pass.
  - `docker run --rm --entrypoint nethernode nethernode:s6 help` -> pass, includes lifecycle/admin/settings/plugins commands.
  - `bash -n ops/install-server-cli.sh` -> pass.
  - image extraction smoke: `NETHERNODE_CLI_IMAGE=nethernode:s6 NETHERNODE_BIN_PATH=<tmp>/bin/nethernode NETHERNODE_SCRIPT_DIR=<tmp>/scripts bash ops/install-server-cli.sh` -> pass; installed binary and scripts mode `755`.
  - `make validate` -> pass.

### S7 - CI/CD no-reset

- Graphify check at phase start: `graphify_available=true`, `semantic_backend_available=false`, `graphify_check_ok`; Markdown fallback used as source of truth.
- Subagent audit confirmed existing PR/merge workflows were already non-mutating; only manual `start-server.yml` and `stop-server.yml` contained SSM/EC2 lifecycle commands.
- Added `ops/check-ci-no-reset.sh` to fail if any non-lifecycle workflow contains EC2 start/stop, SSM send-command, `docker compose up/down/restart`, `ops/start.sh`, `ops/stop-safe.sh`, or `/opt/nethernode/data/minecraft`.
- Wired guard into `make validate`.
- README documents policy:
  - PR/merge workflows validate/build/publish only.
  - manual lifecycle workflows are the only place for EC2/SSM/server mutation.
- Verification:
  - `bash -n ops/check-ci-no-reset.sh` -> pass.
  - `bash ops/check-ci-no-reset.sh` -> `ci_no_reset_ok`.
  - `rg "stop-instances|compose down|ssm send-command|docker compose restart|docker compose up|/opt/nethernode/data/minecraft" .github/workflows` -> matches only manual `start-server.yml`/`stop-server.yml`.
  - `make validate` -> pass.

### S8 - Migration runbook

- Graphify check at phase start: `graphify_available=true`, `semantic_backend_available=false`, `graphify_check_ok`; Markdown fallback used as source of truth.
- Subagent audit confirmed missing pre-fix coverage: README had backup/restore notes but no Fabric-like -> Paper migration procedure; architecture doc had assumptions but no runbook.
- Added README `Paper Migration Runbook`:
  - save + backup first.
  - restore backup into staging target before touching live world.
  - preserve `world/`, dimensions, `level.dat`, player data, stats, advancements, `ops.json`, whitelist/bans, `usercache.json`, `server.properties`, Paper/Geyser managed `plugins/` and `config/`.
  - treat Fabric `mods/` and Fabric-only config as rollback evidence, not active Paper runtime.
  - keep `online-mode=false` for first migration to preserve offline UUIDs; `online-mode=true` requires separate UUID mapping.
  - verify Paper/plugins/Java/Bedrock/admin/inventory/XP/position/save/restart before replacing live data.
  - rollback by restoring known-good backup with explicit `--target` and `--force`.
- Added matching `.agents/architecture/minecraft-runtime.architecture.md` safety summary.
- Verification:
  - `rg "Paper migration|UUID|online-mode|restore|rollback|mods|world/|ops.json" README.md .agents/architecture .agents/tasks/active/nethernode-v2-paper-crossplay-go-cli.task.md` -> pass.
  - `python .agents/tools/check_harness.py` -> `harness_ok`.
  - `python .agents/tools/build_graphify_focus_graphs.py --check` -> `graphify_check_ok`.
  - `git diff --check` -> pass.

### S9 - Azure scaffold

- Graphify check at phase start: `graphify_available=true`, `semantic_backend_available=false`, `graphify_check_ok`; Markdown fallback used as source of truth.
- Subagent audit confirmed pre-fix gap: no `infra/azure` directory and only high-level Azure migration note existed.
- Added validate-only `infra/azure` Terraform scaffold:
  - `versions.tf` with `azurerm`.
  - `variables.tf` for location, VM size, SSH public key, repo URL/branch, ports, disk, and ingress CIDRs.
  - `main.tf` with resource group, VNet/subnet, public IP, NSG rules for Java TCP and Bedrock UDP, optional SSH rule, NIC, and Linux VM.
  - `cloud-init.yaml` bootstraps `/opt/nethernode`, Docker, and repo clone.
  - `outputs.tf` returns resource group, VM, public IP, Java endpoint, and Bedrock endpoint.
  - `.terraform.lock.hcl` committed for reproducible provider resolution.
- Added `infra/azure/README.md` mapping AWS MVP concepts to Azure equivalents and documenting validate-only commands.
- Root README now points to `infra/azure`.
- Verification:
  - `terraform -chdir=infra/azure fmt -check` -> pass after fmt.
  - `terraform -chdir=infra/azure init -backend=false` -> pass, `azurerm v4.80.0`.
  - `terraform -chdir=infra/azure validate` -> pass.
  - `rg "infra/azure|azurerm|Azure|Standard_B2s|terraform -chdir=infra/azure" README.md .agents/architecture infra/azure .agents/tasks/active/nethernode-v2-paper-crossplay-go-cli.task.md` -> pass.

### S10 - SSH key local-only

- Created or reused local-only ED25519 key material outside the repo:
  - private key: `/home/onecode/lab/ec2-nethernode-v2/nethernode-v2`
  - public key: `/home/onecode/lab/ec2-nethernode-v2/nethernode-v2.pub`
- Permissions verified:
  - private key mode `600`
  - public key mode `644`
- Public fingerprint: `SHA256:w+KYaNn7bPfzSRluMEEccadT7iTKvEncR0XqOcdjAlY nethernode-v2 (ED25519)`.
- No private key is inside the repo; no AWS/Azure resource creation happened.

### S11 - Final QA

- Full objective audit completed against S0-S11:
  - PaperMC `26.2` default runtime already captured in S1.
  - Geyser + Floodgate + ViaVersion + ViaBackwards managed in S2.
  - Go `nethernode` root CLI covers lifecycle, status, backups, admin, settings, and plugin script delegation by S6.
  - CI/CD no-reset guard present by S7.
  - Paper migration runbook present by S8.
  - Azure extension scaffold present and validate-only by S9.
  - SSH key material created outside repo by S10.
- Verification commands all passed:
  - `python .agents/tools/build_graphify_focus_graphs.py --check`
  - `python .agents/tools/check_harness.py`
  - `go test ./...`
  - `go vet ./...`
  - `go build ./cmd/nethernode`
  - `docker compose -f compose.yaml config -q`
  - `make validate`
  - `terraform -chdir=infra fmt -check -recursive`
  - `terraform -chdir=infra init -backend=false`
  - `terraform -chdir=infra validate`
  - `terraform -chdir=infra/azure fmt -check`
  - `terraform -chdir=infra/azure init -backend=false`
  - `terraform -chdir=infra/azure validate`
  - `docker build -f server/Dockerfile -t nethernode:s11 .`
  - `docker run --rm --entrypoint nethernode nethernode:s11 help`
  - `bash ops/check-ci-no-reset.sh`
  - `rg "PaperMC|Geyser|Floodgate|ViaVersion|ViaBackwards|Paper Migration|Azure scaffold|no-reset|online-mode|UUID" README.md .agents infra/azure server ops`
- Secret/tracked-file audit:
  - `git ls-files` found no tracked `.env`, `tfstate`, `tfvars`, jars, or `/home/onecode/lab/ec2-nethernode-v2` key files.
  - Private-key/secret regex matches were limited to `.env.example` placeholders and Go test fixture strings.
  - local SSH private key mode remains `600`; public key mode remains `644`.
- Final repo state before S11 docs commit: clean branch `dev`.

### Post-S11 - Leader audit (2026-07-06)

- Leader re-ran the suite independently: `go test ./...` ok (8 packages); spec dry-runs (`status`, `backup-server`, `admin list`, `settings set difficulty hard --apply --dry-run`) exit 0; `make validate` exit 0 with `ci_no_reset_ok`; Azure `terraform validate` Success; `infra/*.tf` AWS diff empty since S2; S6 re-proven with real image build + `docker run --entrypoint nethernode ... help`; SSH key `600/644` outside repo; no key material in any commit; tracked-files secret sweep clean.
- Corrections applied by leader:
  - `dynamic-workflows-nethernode-v2-paper-go.workflow.md` now specializes `dynamic-workflows-claude-code.workflow.md` (it pointed to the Codex contract) and the stale "resume from S5" note was replaced with the closed S0-S11 state.
  - `ci.yml` gains a `go` job (vet/test/build on PR/push): Go unit tests previously ran only locally; `image.yml` covered build only. Validate/build only; no-reset intact.
- Known accepted deviation: extra commit `40c9d6a` shares the S3 subject (gitignore-only mis-commit), documented in `.agents/memory/mistakes.md`; history is local-only (never pushed), squashable before push if desired.

### E2E aux server (2026-07-07)

- Provisioned `nethernode-aux-minecraft` (`i-01e3db31b31d1d1c1`, `3.80.254.187`, c7i-flex.large, us-east-1) with the SAME Terraform module, workspace `aux` + gitignored `infra/aux.tfvars`: plan showed 6 add / 0 change / 0 destroy; dev state untouched. `nethernode-dev-minecraft` (`i-02d96de3fcb0114c7`) only ever read via describe; no lifecycle workflow dispatched.
- Branch `dev` pushed (V2 commits + fixes); CI on push = validate/build only, green (`ci` incl. new `go` job, `infra-validate`).
- SSH keys: `/home/onecode/lab/ec2-nethernode-v2/nethernode-aux-minecraft` (600) + `.pub` (644); SG opens 22 via new opt-in `enable_ssh_ingress` (default false, dev unchanged).
- Live bugs found by the E2E and fixed in repo: AL2023 lacks `docker compose` plugin (user-data now installs it); root-owned synced plugins blocked Geyser config writes (`plugins-sync.sh` now chowns to container UID); RCON not reachable from host (compose now publishes `127.0.0.1:25575`); Go RCON client pipelined a trailer packet that Paper drops (rewritten, live-verified `save-server` + `status` with `rcon: ok`).
- Aux-only divergence: Geyser 2.10.1 crashes on Paper `26.2` (incendo/cloud reflection, upstream gap) -> aux runs `MINECRAFT_VERSION=26.1.2` (local `server/runtime.env` edit on instance, fresh world). Crossplay matrix on aux: Java 26.2 client via ViaVersion, older Java via ViaBackwards, Bedrock via Geyser/Floodgate. Repo default stays `26.2`.
- Final live evidence: 4 plugins enabled; `Started Geyser on UDP port 19132`; external mcstatus `java online=true "Paper 26.1.2"`, `bedrock online=true`; `nethernode status` aggregates container/rcon/java/bedrock/backups/disk on host.

### Aux world port from dev backup (2026-07-07)

- Source: local backup `minecraft-20260707T045930Z.tar.gz` (newest, 235M) from `/home/onecode/lab/nethernode-dev-backups`; dev instance untouched.
- Chunker CLI 1.18.1 converted `world/` `JAVA_26_2 -> JAVA_26_1_2` (DataVersion 4903 -> 4790, 3.5s); warnings: unmapped WITCH + EXPERIENCE_ORB entities dropped (cosmetic).
- Hand-ported what Chunker skips: `players/` (4 players), missing `data/minecraft/*.dat` (scoreboard, wandering_trader, ...), all NBT DataVersion byte-patched to 4790; ops/whitelist/usercache/ban lists copied from backup root.
- Aux swap: fresh-world backup taken (retention 3), stop, world replaced, chown 1000, start: `Done preparing level "world"` with zero datafixer errors; seed `-343522682` (dev's pinned seed); Sirius182 op intact; external mcstatus java+bedrock online; `bluemap purge world` issued for re-render.
- Result: dev's world progress now plays on aux `26.1.2` with full Java+Bedrock crossplay; upgrade back to `26.2` is native once Geyser ships support. Procedure documented in `minecraft-runtime.architecture.md`.

### Post-V2 maintenance - SkinsRestorer (2026-07-11)

- Scope: add persistent offline-mode Java skins without changing world, player data, or backup logic.
- Decision: `skinsrestorer|modrinth|skinsrestorer|paper|latest` joins the existing managed manifest. Resolver evidence: SkinsRestorer `15.12.4` resolves for both aux `26.1.2` and repo-default `26.2`.
- Configuration: no custom YAML and no permissions plugin. SkinsRestorer registers `skinsrestorer.player` with default access for normal players; `skinsrestorer.admin` remains ungranted. Bedrock avatar handling stays Floodgate/Geyser-owned.
- First live check found the installed `/opt/nethernode/scripts/plugins-sync.sh` inferred `/opt/nethernode` instead of tracked app `/opt/nethernode/app`; `nethernode plugins list` therefore could not find the manifest. Corrected script root auto-detection before sync; no runtime/data/backups were changed.
- Required live sequence: preserve aux-local `server/runtime.env` (`MINECRAFT_VERSION=26.1.2`), pull both commits, reinstall host scripts, record backup inventory, run `nethernode plugins sync`, restart only `minecraft`, then verify `SkinsRestorer` in `/plugins`, RCON, Java/Bedrock status, and unchanged backup inventory.
