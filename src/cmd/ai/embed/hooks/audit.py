#!/usr/bin/env python3
"""hooks/audit.py — interaction audit logger.

Appends one JSONL line per event to
~/.ai/audit/interactions/<YYYY-MM>.jsonl using the
domain-specific vocabulary defined in ~/.ai/Common.md §5.2 (chronon,
trace, cwd, actor, kind, engine, stimulus, probe, probe_payload,
emission_marker).

Wired into Claude Code via every event (`SessionStart`,
`UserPromptSubmit`, `PreToolUse`, `PostToolUse`, `Stop`,
`SessionEnd`, `SubagentStop`, `PreCompact`) and into Copilot CLI's
equivalent surface.

Reads the event payload from stdin as JSON. Always exits 0 (audit
must never block).

Self-check:
  --self-check    Verify the audit directory is writable.
"""
from __future__ import annotations

import datetime as dt
import json
import os
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
import _lib  # noqa: E402

# Mapping from Claude Code event names to our vocabulary. Kept here
# (not in the schema) because the mapping is consumer-specific.
CLAUDE_KIND_MAP = {
    "SessionStart": "trace-open",
    "SessionEnd": "trace-close",
    "UserPromptSubmit": "request",
    "PreToolUse": "invocation-attempt",
    "PostToolUse": "invocation-result",
    "Stop": "emission",
    "SubagentStop": "subagent-emission",
    "PreCompact": "compaction-attempt",
}


def audit_dir() -> Path:
    """Path to ~/.ai/audit/interactions/. AI_ROOT overrides $HOME/.ai."""
    root = os.environ.get("AI_ROOT", str(Path.home() / ".ai"))
    return Path(root) / "audit" / "interactions"


def month_file() -> Path:
    """The audit JSONL for the current UTC month."""
    now = dt.datetime.now(dt.timezone.utc)
    return audit_dir() / f"{now.strftime('%Y-%m')}.jsonl"


def chronon() -> str:
    """ISO-8601 with millisecond precision and 'Z' suffix."""
    now = dt.datetime.now(dt.timezone.utc)
    return now.strftime("%Y-%m-%dT%H:%M:%S.") + f"{now.microsecond // 1000:03d}Z"


def normalize_event(raw: dict) -> dict:
    """Map a raw Claude/Copilot event payload to the canonical schema.

    Truncates `stimulus` to 2000 chars and `probe_payload` to 1000
    chars per Common.md §5.2."""
    claude_kind = raw.get("hook_event_name") or raw.get("event") or ""
    kind = CLAUDE_KIND_MAP.get(claude_kind, claude_kind or "signal")

    event = {
        "chronon": chronon(),
        "trace": raw.get("session_id") or raw.get("trace") or "",
        "cwd": raw.get("cwd") or os.getcwd(),
        "actor": raw.get("actor") or _actor_for(kind),
        "kind": kind,
        "engine": raw.get("engine") or raw.get("model") or "",
    }

    # Per-kind payload fields.
    if kind == "request":
        event["stimulus"] = _truncate(raw.get("prompt") or raw.get("stimulus") or "", 2000)
    elif kind in ("invocation-attempt", "invocation-result"):
        event["probe"] = raw.get("tool_name") or raw.get("probe") or ""
        payload = raw.get("tool_input") or raw.get("probe_payload") or {}
        if isinstance(payload, (dict, list)):
            payload = json.dumps(payload)
        event["probe_payload"] = _truncate(str(payload), 1000)
    elif kind in ("emission", "subagent-emission"):
        event["emission_marker"] = raw.get("emission_marker") or raw.get("stop_reason") or ""

    return event


def _actor_for(kind: str) -> str:
    if kind in ("request",):
        return "human"
    if kind in ("emission", "subagent-emission"):
        return "assistant"
    if kind in ("invocation-attempt", "invocation-result"):
        return "tool"
    return "system"


def _truncate(s: str, limit: int) -> str:
    if len(s) <= limit:
        return s
    return s[:limit] + "…[truncated]"


def main(argv: list[str]) -> int:
    if "--self-check" in argv:
        try:
            audit_dir().mkdir(parents=True, exist_ok=True)
            test = month_file().with_suffix(".jsonl.self-check")
            test.write_text("", encoding="utf-8")
            test.unlink(missing_ok=True)
        except Exception as e:
            _lib.log("self-check FAIL:", e)
            return 1
        _lib.log("self-check OK")
        return 0

    raw_input = sys.stdin.read()
    if not raw_input.strip():
        return 0

    try:
        raw = json.loads(raw_input)
    except json.JSONDecodeError:
        raw = {"raw": raw_input}

    try:
        event = normalize_event(raw)
        # Redaction pass: scrub the structured fields. Per Common.md §4.5.
        event["stimulus"] = _lib.redact(event.get("stimulus", ""))
        event["probe_payload"] = _lib.redact(event.get("probe_payload", ""))
        audit_dir().mkdir(parents=True, exist_ok=True)
        with open(month_file(), "a", encoding="utf-8") as f:
            f.write(json.dumps(event, ensure_ascii=False) + "\n")
    except Exception as e:
        _lib.log("audit append failed:", e)
        # Audit failures must NEVER block — exit 0 anyway.
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
