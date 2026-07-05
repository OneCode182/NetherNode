#!/usr/bin/env python3
"""Validate NetherNode harness structure."""

from __future__ import annotations

import json
import sys
from pathlib import Path


ROOT = Path(__file__).resolve().parents[2]

REQUIRED_FILES = (
    "AGENTS.md",
    ".graphifyignore",
    ".agents/AGENTS.md",
    ".agents/env.json",
    ".agents/agents/_.index.md",
    ".agents/agents/orchestrator.agent.md",
    ".agents/agents/infra-agent.agent.md",
    ".agents/agents/minecraft-agent.agent.md",
    ".agents/agents/qa-agent.agent.md",
    ".agents/agents/git-agent.agent.md",
    ".agents/workflows/_.index.md",
    ".agents/workflows/init-session.workflow.md",
    ".agents/workflows/nethernode-step.workflow.md",
    ".agents/protocols/_.index.md",
    ".agents/protocols/context-loading.protocol.md",
    ".agents/protocols/quality-gate.protocol.md",
    ".agents/protocols/verification-retry.protocol.md",
    ".agents/protocols/decision-log.protocol.md",
    ".agents/protocols/atomic-commit.protocol.md",
    ".agents/project/_.index.md",
    ".agents/project/product-brief.md",
    ".agents/architecture/_.index.md",
    ".agents/architecture/aws-options.architecture.md",
    ".agents/architecture/minecraft-runtime.architecture.md",
    ".agents/architecture/observability.architecture.md",
    ".agents/knowledge/_.index.md",
    ".agents/knowledge/graphify-operations.md",
    ".agents/knowledge/graphify-corpus-plan.md",
    ".agents/knowledge/graphify-readiness-audit.md",
    ".agents/memory/_.index.md",
    ".agents/memory/decisions.md",
    ".agents/memory/patterns.md",
    ".agents/memory/mistakes.md",
    ".agents/memory/module-status.md",
    ".agents/tasks/_.index.md",
    ".agents/tasks/active/bootstrap-nethernode.task.md",
    ".agents/sessions/_.index.md",
    ".agents/skills/_.index.md",
    ".agents/tools/_.index.md",
    ".agents/tools/check_harness.py",
    ".agents/tools/build_graphify_focus_graphs.py",
    ".agents/graphify-builds/_.index.md",
    ".agents/graphify-out/_.index.md",
)

REQUIRED_INDEX_DIRS = (
    ".agents/agents",
    ".agents/workflows",
    ".agents/protocols",
    ".agents/project",
    ".agents/architecture",
    ".agents/knowledge",
    ".agents/memory",
    ".agents/tasks",
    ".agents/sessions",
    ".agents/skills",
    ".agents/tools",
    ".agents/graphify-builds",
    ".agents/graphify-out",
)


def main() -> int:
    missing = [path for path in REQUIRED_FILES if not (ROOT / path).is_file()]
    missing_indexes = [path for path in REQUIRED_INDEX_DIRS if not (ROOT / path / "_.index.md").is_file()]

    try:
        json.loads((ROOT / ".agents/env.json").read_text(encoding="utf-8"))
    except Exception as exc:  # noqa: BLE001
        print(f"env.json invalid: {exc}", file=sys.stderr)
        return 1

    if missing or missing_indexes:
        for path in missing:
            print(f"missing file: {path}", file=sys.stderr)
        for path in missing_indexes:
            print(f"missing index: {path}/_.index.md", file=sys.stderr)
        return 1

    print("harness_ok")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
