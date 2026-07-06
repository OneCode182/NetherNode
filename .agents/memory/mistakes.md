# Mistakes

- Do not expose SSH for EC2; use SSM.
- Do not commit generated `.env`, `graphify-out`, cache, or world data.
- Do not claim Switch support until Geyser path is tested.
- Do not use web ports as default Minecraft security group ingress.
- Do not commit a `feat: ...` message for a commit that only stages `.gitignore`/unrelated files: HEAD `40c9d6a feat(cli): add Go nethernode core commands` (2026-07-06) only added `.gitignore` ignore lines, leaving `go.mod`, `cmd/`, `internal/` untracked. Always run `git status --short` + `git diff --stat` right before commit and confirm the intended source files are actually staged, not just ignore-file edits.
