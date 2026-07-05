# Observability Architecture

## Metrics Targets

- Client latency p95 under 130 ms from Cota/Bogota.
- TPS near 20.
- MSPT p95 under 35 ms.
- CPU p95 under 70%.
- Disk free over 20%.

## Local Signals

- `docker compose ps`
- `docker compose logs`
- `ops/observability.sh`
- backup archive count and size

## Cloud Signals

- CloudWatch CPU/network/status checks.
- Disk and memory through CloudWatch agent when installed.
- AWS Budget alarms at monthly cap.
- Server logs shipped only after secret review.

## Backup Rule

Save server state before stop. Keep local dry-run scripts usable without AWS credentials.
