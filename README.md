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

- `minecraft`: Fabric-capable Minecraft Java server (`itzg/minecraft-server`).
- `worker`: tiny local workflow/metrics service for harness experiments.

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
- Worker health: `8080/tcp`

## AWS EC2 IaC

Terraform/OpenTofu lives in `infra/`.

Defaults:

- Region: `us-east-1`
- Instance: `t4g.medium`
- AMI: Amazon Linux 2023 ARM64
- Access: AWS SSM, no public SSH
- Storage: encrypted gp3 root volume
- Budget: USD 8.33/month default
- Ingress: `25565/tcp`, `19132/udp`

Validation:

```bash
terraform -chdir=infra init -backend=false
terraform -chdir=infra fmt -check -recursive
terraform -chdir=infra validate
```

No `terraform apply` in this workflow without explicit human approval.

## Ops

Scripts:

- `ops/start.sh`: starts compose after `.env` exists.
- `ops/stop-safe.sh`: RCON save + stop + compose down.
- `ops/backup.sh`: RCON save + tar backup + retention.
- `ops/restore.sh`: restore archive into world dir.
- `ops/observability.sh`: local status/metrics checks.
- `ops/dns-update.sh`: DuckDNS update with token redaction in dry-run.

## Graphify

Graphify docs and tooling live in `.agents/knowledge/` and `.agents/tools/`.

```bash
python .agents/tools/build_graphify_focus_graphs.py --check
```

Generated graph JSON stays ignored unless explicitly requested.
