#!/usr/bin/env python3
"""hooks/secret-block.py — PreToolUse hook that denies Bash commands
containing secret-shaped strings before they execute.

Reads the canonical pattern set from hooks/patterns.json
(+ patterns.local.json if present). Belt-and-suspenders alongside the
secret-handling rules in Common.md §4. Per SPEC.md §10.1.

Input contract (Claude Code PreToolUse):
  - The full tool-use payload arrives on stdin as JSON.
  - On detection: emit JSON permissionDecision deny on stdout, exit 0.
  - On clean: exit 0, no stdout.
  - Stderr is shown to the user.

Output schema:
  {
    "hookSpecificOutput": {
      "hookEventName": "PreToolUse",
      "permissionDecision": "deny",
      "permissionDecisionReason": "<explanation>"
    }
  }

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


def deny(reason: str) -> None:
    """Emit a permission-deny decision via JSON stdout and exit 0."""
    print(json.dumps({
        "hookSpecificOutput": {
            "hookEventName": "PreToolUse",
            "permissionDecision": "deny",
            "permissionDecisionReason": reason,
        }
    }))
    sys.exit(0)


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


def main(argv: list) -> int:
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

    # Only guard PreToolUse events on Bash/shell tools.
    if isinstance(payload, dict):
        hook_event = (
            payload.get("hookEventName")
            or payload.get("hook_event_name")
            or ""
        )
        if hook_event and hook_event != "PreToolUse":
            return 0

        tool_name = payload.get("tool_name") or payload.get("toolName") or ""
        if tool_name and tool_name not in ("Bash", "shell", "execute"):
            return 0

        # Extract the command to scan.
        tool_input = (
            payload.get("tool_input")
            or payload.get("toolInput")
            or payload.get("toolArgs")
            or {}
        )
        if isinstance(tool_input, dict):
            command = tool_input.get("command") or tool_input.get("cmd") or ""
        else:
            command = extract_command(payload)
    else:
        command = raw

    if not command:
        return 0

    patterns = _lib.load_patterns()
    hits = _lib.scan_lines(command.splitlines() or [command], patterns)
    if not hits:
        return 0

    # Build a deny reason that does NOT echo the full secret value.
    # Take the first hit and construct a truncated/redacted snippet.
    hit = hits[0]
    pattern_id = hit.get("pattern_id", "unknown")
    severity = hit.get("severity", "medium")
    snippet = hit.get("snippet", "[redacted]")

    extra = ""
    if len(hits) > 1:
        extra = f" (and {len(hits) - 1} more match(es))"

    deny(
        f"Possible secret detected in Bash command: pattern={pattern_id} severity={severity}{extra}.\n"
        f"Snippet (redacted): {snippet}\n"
        "Per Common.md §1.P4 (no secrets in artifacts; non-overridable).\n"
        "Use OS clipboard transfer (Common.md §4.2) instead of embedding secrets in commands."
    )
    # deny() calls sys.exit(0); this return is unreachable but satisfies type checkers.
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
