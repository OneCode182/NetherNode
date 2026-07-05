# AWS Options Architecture

## Selected Default

Use EC2 direct with Docker Compose.

## Why

- Minecraft needs long-lived TCP/UDP sockets and persistent world storage.
- EC2 + EBS is cheaper and simpler than Fargate + EFS/NLB for 3-5 players.
- Start/stop cost control is direct and reliable.
- SSM avoids public SSH.

## Region Default

`us-east-1` is default because prior measurements from Colombia favored it over `sa-east-1`, and it is cheaper. Re-test with all players before final production use.

## Rejected For MVP

- Lambda: cannot host live Minecraft process.
- Fargate service + NLB: viable but fixed NLB/storage overhead hurts budget.
- EKS/GameLift: overkill for private small server.
- Spot: acceptable for experiments, not main world.
