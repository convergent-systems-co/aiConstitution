#!/usr/bin/env python3
"""hooks/worktree-guard.py — enforce canonical worktree placement.

Implements ~/.ai/Common.md §U17 (Worktree placement). Two canonical
locations only:

  <repo>/.worktrees/<name>/   (single-repo, dies with the repo)
  ~/.ai/worktrees/<name>/     (cross-repo, persistent)

Ad-hoc placement (`../<branch>/`, `/tmp/worktree-X/`, sibling
directories of the repo, etc.) is forbidden.

Triggers on `git worktree add <path> ...`. Other `git worktree`
subcommands (list, remove, prune) are passed through.

Self-check:
  --self-check
"""
from __future__ import annotations

import argparse
import os
import subprocess
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
import _lib  # noqa: E402


def repo_root() -> str:
    try:
        result = subprocess.run(
            ["git", "rev-parse", "--show-toplevel"],
            capture_output=True, text=True, check=False,
        )
        if result.returncode == 0:
            return result.stdout.strip()
    except FileNotFoundError:
        pass
    return ""


def ai_root() -> str:
    return os.environ.get("AI_ROOT", str(Path.home() / ".ai"))


def is_canonical(path: str) -> bool:
    """A path is canonical if it sits under either
    <repo>/.worktrees/ or <AI_ROOT>/worktrees/."""
    p = Path(path).resolve()
    candidates = []
    rr = repo_root()
    if rr:
        candidates.append(Path(rr).resolve() / ".worktrees")
    candidates.append(Path(ai_root()).resolve() / "worktrees")
    for c in candidates:
        try:
            p.relative_to(c)
            return True
        except ValueError:
            continue
    return False


def check_invocation(argv: list[str]) -> int:
    """argv is the args AFTER `git`."""
    if len(argv) < 2 or argv[0] != "worktree" or argv[1] != "add":
        return 0
    # Path is the first non-flag positional arg after `add`.
    pos = [a for a in argv[2:] if not a.startswith("-")]
    if not pos:
        return 0
    target = pos[0]
    # If the path is relative, resolve relative to cwd (which is what
    # git would do).
    if not is_canonical(target):
        _lib.log(f"blocking — `git worktree add {target}` violates Common.md §U17.")
        _lib.log("Canonical worktree locations:")
        rr = repo_root()
        if rr:
            _lib.log(f"  single-repo (dies with this repo): {rr}/.worktrees/<name>/")
        _lib.log(f"  cross-repo (persistent):           {ai_root()}/worktrees/<name>/")
        _lib.log("Preferred surface: `ai worktree add <name>` (with `--global` for cross-repo).")
        return 1
    return 0


def from_claude_payload() -> int:
    import json
    raw = sys.stdin.read()
    if not raw.strip():
        return 0
    try:
        payload = json.loads(raw)
    except json.JSONDecodeError:
        return 0
    cmd = (
        payload.get("command")
        or payload.get("tool_input", {}).get("command")
        or ""
    ) if isinstance(payload, dict) else ""
    if not cmd.strip().startswith("git "):
        return 0
    return check_invocation(cmd.split()[1:])


def main(argv: list[str]) -> int:
    parser = argparse.ArgumentParser(add_help=True)
    parser.add_argument("--self-check", action="store_true")
    parser.add_argument("--mode", choices=["claude", "wrapper"], default=None)
    parser.add_argument("rest", nargs=argparse.REMAINDER)
    args = parser.parse_args(argv)

    if args.self_check:
        return _lib.self_check_ok()

    if args.mode == "claude" or (args.mode is None and not sys.stdin.isatty()):
        return from_claude_payload()

    return check_invocation(args.rest)


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
