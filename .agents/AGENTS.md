# NetherNode Agent Harness

> Entry point for agents working in NetherNode.

## Boot Sequence

1. Read root `AGENTS.md`.
2. Read `.agents/env.json`.
3. Read `.prompts/orquestacion-dynamic-workflows.md`.
4. Read `workflows/init-session.workflow.md`.
5. Load only needed `_.index.md` files.
6. For every implementation step, use `workflows/nethernode-step.workflow.md`.
7. For failures, use `protocols/verification-retry.protocol.md`.
8. For commits, use `protocols/atomic-commit.protocol.md`.

## Authority Order

1. Current human instruction.
2. `.prompts/orquestacion-dynamic-workflows.md`.
3. `workflows/`.
4. `protocols/`.
5. `project/` for product intent.
6. `architecture/` for system design.
7. `knowledge/` for Graphify and curated technical notes.
8. `tasks/` and `sessions/` for active state.
9. `memory/` for durable lessons.

## Folder Map

| Folder | Role |
|---|---|
| `agents/` | Agent role specs. |
| `workflows/` | Session and step lifecycle. |
| `protocols/` | Verification, context, quality, commits. |
| `prompts/` | Reusable prompt contracts for audits and agent handoffs. |
| `project/` | Product truth. |
| `architecture/` | Runtime, AWS, observability design. |
| `knowledge/` | Graphify ops and corpus plan. |
| `memory/` | Decisions, patterns, mistakes, module status. |
| `tasks/` | Active task boards. |
| `sessions/` | Handoffs and snapshots. |
| `skills/` | Skill references. |
| `tools/` | Harness utilities. |
| `graphify-builds/` | Graph shards and build logs. |
| `graphify-out/` | Root graph aliases when present. |

## Graphify Router

| Question | Start |
|---|---|
| Product goal or scope | `project/product-brief.md` |
| Claude Code orchestration | `workflows/dynamic-workflows-claude-code.workflow.md` |
| Codex orchestration | `workflows/dynamic-workflows-codex.workflow.md` |
| AWS choice or cost/latency | `architecture/aws-options.architecture.md` |
| Minecraft/Fabric/runtime | `architecture/minecraft-runtime.architecture.md` |
| Metrics/alerts/backups | `architecture/observability.architecture.md` |
| Full quality audit | `prompts/nethernode-quality-audit.prompt.md` |
| Code relationship | `knowledge/graphify-operations.md`, then smallest graph shard |

Graphify is navigation only. If Graphify conflicts with Markdown, trust Markdown and log the conflict.
