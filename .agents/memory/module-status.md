# Module Status

| Module | Status | Notes |
|---|---|---|
| Harness | MVP scaffolded | Uses `.agents/` with indexes and protocols. |
| Runtime | V2 in progress | Paper `26.2` + managed crossplay plugins (Geyser/Floodgate/Via*) via `plugins sync`; Geyser upstream tops at `26.1.x` (2026-07-06). |
| Infra | MVP scaffolded | Terraform validates without apply. |
| Ops | MVP scaffolded | Backup/restore/observability dry-runs. |
| Go CLI | Admin/settings done (S5) | `internal/{rcon,backup,compose,mcstatus,cli,opsjson,serverprops}` build/vet/test offline (`go test ./...`, `go vet ./...`, `go build ./cmd/nethernode` all ok); `cmd/nethernode` wires lifecycle plus `admin list/add/remove` and `settings get/set --apply`; all mutating paths support `--dry-run`; plugin management remains shell-wrapper until later Go port. |
| Graphify | MVP scaffolded | `--check` works; semantic graph optional. |
