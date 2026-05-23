#!/usr/bin/env python3
"""hooks/destructive-gh-guard.py — gate destructive `gh` operations
per Common.md §2.2.

Blocks (without --force-i-mean-it confirmation):

  gh repo delete <repo>
  gh release delete <tag>
  gh secret delete <name>
  gh auth logout

These are the high-blast-radius / irreversible operations on the
default `gh` surface. Other subcommands pass through.

Self-check:
  --self-check
"""
from __future__ import annotations

import argparse
import os
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
import _lib  # noqa: E402


GUARDED = [
    ("repo", "delete"),
    ("release", "delete"),
    ("secret", "delete"),
    ("auth", "logout"),
]

# Escape hatch: the user can set AI_ALLOW_DESTRUCTIVE_GH=1 for a single
# session to bypass these checks. Setting this is itself a §2.2 action
# and is logged.
BYPASS_ENV = "AI_ALLOW_DESTRUCTIVE_GH"


def check_invocation(argv: list[str]) -> int:
    """argv is the args AFTER `gh`."""
    if len(argv) < 2:
        return 0
    a, b = argv[0], argv[1]
    if (a, b) not in GUARDED:
        return 0

    if os.environ.get(BYPASS_ENV) == "1":
        _lib.log(f"`gh {a} {b}` — bypass active ({BYPASS_ENV}=1). Logged.")
        return 0

    _lib.log(f"blocking — `gh {a} {b}` is a §2.2 destructive operation.")
    _lib.log("Per Common.md §2.2 + §2.4: name what will be destroyed, snapshot if reversible, wait for an unambiguous yes.")
    _lib.log(f"To bypass for one session only: AI_ALLOW_DESTRUCTIVE_GH=1 gh {a} {b} ...")
    return 1


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
    if not cmd.strip().startswith("gh "):
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
