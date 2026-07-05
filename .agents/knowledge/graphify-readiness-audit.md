# Graphify Readiness Audit

## 2026-07-04

| Shard | Status | Notes |
|---|---|---|
| `repo-code` | planned | Build with `.agents/tools/build_graphify_focus_graphs.py`. |
| `harness-docs` | markdown fallback | Requires semantic backend for full graph. |
| `master-clean` | fallback-ready | Merge only after source shard exists. |

## Known Gaps

- Generated graph JSON is ignored by default.
- Semantic docs graph depends on configured provider key.
- Agents must use Markdown docs when graph is missing or stale.
