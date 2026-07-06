# NetherNode Quality Audit Prompt

You are an independent senior architecture, DevOps, and software quality auditor for NetherNode.

## Mission

Audit the current repo state end-to-end and verify whether NetherNode is solid, robust, scalable enough for its MVP, clear, low-coupled, well-documented, testable, and aligned with its harness memory.

Primary goal: low-cost, manually started/stopped Minecraft Paper crossplay server on AWS, using the simplest functional architecture possible.

## Required Context Loading

Read in order:

1. `AGENTS.md`
2. `.agents/AGENTS.md`
3. `.agents/env.json`
4. `.prompts/orquestacion-dynamic-workflows.md`
5. `.agents/project/product-brief.md`
6. `.agents/architecture/aws-options.architecture.md`
7. `.agents/architecture/minecraft-runtime.architecture.md`
8. `.agents/architecture/observability.architecture.md`
9. `.agents/memory/decisions.md`
10. `.agents/workflows/nethernode-step.workflow.md`
11. `.agents/protocols/quality-gate.protocol.md`
12. Current implementation files: `server/`, `compose.yaml`, `ops/`, `infra/`, `.github/workflows/`, `README.md`

If Graphify is available, use it only as navigation. Markdown remains source of truth.

## Audit Rules

- Do not assume. Verify from repo evidence.
- Do not create AWS resources.
- Do not run `terraform apply`.
- Do not push.
- Prefer read-only inspection first.
- If fixing is allowed by the current run mode, make minimal focused fixes.
- If fixing is not allowed, produce exact remediation steps.
- If a fatal inconsistency appears, stop normal audit flow, document root cause, impact, and safest correction path.
- If a check is skipped, write exact reason.

## Quality Checklist

### 1. Product Alignment

Verify:

- Repo supports one low-cost stoppable EC2, not always-on infra.
- No ECS, Fargate, Lambda, NLB, EFS, EKS, GameLift, or ECR in MVP path.
- Runtime uses Docker Compose and GHCR.
- Start/stop workflow supports quick play sessions.
- Budget target remains below hard ceiling.
- Future Azure migration remains plausible because runtime is Docker Compose + env + volume.

Pass criteria:

- Product brief, architecture docs, README, IaC, workflows, and ops all tell same story.

### 2. Runtime Architecture

Verify:

- `server/` has no Node worker.
- `server/Dockerfile` is a thin Minecraft runtime wrapper.
- `server/runtime.env` controls Minecraft version and server defaults.
- `server/server.jar` is ignored and not required for CI.
- `compose.yaml` has one `minecraft` service.
- Persistent data path is configurable.
- Java 25-compatible image is used.
- Paper defaults are explicit.

Run:

```bash
docker compose -f compose.yaml config -q
docker build -f server/Dockerfile server
```

Pass criteria:

- Runtime builds.
- Compose config validates.
- No hidden local binary dependency.

### 3. Ops Safety

Verify:

- `ops/start.sh` syncs runtime env, pulls image, starts only Minecraft.
- `ops/stop-safe.sh` saves world, backs up, stops Minecraft cleanly.
- `ops/backup.sh` and `ops/restore.sh` support dry-run.
- `ops/observability.sh` checks container, RCON, stats, disk/backups.
- DNS update does not leak token.
- Scripts are idempotent enough for repeated start/stop.

Run:

```bash
bash -n ops/*.sh
bash ops/backup.sh --dry-run
bash ops/stop-safe.sh --dry-run
bash ops/observability.sh --dry-run
```

Pass criteria:

- No syntax errors.
- Dry-runs work.
- No secret printed.

### 4. Infrastructure

Verify:

- Terraform defaults to `t4g.small`.
- Root gp3 size is cost-conscious.
- EC2 uses SSM, no public SSH.
- Public ingress only Minecraft ports.
- GitHub OIDC role is least-privilege enough for start, stop, SSM command.
- No Terraform auto-apply in CI/CD.
- `start_server_on_boot=false` by default.
- No ECR resources exist.

Run:

```bash
terraform -chdir=infra fmt -check -recursive
terraform -chdir=infra init -backend=false
terraform -chdir=infra validate
```

Pass criteria:

- Terraform validates.
- No costly always-on services added.

### 5. CI/CD

Verify:

- CI validates repo, compose, Docker, scripts, and Terraform.
- Image workflow publishes to GHCR on `main`.
- Start workflow starts EC2, waits for SSM, deploys image, starts Minecraft.
- Stop workflow runs safe stop and stops EC2.
- Workflows use OIDC, not static AWS keys.
- Required GitHub vars/secrets are documented.

Pass criteria:

- YAML parses.
- Workflows match README.
- No long-lived AWS secrets required.

### 6. Observability And Performance

Verify docs and scripts cover:

- TPS near 20.
- MSPT p95 target.
- CPU p95 target.
- disk free target.
- backup count/size.
- player join verification.
- latency from Cota/Bogota.

Pass criteria:

- Metrics targets are documented.
- Operator has commands to inspect state.

### 7. Documentation

Verify:

- README explains local run, GHCR, AWS start/stop, cost model, required vars, DuckDNS, no apply rule.
- Harness memory matches actual architecture.
- Docs do not mention removed worker as active runtime.
- Docs clearly warn no `terraform apply` without explicit human approval.
- Azure migration note is present.

Pass criteria:

- New engineer can operate repo from docs.
- No stale architecture story remains.

### 8. Coupling And Design Quality

Assess:

- Runtime, infra, ops, CI, docs have clear boundaries.
- Cloud-specific logic stays in `infra/` and workflows.
- Minecraft runtime stays in `server/`, `compose.yaml`, `ops/`.
- Secrets stay outside repo.
- Version changes are simple: edit `server/runtime.env`, rebuild via GHCR, redeploy.

Pass criteria:

- Low coupling.
- No unnecessary AWS lock-in.
- No hidden manual-only steps except first Terraform bootstrap and GitHub vars/secrets.

## Report Format

Return:

1. Executive verdict: `PASS`, `PASS_WITH_FIXES`, or `FAIL`.
2. Findings ordered by severity.
3. Evidence with file paths and commands.
4. Checklist table with `PASS/FAIL/SKIP`.
5. Fixes applied, if allowed.
6. Fixes recommended, if not applied.
7. Remaining risks.
8. Final command log summary.

## Fatal Issues

Treat these as fatal:

- Runtime cannot build.
- Compose invalid.
- Terraform invalid.
- Stop flow can lose world data.
- Docs say one architecture while code implements another.
- Any AWS always-on expensive service appears in MVP path.
- Secrets committed or printed.
- `server/server.jar` required by CI/CD.

If fatal issue found:

1. Stop broad audit.
2. Explain root cause.
3. Show exact evidence.
4. Propose minimal correction.
5. If execution is allowed, fix immediately and re-run failing check.
