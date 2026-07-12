# Patterns

- Load `AGENTS.md` -> `.agents/env.json` -> orchestration prompt before edits.
- Keep each step in one atomic commit.
- Use dry-run modes for ops scripts before mutating data.
- Prefer config in `.env.example`; keep real `.env` untracked.
- Bundle first-party Paper plugin jars in image `/plugins`; let itzg sync into persistent `/data/plugins` so plugin state stays in normal backups and never joins world data.
