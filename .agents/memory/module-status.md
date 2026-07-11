# Module Status

| Module | Status | Notes |
|---|---|---|
| Harness | V2 task closed (S11) | Uses `.agents/` with indexes, protocols, dynamic workflow, task evidence, Graphify fallback, and S0-S11 verification log. |
| Runtime | Paper crossplay + skins managed | Paper `26.2` + Geyser/Floodgate/Via*/SkinsRestorer via `plugins sync`; Geyser upstream tops at `26.1.x` (2026-07-06), while SkinsRestorer resolves for `26.1.2` and `26.2`. README/architecture document backup -> staging restore -> Paper verify -> rollback, `online-mode=false`/UUID safety, and default player skin permissions. |
| Infra | Azure scaffold done (S9) | AWS Terraform remains default path and validates without apply; `infra/azure` adds validate-only Azure VM/network scaffold with Docker Compose runtime parity and no CI/CD/deploy integration. |
| Ops | CI no-reset guard done (S7) | Backup/restore/observability dry-runs; `ops/check-ci-no-reset.sh` enforces that non-lifecycle GitHub workflows do not start/stop/restart EC2 or mutate `/opt/nethernode/data/minecraft`. |
| Go CLI | Status hardening live-verified | `internal/{rcon,backup,compose,mcstatus,cli,opsjson,serverprops}` build/vet/test offline; status uses public `MINECRAFT_STATUS_HOST`, container `rcon-cli` first, TCP fallback, and terminal-aware color. Aux verified Java `Paper 26.1.2`, Bedrock `26.33`, RCON/player list, disk, and five unchanged backups. `ops/install-server-cli.sh` uses local Go, image extract, or Docker Go builder so EC2 does not regress to shell wrapper. |
| Graphify | MVP scaffolded | `--check` works; semantic graph optional. |
