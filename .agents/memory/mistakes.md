# Mistakes

- Do not expose SSH for EC2; use SSM.
- Do not commit generated `.env`, `graphify-out`, cache, or world data.
- Do not claim Switch support until Geyser path is tested.
- Do not use web ports as default Minecraft security group ingress.
- Do not commit a `feat: ...` message for a commit that only stages `.gitignore`/unrelated files: HEAD `40c9d6a feat(cli): add Go nethernode core commands` (2026-07-06) only added `.gitignore` ignore lines, leaving `go.mod`, `cmd/`, `internal/` untracked. Always run `git status --short` + `git diff --stat` right before commit and confirm the intended source files are actually staged, not just ignore-file edits.
- AL2023's `docker` package has no compose plugin; any bootstrap that calls `docker compose` must install the plugin first (fixed in `infra/user-data.tftpl`, 2026-07-07).
- Do not pipeline RCON packets to Minecraft: a trailer EXECCOMMAND sent before reading the response makes Paper close the connection (EOF). Detect end-of-response by short (<4096) fragment instead (fixed in `internal/rcon`, live-verified).
- Root-run plugin sync must chown results to the container UID (1000); root-owned `plugins/` blocked Geyser from writing its own config and it disabled itself.
- Geyser 2.10.1 does not run on Paper 26.2 (cloud/reflection crash on enable): verify Geyser's supported MC version before bumping `MINECRAFT_VERSION`.
