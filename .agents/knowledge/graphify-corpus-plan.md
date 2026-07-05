# Graphify Corpus Plan

## `repo-code`

- Include: `server/`, `ops/`, `infra/`, `compose.yaml`, `Makefile`.
- Purpose: code and IaC relationship discovery.
- Backend: AST/code first; docs optional.

## `harness-docs`

- Include: `.agents/project`, `.agents/architecture`, `.agents/knowledge`, `.agents/memory`.
- Purpose: low-token harness navigation.
- Backend: semantic backend required for full graph.

## `master-clean`

- Include: `repo-code` plus `harness-docs` when available.
- Purpose: default broad graph.
- Fallback: use `repo-code` graph and Markdown docs.
