# NetherNode Product Brief

## Goal

Provision a private Minecraft Java/Fabric server for 3-5 friends in Colombia with low latency, low AWS cost, quick start/stop, backups, observability, and future Bedrock/Switch crossplay experiments.

## Users

- Java players on Windows and macOS.
- One Bedrock/Switch player, treated as compatibility-sensitive.
- Operator is a software engineer comfortable with CLI, IaC, Docker, and AWS.

## Defaults

- Region: `us-east-1` until friend latency tests prove otherwise.
- Cloud shape: stoppable EC2, not Lambda.
- Instance: Graviton `t4g.medium` baseline, `t4g.large` burst option.
- Runtime: Docker Compose with Fabric-capable Minecraft server.
- Budget target: less than USD 50 over 6 months.
- Control: CLI plus scheduled start/stop workflows.

## Non-Goals

- No always-on production cluster.
- No `terraform apply` without explicit approval.
- No claim that client-side mods auto-install for Java or Switch clients.
- No public SSH; admin access should use AWS SSM.
