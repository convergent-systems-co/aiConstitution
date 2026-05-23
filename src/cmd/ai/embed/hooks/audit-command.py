#!/usr/bin/env python3
"""hooks/audit-command.py — wrapper postHook that records every wrapped
command invocation to the audit log.

Per SPEC.md §10.5.4: "If a bypass leaves no audit line where one
would normally appear, the gap itself is visible." This hook makes
the appearance side reliable; the absence side is what surfaces in
forensic review.

Inputs (from the wrapper environment):
  WRAPPED_CMD       — the underlying command ("git", "gh", etc.)
  WRAPPED_ARGV      — the argv it was invoked with, JSON-encoded
  WRAPPED_EXIT      — the exit code of the real command
  WRAPPED_DURATION  — wall-clock seconds the command took (string)

Self-check:
  --self-check
"""
from __future__ import annotations

import argparse
import datetime as dt
import json
import os
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
import _lib  # noqa: E402


def audit_dir() -> Path:
    root = os.environ.get("AI_ROOT", str(Path.home() / ".ai"))
    return Path(root) / "audit" / "interactions"


def month_file() -> Path:
    now = dt.datetime.now(dt.timezone.utc)
    return audit_dir() / f"{now.strftime('%Y-%m')}.jsonl"


def chronon() -> str:
    now = dt.datetime.now(dt.timezone.utc)
    return now.strftime("%Y-%m-%dT%H:%M:%S.") + f"{now.microsecond // 1000:03d}Z"


def main(argv: list[str]) -> int:
    parser = argparse.ArgumentParser(add_help=True)
    parser.add_argument("--self-check", action="store_true")
    args = parser.parse_args(argv)
    if args.self_check:
        try:
            audit_dir().mkdir(parents=True, exist_ok=True)
        except Exception as e:
            _lib.log("self-check FAIL:", e)
            return 1
        _lib.log("self-check OK")
        return 0

    cmd = os.environ.get("WRAPPED_CMD", "")
    argv_json = os.environ.get("WRAPPED_ARGV", "[]")
    exit_code = os.environ.get("WRAPPED_EXIT", "0")
    duration = os.environ.get("WRAPPED_DURATION", "")

    try:
        wrapped_argv = json.loads(argv_json)
    except json.JSONDecodeError:
        wrapped_argv = []

    # Redact the argv (the secret patterns include URL-with-credentials).
    redacted_argv = [_lib.redact(str(a)) for a in wrapped_argv]
    probe_payload = json.dumps(redacted_argv)[:1000]

    event = {
        "chronon": chronon(),
        "trace": os.environ.get("AI_SESSION_ID", ""),
        "cwd": os.getcwd(),
        "actor": "tool",
        "kind": "invocation-result",
        "engine": "command-wrapper",
        "probe": cmd,
        "probe_payload": probe_payload,
        "emission_marker": f"exit={exit_code} duration={duration}",
    }

    try:
        audit_dir().mkdir(parents=True, exist_ok=True)
        with open(month_file(), "a", encoding="utf-8") as f:
            f.write(json.dumps(event, ensure_ascii=False) + "\n")
    except Exception as e:
        # Never block; audit failures are themselves auditable.
        _lib.log("audit-command append failed:", e)
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
