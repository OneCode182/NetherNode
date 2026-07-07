# NetherNode

Ephemeral Minecraft Paper crossplay server platform on AWS, with containers, IaC, backups, observability, dynamic workflow harness, and low-cost start/stop operations.

## Agent Harness

Agents start here:

1. `AGENTS.md`
2. `.agents/AGENTS.md`
3. `.agents/env.json`
4. `.prompts/orquestacion-dynamic-workflows.md`
5. `.agents/workflows/init-session.workflow.md`

Step work must follow `.agents/workflows/nethernode-step.workflow.md`:

1. Idea + Diseno Base
2. Implementacion
3. Testeo + Verificacion
4. Evaluacion + Correcciones
5. Documentacion
6. Commit Atomico

## Local Runtime

Services:

- `minecraft`: single PaperMC server (Java TCP `25565`, Bedrock UDP `19132` reserved for Geyser crossplay).

The repo builds a thin wrapper image from `server/Dockerfile`, based on
`itzg/minecraft-server:stable-java25`. Runtime defaults live in
`server/runtime.env`; local secrets and operator choices stay in `.env`.

Setup:

```bash
cp .env.example .env
sed -i 's/MINECRAFT_EULA=FALSE/MINECRAFT_EULA=TRUE/' .env
make up
```

Useful commands:

```bash
make help
make status
make logs
make stop-safe
make backup-dry-run
make observability
```

Ports:

- Java: `25565/tcp`
- Bedrock/Geyser: `19132/udp`

## Crossplay

Managed Paper plugin stack, declared in `server/plugins.manifest` and synced by
`nethernode plugins sync` (`ops/plugins-sync.sh`):

- Geyser-Spigot: Bedrock clients join over UDP `19132`.
- Floodgate-Spigot: Bedrock players join without a Java account; its signing
  key persists at `/data/plugins/floodgate/key.pem`.
- ViaVersion: newer Java clients on an older server protocol.
- ViaBackwards: older Java clients on a newer server protocol.
- TAB + PlaceholderAPI (Player expansion): view-only player info for everyone
  — tab list shows `❤ health Lv xp | (x, y, z) | ping ms` per player, and a
  bare 5-segment health bar (20% per segment) floats above each head in
  proximity (template: `server/config/tab/config.yml`).

```bash
make plugins-sync-dry-run   # resolve versions, print plan
make plugins-sync           # download jars into data/minecraft/plugins
make plugins-list           # offline manifest + installed jars
```

Geyser config template (`server/config/geyser/config.yml`) is installed only
when missing: `bedrock 0.0.0.0:19132`, `remote 127.0.0.1:25565`,
`auth-type: floodgate`.

Compatibility notes:

