#!/usr/bin/env python3
"""hooks/destructive-kubectl-guard.py — gate destructive `kubectl`
operations per Common.md §2.2. Opt-in via command-wrappers.toml.

Blocks (without bypass env): `kubectl delete`, `kubectl drain`,
`kubectl cordon`. Other subcommands pass through.

The bypass env is AI_ALLOW_DESTRUCTIVE_KUBECTL=1.

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


GUARDED = {"delete", "drain", "cordon"}
BYPASS_ENV = "AI_ALLOW_DESTRUCTIVE_KUBECTL"


def check_invocation(argv: list[str]) -> int:
    if not argv or argv[0] not in GUARDED:
        return 0
    if os.environ.get(BYPASS_ENV) == "1":
        _lib.log(f"`kubectl {argv[0]}` — bypass active ({BYPASS_ENV}=1). Logged.")
        return 0
    _lib.log(f"blocking — `kubectl {argv[0]}` mutates cluster state.")
    _lib.log("Per Common.md §2.2 + §2.4: state what will change, name reversibility, wait for an unambiguous yes.")
    _lib.log(f"To bypass for one session only: {BYPASS_ENV}=1 kubectl {argv[0]} ...")
    return 1


def main(argv: list[str]) -> int:
    parser = argparse.ArgumentParser(add_help=True)
    parser.add_argument("--self-check", action="store_true")
    parser.add_argument("rest", nargs=argparse.REMAINDER)
    args = parser.parse_args(argv)
    if args.self_check:
        return _lib.self_check_ok()
    return check_invocation(args.rest)


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
