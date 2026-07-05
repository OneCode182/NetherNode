# AWS Options Architecture

## Selected Default

Use one stoppable EC2 instance with Docker Compose.

## Why

- Minecraft needs long-lived TCP/UDP sockets and persistent world storage.
- EC2 + local gp3 storage is cheaper and simpler than Fargate + EFS/NLB for 3-5 players.
- Start/stop cost control is direct and reliable.
- SSM avoids public SSH.
- GHCR avoids AWS registry lock-in and keeps Azure migration simpler.

## Region Default

`us-east-1` is default because prior measurements from Colombia favored it over `sa-east-1`, and it is cheaper. Re-test with all players before final production use.

## Rejected For MVP

- Lambda: cannot host live Minecraft process.
- Fargate service + NLB: viable but fixed NLB/storage overhead hurts budget.
- ECR: not needed for MVP because GHCR is enough and more cloud-portable.
- Elastic IP: avoided because public IPv4 has hourly cost even when attached.
- EKS/GameLift: overkill for private small server.
- Spot: acceptable for experiments, not main world.

## Cost Controls

- Default `t4g.small`; scale to `t4g.medium` only when TPS/MSPT/RAM fail.
- Stop EC2 after play session.
- Keep no always-on load balancer, NAT gateway, EFS, or Elastic IP.
- Use DuckDNS/Route53 update after start instead of fixed IP.
