# Graphify Operations

## Purpose

Graphify narrows NetherNode navigation. It does not override Markdown truth in `project/`, `architecture/`, or `memory/`.

## Shards

| Shard | Corpus | Output |
|---|---|---|
| `repo-code` | `server/`, `ops/`, `infra/`, compose, Makefile | `.agents/graphify-builds/repo-code/graphify-out/graph.json` |
| `harness-docs` | `.agents/project`, `.agents/architecture`, `.agents/knowledge`, `.agents/memory` | `.agents/graphify-builds/harness-docs/graphify-out/graph.json` |
| `master-clean` | `repo-code` plus `harness-docs` when semantic backend exists | `.agents/graphify-builds/master-clean/graphify-out/graph.json` |

## Commands

```bash
python .agents/tools/build_graphify_focus_graphs.py --check
python .agents/tools/build_graphify_focus_graphs.py --only repo-code
python .agents/tools/build_graphify_focus_graphs.py --only harness-docs
python .agents/tools/build_graphify_focus_graphs.py --only masters
python .agents/tools/build_graphify_focus_graphs.py
```

## Semantic Backend Policy

`harness-docs` needs semantic extraction. If no `GEMINI_API_KEY`, `GOOGLE_API_KEY`, `OPENAI_API_KEY`, `ANTHROPIC_API_KEY`, or `DEEPSEEK_API_KEY` exists, write fallback markers and use Markdown directly.

## Git Policy

Commit docs, script, fallback marker docs, and indexes. Do not commit generated `graphify-out` JSON, staging dirs, or cache files.
