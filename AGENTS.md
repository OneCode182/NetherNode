# NetherNode Agents Entry

Read this file first. Then load `.agents/AGENTS.md`, `.agents/env.json`, and
`.prompts/orquestacion-dynamic-workflows.md`.

## Repo Contract

- `.agents/` owns harness memory, agents, protocols, tasks, sessions, and Graphify ops.
- `server/`, `ops/`, `infra/`, and `.github/` own implementation.
- Graphify narrows navigation; Markdown docs remain source of truth.
- `terraform apply` and any AWS resource creation need explicit human approval.

## Required Flow

1. Read `.prompts/orquestacion-dynamic-workflows.md`.
2. Read `.agents/workflows/init-session.workflow.md`.
3. For each step, follow `.agents/workflows/nethernode-step.workflow.md`.
4. On failure, follow `.agents/protocols/verification-retry.protocol.md`.
5. For commits, follow `.agents/protocols/atomic-commit.protocol.md`.
