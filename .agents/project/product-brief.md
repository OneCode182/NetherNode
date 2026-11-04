# NetherNode Product Brief

## Goal

Provision a private Minecraft Paper crossplay server (Java + Bedrock/Switch via Geyser) for 3-5 friends in Colombia with low latency, low AWS cost, quick start/stop, backups, and observability.

## Users

- Java players on Windows and macOS.
- One Bedrock/Switch player, treated as compatibility-sensitive.
- Operator is a software engineer comfortable with CLI, IaC, Docker, and AWS.

## Defaults

- Region: `us-east-1` until friend latency tests prove otherwise.
- Cloud shape: stoppable EC2, not Lambda.
- Instance: Graviton `t4g.small` cost-first; `t4g.medium` only after metrics prove need.
- Runtime: Docker Compose with a single PaperMC service.
- AWS credits: USD 90 over 6 months.
- Budget target: spend as little as possible, ideally under USD 30 over 6 months; USD 50 is the hard ceiling.
- Control: GitHub manual start/stop workflows plus local CLI.
- Registry: GHCR for portability; no ECR in MVP.

## Non-Goals

- No always-on production cluster.
- No ECS, Fargate, NLB, EFS, Lambda, EKS, GameLift, or ECR for MVP.
- No `terraform apply` without explicit approval.
- No claim that client-side mods auto-install for Java or Switch clients.
- No public SSH; admin access should use AWS SSM.
