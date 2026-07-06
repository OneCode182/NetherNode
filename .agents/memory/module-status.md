# Module Status

| Module | Status | Notes |
|---|---|---|
| Harness | MVP scaffolded | Uses `.agents/` with indexes and protocols. |
| Runtime | V2 in progress | Paper `26.2` + managed crossplay plugins (Geyser/Floodgate/Via*) via `plugins sync`; Geyser upstream tops at `26.1.x` (2026-07-06). |
| Infra | MVP scaffolded | Terraform validates without apply. |
| Ops | CI no-reset guard done (S7) | Backup/restore/observability dry-runs; `ops/check-ci-no-reset.sh` enforces that non-lifecycle GitHub workflows do not start/stop/restart EC2 or mutate `/opt/nethernode/data/minecraft`. |
| Go CLI | Packaged in image (S6) | `internal/{rcon,backup,compose,mcstatus,cli,opsjson,serverprops}` build/vet/test offline; `cmd/nethernode` wires lifecycle, admin/settings, and plugin script delegation; image `server/Dockerfile` packages the Go binary at `/usr/local/bin/nethernode`; `ops/install-server-cli.sh` can extract that binary from `NETHERNODE_CLI_IMAGE`. |
| Graphify | MVP scaffolded | `--check` works; semantic graph optional. |
