# NetherNode

Ephemeral Minecraft Java/Fabric server platform on AWS, with containers, IaC, backups, observability, dynamic workflow harness, and low-cost start/stop operations.

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

- `minecraft`: single Fabric-capable Minecraft Java server.

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
- Bedrock/Geyser future path: `19132/udp`

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
every push/PR; `image.yml` validates the Docker build when `server/` or compose
files change; `infra-validate.yml` validates Terraform when `infra/` changes.
On push to `main`, `.github/workflows/image.yml` publishes the Minecraft image
to GHCR.

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
- `ops/install-server-cli.sh`: installs `/usr/local/bin/nethernode` and
  `/opt/nethernode/scripts/*` on the EC2 host.
- `ops/nethernode`: installed CLI wrapper for `nethernode save-server` and
  `nethernode backup-server`.
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

## Graphify

Graphify docs and tooling live in `.agents/knowledge/` and `.agents/tools/`.

```bash
python .agents/tools/build_graphify_focus_graphs.py --check
```

Generated graph JSON stays ignored unless explicitly requested.
