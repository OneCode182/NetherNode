#!/usr/bin/env python3
"""Build focused Graphify graphs for NetherNode."""

from __future__ import annotations

import argparse
import os
import shutil
import subprocess
import sys
from datetime import datetime, timezone
from pathlib import Path


ROOT = Path(__file__).resolve().parents[2]
BUILDS = ROOT / ".agents" / "graphify-builds"
STAGING = ROOT / "graphify-staging"
LOG = BUILDS / "build-log.md"

SEMANTIC_KEYS = (
    "GEMINI_API_KEY",
    "GOOGLE_API_KEY",
    "OPENAI_API_KEY",
    "ANTHROPIC_API_KEY",
    "DEEPSEEK_API_KEY",
)


def log(message: str) -> None:
    BUILDS.mkdir(parents=True, exist_ok=True)
    stamp = datetime.now(timezone.utc).isoformat()
    with LOG.open("a", encoding="utf-8") as file:
        file.write(f"\n## {stamp}\n\n{message.rstrip()}\n")


def run(cmd: list[str]) -> subprocess.CompletedProcess[str]:
    return subprocess.run(cmd, cwd=ROOT, text=True, stdout=subprocess.PIPE, stderr=subprocess.STDOUT, check=False)


def reset(path: Path) -> None:
    if path.exists():
        shutil.rmtree(path)
    path.mkdir(parents=True, exist_ok=True)


def copy(src: Path, dst: Path) -> None:
    if src.is_dir():
        shutil.copytree(
            src,
            dst,
            ignore=shutil.ignore_patterns(".git", ".terraform", "node_modules", "data", "backups", "tmp", "graphify-out"),
            dirs_exist_ok=True,
        )
    elif src.exists():
        dst.parent.mkdir(parents=True, exist_ok=True)
        shutil.copy2(src, dst)


def semantic_backend_available() -> bool:
    return any(os.environ.get(key) for key in SEMANTIC_KEYS)


def graphify_available() -> bool:
    return shutil.which("graphify") is not None


def check() -> int:
    required = [
        ROOT / ".agents/knowledge/graphify-operations.md",
        ROOT / ".agents/knowledge/graphify-corpus-plan.md",
        ROOT / ".agents/knowledge/graphify-readiness-audit.md",
        ROOT / ".graphifyignore",
    ]
    missing = [str(path.relative_to(ROOT)) for path in required if not path.exists()]
    if missing:
        print("missing graphify files: " + ", ".join(missing), file=sys.stderr)
        return 1
    print(f"graphify_available={str(graphify_available()).lower()}")
    print(f"semantic_backend_available={str(semantic_backend_available()).lower()}")
    print("graphify_check_ok")
    return 0


def extract(input_dir: Path, out_dir: Path) -> bool:
    if not graphify_available():
        log("SKIP graphify extract\n\n- blocker: graphify command not found")
        print("graphify command not found; install graphifyy or use --check", file=sys.stderr)
        return False
    result = run(["graphify", "extract", str(input_dir), "--out", str(out_dir)])
    graph = out_dir / "graphify-out" / "graph.json"
    ok = result.returncode == 0 and graph.exists()
    status = "SUCCESS" if ok else "FAIL"
    log(
        f"{status} graphify extract\n\n"
        f"- input: `{input_dir}`\n"
        f"- output: `{graph}`\n"
        f"- returncode: {result.returncode}\n"
        f"- stdout:\n\n```text\n{result.stdout[-3000:]}\n```"
    )
    return ok


def build_repo_code() -> bool:
    build = BUILDS / "repo-code"
    reset(build)
    stage = STAGING / "repo-code" / "input"
    reset(stage)
    for rel in ("server", "ops", "infra", "compose.yaml", "Makefile"):
        copy(ROOT / rel, stage / rel)
    return extract(stage, build)


def build_harness_docs() -> bool:
    build = BUILDS / "harness-docs"
    reset(build)
    if not semantic_backend_available():
        marker = build / "HARNESS_DOCS_MARKDOWN_FALLBACK.md"
        marker.write_text(
            "# Harness Docs Markdown Fallback\n\nNo semantic backend available. Use `.agents/project`, `.agents/architecture`, `.agents/knowledge`, and `.agents/memory` directly.\n",
            encoding="utf-8",
        )
        log("SKIP harness-docs\n\n- blocker: no semantic backend env var")
        return False
    stage = STAGING / "harness-docs" / "input"
    reset(stage)
    for rel in (".agents/project", ".agents/architecture", ".agents/knowledge", ".agents/memory"):
        copy(ROOT / rel, stage / rel)
    return extract(stage, build)


def build_masters(include_harness: bool) -> bool:
    repo_graph = BUILDS / "repo-code" / "graphify-out" / "graph.json"
    harness_graph = BUILDS / "harness-docs" / "graphify-out" / "graph.json"
    master = BUILDS / "master-clean"
    reset(master)
    if not repo_graph.exists():
        marker = master / "MASTER_CLEAN_MARKDOWN_FALLBACK.md"
        marker.write_text("# Master Clean Fallback\n\nNo repo graph exists yet. Use Markdown and source files directly.\n", encoding="utf-8")
        log("SKIP master-clean\n\n- blocker: repo-code graph missing")
        return False
    if include_harness and harness_graph.exists() and graphify_available():
        result = run(["graphify", "merge-graphs", str(repo_graph), str(harness_graph), "--out", str(master / "graphify-out/graph.json")])
        ok = result.returncode == 0
        log(f"{'SUCCESS' if ok else 'FAIL'} master-clean merge\n\n```text\n{result.stdout[-3000:]}\n```")
        return ok
    shutil.copytree(repo_graph.parent, master / "graphify-out", dirs_exist_ok=True)
    (master / "MASTER_CLEAN_MARKDOWN_FALLBACK.md").write_text(
        "# Master Clean Markdown Fallback\n\nThis graph currently uses `repo-code` only. Harness docs remain authoritative Markdown.\n",
        encoding="utf-8",
    )
    log("SUCCESS master-clean fallback\n\n- source: repo-code")
    return True


def main() -> int:
    parser = argparse.ArgumentParser(description="Build focused Graphify graphs for NetherNode.")
    parser.add_argument("--check", action="store_true", help="Validate Graphify harness config without building graphs.")
    parser.add_argument("--only", choices=["repo-code", "harness-docs", "masters"], help="Build one shard only.")
    args = parser.parse_args()

    if args.check:
        return check()

    BUILDS.mkdir(parents=True, exist_ok=True)
    repo_ok = build_repo_code() if args.only in (None, "repo-code") else (BUILDS / "repo-code/graphify-out/graph.json").exists()
    harness_ok = build_harness_docs() if args.only in (None, "harness-docs") else (BUILDS / "harness-docs/graphify-out/graph.json").exists()
    if args.only in (None, "masters"):
        build_masters(include_harness=harness_ok)

    if args.only == "harness-docs":
        return 0 if harness_ok else 2
    if args.only == "repo-code":
        return 0 if repo_ok else 2
    return 0 if repo_ok else 2


if __name__ == "__main__":
    raise SystemExit(main())
