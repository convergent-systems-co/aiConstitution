#!/usr/bin/env python3
"""hooks/secret-block.py — PreToolUse hook that denies Bash commands
containing secret-shaped strings before they execute.

Reads patterns from hooks/patterns.json (+ patterns.local.json if
present). Belt-and-suspenders alongside the secret-handling rules in
Common.md §4. Per SPEC.md §10.1.

Input contract (Claude Code PreToolUse):
  - The full tool-use payload arrives on stdin as JSON.
  - Exit 0 → allow. Exit 1 (or non-zero) → block.
  - Stderr is shown to the user.

Self-check:
  --self-check  Loads patterns.json and compiles every regex.
"""
from __future__ import annotations

import json
import sys
from pathlib import Path

# Allow `import _lib` when the hooks dir is on PYTHONPATH OR when this
# script is exec'd directly from ~/.ai/hooks/.
sys.path.insert(0, str(Path(__file__).resolve().parent))
import _lib  # noqa: E402


def extract_command(payload: dict) -> str:
    """Best-effort extraction of the command being run from the
    Claude tool-use payload shape. Falls back to stringifying the
    whole payload, which still matches patterns."""
    if not isinstance(payload, dict):
        return json.dumps(payload)
    # Common shapes across Claude Code event versions.
    cmd = (
        payload.get("command")
        or payload.get("input", {}).get("command")
        or payload.get("params", {}).get("command")
        or payload.get("tool_input", {}).get("command")
    )
    if cmd:
        return cmd
    return json.dumps(payload)


def main(argv: list[str]) -> int:
    if "--self-check" in argv:
        return _lib.self_check_ok()

    raw = sys.stdin.read()
    if not raw.strip():
        # No payload to inspect; allow by default.
        return 0

    try:
        payload = json.loads(raw)
    except json.JSONDecodeError:
        # If we can't parse, still scan the raw text for patterns.
        payload = {"raw": raw}

    command = extract_command(payload) if isinstance(payload, dict) else raw
    patterns = _lib.load_patterns()
    hits = _lib.scan_lines(command.splitlines() or [command], patterns)
    if not hits:
        return 0

    _lib.log(f"blocking — {len(hits)} secret-like match(es) in tool input:")
    for h in hits[:10]:
        _lib.log(
            f"  pattern={h['pattern_id']} severity={h['severity']} "
            f"line={h['line']} col={h['col']} snippet={h['snippet']}"
        )
    _lib.log("Per Common.md §1.P4 (no secrets in artifacts; non-overridable).")
    _lib.log("See SPEC.md §10.1 for canonical pattern source.")
    return 1


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
