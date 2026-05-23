#!/usr/bin/env python3
"""hooks/destructive-terraform-guard.py — gate `terraform {destroy,apply}`
per Common.md §2.2. Opt-in via command-wrappers.toml.

Blocks (without bypass env): `terraform destroy`, `terraform apply`.
Other subcommands pass through.

The bypass env is AI_ALLOW_DESTRUCTIVE_TERRAFORM=1.

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


GUARDED = {"destroy", "apply"}
BYPASS_ENV = "AI_ALLOW_DESTRUCTIVE_TERRAFORM"


def check_invocation(argv: list[str]) -> int:
    if not argv or argv[0] not in GUARDED:
        return 0
    if os.environ.get(BYPASS_ENV) == "1":
        _lib.log(f"`terraform {argv[0]}` — bypass active ({BYPASS_ENV}=1). Logged.")
        return 0
    _lib.log(f"blocking — `terraform {argv[0]}` mutates real infrastructure.")
    _lib.log("Per Common.md §2.2 + §2.4: state what will change, name reversibility, wait for an unambiguous yes.")
    _lib.log(f"To bypass for one session only: {BYPASS_ENV}=1 terraform {argv[0]} ...")
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