- Nintendo Switch cannot add servers directly; players set console Primary DNS
  to a BedrockConnect address and pick the server from its menu
  (see https://geysermc.org/wiki/geyser/using-geyser-with-consoles/).
- Via plugins do not promise every historical/future version; Mojang protocol
  changes can require plugin updates.
- As of 2026-07-06 Geyser/Floodgate publish support up to MC `26.1.x`; Bedrock
  join on `26.2` works once Geyser ships `26.2` support (re-run
  `make plugins-sync`). ViaVersion/ViaBackwards already resolve `26.2`
  (snapshot builds).

## Lean AWS Runtime

Terraform/OpenTofu lives in `infra/`.

Defaults:

- Region: `us-east-1`
- Instance: `t4g.small` cost-first, upgrade to `t4g.medium` only if metrics fail
- AMI: Amazon Linux 2023 ARM64
- Access: AWS SSM, no public SSH
- Storage: encrypted 20 GiB gp3 root volume
- Budget: USD 8.33/month default
- Ingress: `25565/tcp`, `19132/udp`
- Runtime path: `/opt/nethernode/app`
- World path: `/opt/nethernode/data/minecraft`
- Backups path: `/opt/nethernode/backups`

Validation:

```bash
terraform -chdir=infra init -backend=false
terraform -chdir=infra fmt -check -recursive
terraform -chdir=infra validate
```

No `terraform apply` in this workflow without explicit human approval.

## GitHub CI/CD

Validation is split by path filters: `ci.yml` validates compose and scripts on
every push/PR; `image.yml` validates the Docker build when `server/`, `cmd/`,
`internal/`, `go.mod`, or compose files change; `infra-validate.yml` validates
Terraform when `infra/` changes. On push to `master`,
`.github/workflows/image.yml` publishes the Minecraft image
to GHCR.

The image is built from the repo root so it can package the Go CLI into the
runtime image:

```bash
docker build -f server/Dockerfile -t nethernode:local .
docker run --rm --entrypoint nethernode nethernode:local help
```

To install the exact CLI binary from an image on the host:

```bash
sudo NETHERNODE_CLI_IMAGE=ghcr.io/<owner>/nethernode:latest \
  bash ops/install-server-cli.sh
```

Manual cloud controls:

- `.github/workflows/start-server.yml`: starts EC2, waits for SSM, pulls repo,
  deploys `ghcr.io/<owner>/<repo>:latest`, starts Minecraft, optionally updates
  DuckDNS.
- `.github/workflows/stop-server.yml`: runs safe stop, creates backup, stops EC2.

Required GitHub variables:

- `AWS_REGION`
- `AWS_ROLE_ARN`
- `EC2_INSTANCE_ID`
- `MINECRAFT_EULA=TRUE` after accepting the Minecraft EULA

Optional DuckDNS settings:

- Variable: `DUCKDNS_DOMAIN`
- Secret: `DUCKDNS_TOKEN`

GHCR package should be public for the EC2 host to pull without storing GitHub
tokens on the instance.

No-reset policy:

- PR and merge workflows (`ci.yml`, `image.yml`, `infra-validate.yml`) only
  validate/build/publish. They must not start, stop, restart, or mutate the EC2
  Minecraft runtime or `/opt/nethernode/data/minecraft`.
- Manual lifecycle workflows (`start-server.yml`, `stop-server.yml`) are the
  only workflows allowed to send SSM commands or start/stop EC2.
- `ops/check-ci-no-reset.sh` enforces this split in `make validate`.

## Cost Model

Target: under USD 30 over 6 months (hard ceiling USD 50) against USD 90 AWS
credits. The USD 8.33/month budget alarm derives from the ceiling.

The design avoids ECS, Fargate, NLB, EFS, ECR, and Elastic IP for the MVP.
Costs are dominated by running EC2 hours, public IPv4 hours while running, and
small gp3 storage.

- `t4g.small`: use first when AWS trial/free eligibility allows it.
- `t4g.medium`: fallback only if TPS/MSPT/RAM metrics require it.
- Public IPv4: charged only while the instance is running when no Elastic IP is
  allocated.
- Always-on is explicitly out of scope.

## Ops

Scripts:

- `ops/sync-runtime-env.sh`: syncs versioned Minecraft defaults into `.env`.
- `ops/start.sh`: syncs runtime env, pulls image, starts compose, updates DuckDNS when configured.
- `ops/stop-safe.sh`: RCON save + backup + stop + compose down.
- `ops/backup.sh`: RCON save + tar backup + retention.
- `ops/save-server.sh`: force-save full world/player state with RCON.
- `ops/backup-server.sh`: force-save, create backup archive, keep newest 5 backups.
- `ops/install-server-cli.sh`: installs the Go `/usr/local/bin/nethernode`
  CLI and `/opt/nethernode/scripts/*` on the EC2 host.
- `ops/nethernode`: legacy shell wrapper kept as a local/fallback helper.
- `ops/restore.sh`: restore archive into world dir.
- `ops/observability.sh`: local status/metrics checks (container, RCON players,
  stats, disk free vs the >20% target, backup count/sizes).
- `ops/dns-update.sh`: DuckDNS update with token redaction in dry-run.

### World Save And Backup Safety

`rcon-cli save-all flush` sends Minecraft's `save-all flush` command through
RCON. It forces the running server to write the current world and player state
to disk before the next operation continues. This includes chunks, player
position, inventory, XP, stats, advancements, `level.dat`, and runtime files
persisted under `/opt/nethernode/data/minecraft`.

This command does not create a backup by itself. In NetherNode, `ops/backup.sh`
uses it first, then creates a `.tar.gz` archive in `/opt/nethernode/backups`.
`ops/stop-safe.sh` uses the safer shutdown sequence: `save-all flush`, backup,
server stop, then `docker compose down`.

The EC2 deploy workflow installs a host CLI:

```bash
nethernode save-server
nethernode backup-server
```

`nethernode save-server` only flushes world/player state to disk.
`nethernode backup-server` flushes state, pauses autosave while archiving,
creates a `.tar.gz` under `/opt/nethernode/backups`, then keeps only the newest
5 backups by default.

Run a manual world save:

```bash
sudo docker exec nethernode-minecraft rcon-cli save-all flush
```

Create a backup while keeping only the newest local backup:

```bash
cd /opt/nethernode/app

sudo BACKUP_SOURCE=/opt/nethernode/data/minecraft \
  BACKUP_DEST=/opt/nethernode/backups \
  BACKUP_RETENTION=1 \
  COMPOSE_FILE=/opt/nethernode/app/compose.yaml \
  bash ops/backup.sh
```

Safely stop the server while keeping only the newest local backup:

```bash
cd /opt/nethernode/app

sudo BACKUP_SOURCE=/opt/nethernode/data/minecraft \
  BACKUP_DEST=/opt/nethernode/backups \
  BACKUP_RETENTION=1 \
  COMPOSE_FILE=/opt/nethernode/app/compose.yaml \
  bash ops/stop-safe.sh
```

Use `BACKUP_RETENTION=1` for a low-disk policy that leaves only the latest
backup archive. Do not use `BACKUP_RETENTION=0` to mean "keep one"; in the
current script, `0` skips pruning after creating the new archive. Stopping and
starting the EC2 instance preserves the world as long as the backing EBS volume
is not deleted or the instance is not terminated with delete-on-termination
enabled.

### Paper Migration Runbook

Use this when moving an existing world into the Paper crossplay runtime or when
validating a Fabric-like world after the runtime change. Do this repo-only or on
a staging path first; do not overwrite the live world until verification passes.

1. Save and backup current world:

   ```bash
   nethernode save-server
   nethernode backup-server --retention 5
   ```

2. Restore the backup into staging, never straight over live data first:

   ```bash
   cd /opt/nethernode/app

   bash ops/restore.sh \
     --archive /opt/nethernode/backups/<backup>.tar.gz \
     --target /opt/nethernode/staging/paper-migration \
     --dry-run

   bash ops/restore.sh \
     --archive /opt/nethernode/backups/<backup>.tar.gz \
     --target /opt/nethernode/staging/paper-migration
   ```

3. Preserve these files/directories from `/opt/nethernode/data/minecraft`:
   `world/`, `world_nether/`, `world_the_end/`, `level.dat`, `playerdata/`,
   `stats/`, `advancements/`, `ops.json`, `whitelist.json`,
   `banned-players.json`, `banned-ips.json`, `usercache.json`,
   `server.properties`, `plugins/`, and `config/` when already Paper/Geyser
   managed.

4. Do not migrate active Fabric-only runtime folders as Paper features:
   `mods/`, Fabric loader/libs, and Fabric-only config. Keep them in the backup
   archive for rollback evidence, but treat them as inert under Paper.

5. Keep `online-mode=false` for the first Paper migration if the existing world
   was played in offline mode. This preserves current offline UUIDs for player
   inventory, XP, position, stats, advancements, and admin entries. Moving to
   `online-mode=true` is a separate migration requiring UUID mapping; doing it
   casually can make players appear as new users with empty inventories.

6. Verify Paper before replacing live data:

   ```bash
   docker compose -f compose.yaml config -q
   nethernode plugins list
   nethernode status --dry-run
   rg -n "online-mode|level-name|white-list" /opt/nethernode/staging/paper-migration/server.properties
   ```

   Then perform a real smoke test on a disposable/staging host: Java client
   join, Bedrock/Geyser join when upstream version support exists, admin
   command check, inventory/XP/position check, and short play/save/restart check.

7. Roll back by stopping the server and restoring the known-good backup into the
   live target only after confirming the target:

   ```bash
   cd /opt/nethernode/app

   bash ops/restore.sh \
     --archive /opt/nethernode/backups/<known-good>.tar.gz \
     --target /opt/nethernode/data/minecraft \
     --force
   ```

Manual checks without a script:

- Latency (target: p95 under 130 ms from Cota/Bogota): each player runs
  `ping -c 20 <duckdns-domain>` while the server is up and reports p95/max.
- TPS/MSPT (targets: TPS ~20, MSPT p95 under 35 ms): not automated yet; add a
  profiling mod (e.g. `spark` via `MINECRAFT_MODRINTH_PROJECTS`) and query it
  over RCON when tuning is needed.
- CPU p95 (target: under 70%): use the CloudWatch console `CPUUtilization`
  metric for the instance (basic 5-minute EC2 metrics, no extra infra).

## Azure Migration Note

The app boundary is intentionally portable: Docker Compose + env file + volume.
To migrate later, keep `server/`, `compose.yaml`, and `ops/` mostly unchanged;
replace only `infra/` and GitHub AWS role usage with Azure VM/IAM equivalents.

An Azure extension scaffold lives in `infra/azure/`. It is validate-only in the
V2 workflow and must not be applied without explicit approval:

```bash
terraform -chdir=infra/azure init -backend=false
terraform -chdir=infra/azure validate
```

## Graphify

Graphify docs and tooling live in `.agents/knowledge/` and `.agents/tools/`.

```bash
python .agents/tools/build_graphify_focus_graphs.py --check
```

Generated graph JSON stays ignored unless explicitly requested.
