#!/usr/bin/env python3
"""hooks/no-verify-strip.py — wrapper preHook that strips --no-verify
from `git commit`. Per SPEC.md §10.3 + §10.5.2.

The wrapper at ~/.ai/bin/git invokes this with the post-strip argv
already; the script's job is to detect whether the user requested a
bypass and, depending on settings.secret_scanning.allowNoVerifyBypass,
either strip silently, warn, or pass through.

Default (allowNoVerifyBypass=false): strip silently and log to the
audit pipeline.
Override (allowNoVerifyBypass=true):  the wrapper config simply
removes this hook from preHooks (and the one-month nag fires from
elsewhere).

Self-check:
  --self-check
"""
from __future__ import annotations

import argparse
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
import _lib  # noqa: E402


def check_invocation(argv: list[str]) -> int:
    """argv is the args AFTER `git`. Returns 0 always (this is an
    advisory; the actual stripping happens in the wrapper)."""
    if not argv or argv[0] != "commit":
        return 0
    stripped = []
    bypass_seen = False
    for a in argv[1:]:
        if a in ("--no-verify", "-n"):
            bypass_seen = True
            continue
        stripped.append(a)
    if bypass_seen:
        _lib.log("`--no-verify` was present and is being stripped.")
        _lib.log("Per Common.md §3.6: the override format is non-negotiable; same principle here.")
        _lib.log("To allow bypass: set settings.secret_scanning.allowNoVerifyBypass=true (one-month nag fires).")
    return 0


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
