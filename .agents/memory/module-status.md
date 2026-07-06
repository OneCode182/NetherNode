# Module Status

| Module | Status | Notes |
|---|---|---|
| Harness | MVP scaffolded | Uses `.agents/` with indexes and protocols. |
| Runtime | V2 in progress | Paper `26.2` + managed crossplay plugins (Geyser/Floodgate/Via*) via `plugins sync`; Geyser upstream tops at `26.1.x` (2026-07-06). |
| Infra | MVP scaffolded | Terraform validates without apply. |
| Ops | MVP scaffolded | Backup/restore/observability dry-runs. |
| Go CLI | Core packages done (S3) | `internal/{rcon,backup,compose,mcstatus}` build and test offline; `cmd/nethernode` scaffold only, no lifecycle subcommands yet (S4); source files untracked in git despite HEAD commit message (see mistakes.md). |
| Graphify | MVP scaffolded | `--check` works; semantic graph optional. |
