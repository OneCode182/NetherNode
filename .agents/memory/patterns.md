# Patterns

- Load `AGENTS.md` -> `.agents/env.json` -> orchestration prompt before edits.
- Keep each step in one atomic commit.
- Use dry-run modes for ops scripts before mutating data.
- Prefer config in `.env.example`; keep real `.env` untracked.
- Bundle first-party Paper plugin jars in image `/plugins`; let itzg sync into persistent `/data/plugins` so plugin state stays in normal backups and never joins world data.
- Local cloud controller pattern: read current EC2 IP from AWS on every poll; keep SSH local-only convenience, CI/CD on OIDC + SSM, key unlock in OS SSH agents, and persistent `/opt/nethernode/{data,backups}` outside sync/pull scope.
