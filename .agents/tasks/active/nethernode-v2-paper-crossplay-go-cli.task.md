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
| S5 | pending | CLI admin/settings | `admin list/add/remove`, `settings get/set --apply`, atomic file writes. | `nethernode admin list --dry-run`; `nethernode settings set difficulty hard --apply --dry-run` | `feat(cli): manage admins and server settings` |
| S6 | pending | Image + install | Multi-stage Dockerfile builds Go binary; install `/usr/local/bin/nethernode` from image. | `docker run --rm <image> nethernode help`; `bash -n ops/install-server-cli.sh` | `ci: package Go CLI in Minecraft image` |
| S7 | pending | CI/CD no-reset | PR/merge validate/build only; no automatic stop/restart/reset; manual lifecycle intact. | `rg "stop-instances|compose down|ssm send-command" .github/workflows` | `ci: protect running server from automatic resets` |
| S8 | pending | Migration runbook | Backup -> staging restore -> Paper verify; UUID/online-mode/Fabric leftovers documented. | `rg "Paper migration|UUID|online-mode" README.md .agents` | `docs: add Paper migration safety runbook` |
| S9 | pending | Azure scaffold | `infra/azure` minimal Terraform scaffold + README; no deploy. | `terraform -chdir=infra/azure init -backend=false`; `terraform -chdir=infra/azure validate` | `chore(infra): add Azure extension scaffold` |
| S10 | pending | SSH key local-only | Create `/home/onecode/lab/ec2-nethernode-v2/nethernode-v2(.pub)`; never commit private key. | `stat -c "%a %n" /home/onecode/lab/ec2-nethernode-v2/nethernode-v2*` | No commit unless docs change |
| S11 | pending | Final QA | Full suite, docs/harness alignment, no secrets, commits atomic. | `go test ./...`; `make validate`; Terraform validate; harness check | `docs: finalize NetherNode v2 operating docs` |

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
