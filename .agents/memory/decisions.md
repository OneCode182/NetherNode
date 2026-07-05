# Decisions

## 2026-07-04 - Use EC2 direct for MVP

- Context: Small private Minecraft server, low cost, scheduled nights.
- Decision: Use stoppable EC2 + EBS + Docker Compose.
- Rationale: Lowest cost/complexity for persistent world and TCP/UDP.
- Consequence: Operator manages one VM; scripts handle safe stop and backup.

## 2026-07-04 - Graphify is navigation only

- Context: Harness needs low-token routing.
- Decision: Markdown docs remain authority; Graphify narrows discovery.
- Rationale: Semantic graph may be stale or missing without backend key.
- Consequence: Agents check `project/` and `architecture/` first.

## 2026-07-04 - Keep cloud architecture lean

- Context: Operator has AWS trial credits but wants minimal spend and quick manual start/stop.
- Decision: Use GHCR + one stoppable EC2 + Docker Compose; avoid ECS, Fargate, NLB, EFS, ECR, and Elastic IP for MVP.
- Rationale: Fewer AWS services lowers fixed cost and keeps later Azure migration simple.
- Consequence: GitHub workflows publish images and control EC2 through OIDC + SSM; EC2 runs only during play sessions.

## 2026-07-05 - c7i-flex.large under AWS Free Plan constraint

- Context: Account is AWS Free Plan (USD 90 credits); EC2 restricted to free-tier-eligible types. `t4g.medium` rejected at RunInstances (InvalidParameterCombination).
- Decision: Use `c7i-flex.large` (x86_64, 4 GiB) with x86_64 AL2023 AMI via new `ami_architecture` variable; keep repo default `t4g.small`/arm64. Reuse pre-existing account GitHub OIDC provider via `github_oidc_provider_arn` tfvar.
- Rationale: 4 GiB matches original t4g.medium intent; GHCR image is currently amd64-only, so x86 avoids a multi-arch rebuild. ~USD 0.085/h, within credits.
- Consequence: Higher hourly cost than Graviton; revisit t4g after account upgrade + multi-arch image build.
