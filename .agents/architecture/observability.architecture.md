# Observability Architecture

## Metrics Targets

Each target lists how it is measured today. Manual = documented procedure in
README Ops section, no automation yet.

- Client latency p95 under 130 ms from Cota/Bogota. Measure: manual, player `ping` against DuckDNS domain.
- TPS near 20. Measure: manual, requires profiling mod (e.g. spark) via `MINECRAFT_MODRINTH_PROJECTS`; not installed by default.
- MSPT p95 under 35 ms. Measure: manual, same profiling mod as TPS.
- CPU p95 under 70%. Measure: manual, CloudWatch console `CPUUtilization` (basic EC2 metrics, no agent needed).
- Disk free over 20%. Measure: automated, `ops/observability.sh` runs `df -h` on the data dir.

## Local Signals

- `docker compose ps`
- `docker compose logs`
- `ops/observability.sh`
- `nethernode status`: container state, `docker exec ... rcon-cli list`, public
  Java/Bedrock mcstatus.io probes, backups, and disk. Set
  `MINECRAFT_STATUS_HOST` to public DNS; mcstatus.io cannot query `localhost`
  from the EC2 host.
- backup archive count and size

## Cloud Signals

- CloudWatch CPU/network/status checks (basic metrics via console; no alarms or dashboards provisioned in `infra/` yet).
- Disk and memory through CloudWatch agent when installed (not installed by default).
- AWS Budget alarms at monthly cap.
- Server logs shipped only after secret review.

## Backup Rule

Save server state before stop. Keep local dry-run scripts usable without AWS credentials.
