#!/usr/bin/env python3
"""hooks/audit.py — interaction audit logger.

Appends one JSONL line per event to
~/.ai/audit/interactions/<YYYY-MM>.jsonl using the
domain-specific vocabulary defined in ~/.ai/Common.md §5.2 (chronon,
trace, cwd, actor, kind, engine, stimulus, probe, probe_payload,
emission_marker).

Wired into Claude Code via every event (SessionStart, UserPromptSubmit,
PreToolUse, PostToolUse, Stop, SessionEnd, SubagentStop, PreCompact, and
additional event types listed in KIND_MAP below).

Reads the event payload from stdin as JSON. Always exits 0 (audit
must never block).

Vocabulary mapping (deliberate non-mirror of Claude/Copilot terms):

    Common term     ->  This system
    ─────────────────────────────────
    session         ->  trace
    turn            ->  exchange
    tool call       ->  invocation / probe
    user message    ->  request / stimulus
    assistant turn  ->  emission
    timestamp       ->  chronon
    model           ->  engine

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

# Mapping from Claude Code event names to our vocabulary.
# Vocabulary deliberately does NOT mirror Claude or Copilot terms per Common.md §5.2.
KIND_MAP = {
    # Core Claude Code events (PascalCase)
    "SessionStart": "trace-open",
    "SessionEnd": "trace-close",
    "UserPromptSubmit": "request",
    "PreToolUse": "invocation-attempt",
    "PostToolUse": "invocation-result",
    "PostToolUseFailure": "invocation-failure",
    "Stop": "emission",
    "SubagentStart": "subagent-trace-open",
    "SubagentStop": "subagent-emission",
    "Notification": "signal",
    "PreCompact": "compaction-attempt",
    "ErrorOccurred": "fault",
    "PermissionRequest": "permission-prompt",
    # Copilot CLI camelCase event names
    "userPromptSubmitted": "request",
    "agentStop": "emission",
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


def _truncate(s: str, limit: int) -> str:
    if len(s) <= limit:
        return s
    return s[:limit] + "...[truncated]"


def _actor_for(kind: str) -> str:
    if kind == "request":
        return "human"
    if kind in ("emission", "subagent-emission", "signal", "subagent-trace-open"):
        return "assistant"
    if "invocation" in kind or kind == "permission-prompt":
        return "tool"
    return "system"


def normalize_event(raw: dict) -> dict:
    """Map a raw Claude/Copilot event payload to the canonical schema.

    Truncates stimulus to 2000 chars and probe_payload to 1000
    chars per Common.md §5.2. Redacts secrets via _lib.redact()."""
    # Primary key is hookEventName (Claude Code PascalCase); fall back to
    # hook_event_name, then event.
    claude_kind = (
        raw.get("hookEventName")
        or raw.get("hook_event_name")
        or raw.get("event")
        or ""
    )
    kind = KIND_MAP.get(claude_kind, claude_kind.lower() if claude_kind else "signal")

    event = {
        "chronon": chronon(),
        "trace": (
            raw.get("session_id")
            or raw.get("sessionId")
            or raw.get("trace")
            or ""
        ),
        "cwd": raw.get("cwd") or raw.get("workingDirectory") or os.getcwd(),
        "actor": raw.get("actor") or _actor_for(kind),
        "kind": kind,
        "engine": raw.get("engine") or raw.get("model") or raw.get("modelName") or "",
    }

    # Per-kind payload fields — only include fields relevant to the event type.
    if kind == "request":
        stimulus = raw.get("prompt") or raw.get("stimulus") or ""
        event["stimulus"] = _lib.redact(_truncate(str(stimulus), 2000))

    elif kind in ("invocation-attempt", "invocation-result", "invocation-failure"):
        event["probe"] = (
            raw.get("tool_name")
            or raw.get("toolName")
            or raw.get("probe")
            or ""
        )
        payload = (
            raw.get("tool_input")
            or raw.get("toolInput")
            or raw.get("toolArgs")
            or raw.get("probe_payload")
            or {}
        )
        if isinstance(payload, (dict, list)):
            payload = json.dumps(payload)
        event["probe_payload"] = _lib.redact(_truncate(str(payload), 1000))
        # Record that a result is present (don't log content — can be large)
        if kind == "invocation-result":
            result_val = (
                raw.get("tool_response")
                or raw.get("toolResponse")
                or raw.get("toolResult")
                or raw.get("tool_result")
            )
            if result_val is not None:
                event["probe_result_marker"] = "present"

    elif kind in ("emission", "subagent-emission"):
        event["emission_marker"] = (
            raw.get("stopReason")
            or raw.get("stop_reason")
            or raw.get("emission_marker")
            or "stop"
        )
        if raw.get("transcriptPath") or raw.get("transcript_path"):
            event["transcript_marker"] = "present"

    elif kind == "compaction-attempt":
        event["compaction_trigger"] = raw.get("trigger") or ""

    elif kind == "fault":
        err = raw.get("error") or {}
        if isinstance(err, dict):
            event["fault_name"] = err.get("name", "")
            event["fault_message"] = _truncate(err.get("message", ""), 500)
        else:
            event["fault_name"] = ""
            event["fault_message"] = ""
        event["fault_context"] = (
            raw.get("errorContext") or raw.get("error_context") or ""
        )
        event["fault_recoverable"] = bool(raw.get("recoverable", False))

    elif kind == "permission-prompt":
        event["probe"] = raw.get("tool_name") or raw.get("toolName") or ""
        event["permission_reason"] = _truncate(str(raw.get("reason", "")), 500)

    elif kind == "signal":
        event["signal_type"] = (
            raw.get("notification_type")
            or raw.get("notificationType")
            or ""
        )
        event["signal_message"] = _truncate(str(raw.get("message", "")), 500)

    return event


def main(argv: list) -> int:
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
        # Redaction pass: scrub the structured fields in-place, but
        # ONLY when the per-kind field is present. Per Common.md §4.5
        # the redaction itself is non-optional; per §5.2 the per-kind
        # fields are kind-scoped (stimulus only for `request`,
        # probe/probe_payload only for invocation-*, emission_marker
        # only for emission/subagent-emission). Materializing empty
        # strings for unrelated kinds pollutes downstream greps.
        if "stimulus" in event:
            event["stimulus"] = _lib.redact(event["stimulus"])
        if "probe_payload" in event:
            event["probe_payload"] = _lib.redact(event["probe_payload"])
        audit_dir().mkdir(parents=True, exist_ok=True)
        with open(month_file(), "a", encoding="utf-8") as f:
            f.write(json.dumps(event, ensure_ascii=False, separators=(",", ":")) + "\n")
    except Exception as e:
        _lib.log("audit append failed:", e)
        # Audit failures must NEVER block — exit 0 anyway.
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
