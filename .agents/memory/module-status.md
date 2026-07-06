# Module Status

| Module | Status | Notes |
|---|---|---|
| Harness | MVP scaffolded | Uses `.agents/` with indexes and protocols. |
| Runtime | V2 in progress | Paper `26.2` + managed crossplay plugins (Geyser/Floodgate/Via*) via `plugins sync`; Geyser upstream tops at `26.1.x` (2026-07-06). |
| Infra | MVP scaffolded | Terraform validates without apply. |
| Ops | MVP scaffolded | Backup/restore/observability dry-runs. |
| Go CLI | Lifecycle done (S4) | `internal/{rcon,backup,compose,mcstatus,cli}` build/vet/test offline (`go test ./...` all `ok`); `cmd/nethernode` wires `start/stop/restart/status/save-server/backup-server` via `internal/cli`, all with a global `--dry-run` that touches nothing; admin/settings/plugins land in S5. S3 untracked-source issue (see mistakes.md) still applies until commit. |
| Graphify | MVP scaffolded | `--check` works; semantic graph optional. |
