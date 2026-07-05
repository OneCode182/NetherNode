# Task: Bootstrap NetherNode

## Objective

Create NetherNode harness, repo foundation, runtime, IaC, ops, verification, and atomic commits.

## Shared Task List

| ID | Status | Priority | Owner | Scope | Files | Deliverable | Dependencies | Validation |
|---|---|---|---|---|---|---|---|---|
| S0 | done | P0 | orchestrator | Harness scaffold | `AGENTS.md`, `.agents/AGENTS.md`, `.agents/env.json`, indexes | Harness MVP | none | `python .agents/tools/check_harness.py` |
| S1 | done | P0 | architecture | Project truth | `.agents/project/**`, `.agents/architecture/**`, `.agents/memory/**` | Product/architecture baseline | S0 | Link/path check |
| S2 | done | P0 | orchestrator | Step workflow | `.agents/workflows/**`, `.agents/protocols/**`, task board | Failure loop + commit gate | S0 | Harness check |
| S3 | done | P1 | graphify | Graphify layer | `.graphifyignore`, `.agents/knowledge/**`, `.agents/tools/build_graphify_focus_graphs.py` | Graphify check/fallback | S1 | `python .agents/tools/build_graphify_focus_graphs.py --check` |
| S4 | done | P0 | runtime | Repo foundation | `.editorconfig`, `.gitignore`, `Makefile`, `.github/**`, dirs | Repo ready for runtime/IaC | S0 | `make help` |
| S5 | done | P0 | minecraft | Local runtime | `compose.yaml`, `.env.example`, `server/**` | Local Fabric-capable runtime | S4 | Compose config when Docker exists |
| S6 | done | P0 | infra | EC2 IaC | `infra/**`, infra CI | Terraform validates; no apply | S4 | `terraform validate` when available |
| S7 | done | P1 | ops | Ops/runbooks | `ops/**`, README | Backup/restore/observability | S5 | shell syntax + dry-runs |

## Step Loop

Every row used:

1. Idea + Diseno Base
2. Implementacion
3. Testeo + Verificacion
4. Evaluacion + Correcciones
5. Documentacion
6. Commit Atomico

## Verification Notes

- Generated `.env`, graph outputs, caches, and staging dirs are ignored.
- AWS plan/apply skipped until credentials and explicit approval exist.
- Semantic Graphify graph may be skipped without provider key; Markdown fallback remains authoritative.
