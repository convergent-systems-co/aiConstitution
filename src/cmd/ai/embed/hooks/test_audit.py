"""test_audit.py — tests for audit.py hook (story #94).

Tests invoke audit.py as a subprocess with a JSON event on stdin,
then verify the JSONL record written to the audit log.

RED criteria (current embedded version will fail):
- AC: embedded audit.py uses `hook_event_name` as primary key; live uses `hookEventName`
- AC: embedded does not handle PostToolUseFailure, SubagentStart, Notification,
      ErrorOccurred, PermissionRequest event types
- AC: embedded does not emit `probe_result_marker`, `compaction_trigger`, `fault_*`,
      `permission_reason`, `signal_*`, `transcript_marker` per-kind fields
"""
from __future__ import annotations

import json
import os
import subprocess
import sys
import tempfile
from pathlib import Path

import pytest

HOOKS_DIR = Path(__file__).resolve().parent
AUDIT_PY = str(HOOKS_DIR / "audit.py")


def run_hook(event: dict, ai_root: str) -> subprocess.CompletedProcess:
    env = {**os.environ, "AI_ROOT": ai_root}
    return subprocess.run(
        [sys.executable, AUDIT_PY],
        input=json.dumps(event),
        capture_output=True,
        text=True,
        env=env,
    )


def last_jsonl_record(ai_root: str) -> dict:
    """Return the last record written to the audit JSONL."""
    from datetime import datetime, timezone
    now = datetime.now(timezone.utc)
    month = now.strftime("%Y-%m")
    audit_dir = Path(ai_root) / "audit" / "interactions"
    jsonl = audit_dir / f"{month}.jsonl"
    assert jsonl.exists(), f"Audit JSONL not found: {jsonl}"
    lines = [l for l in jsonl.read_text().splitlines() if l.strip()]
    assert lines, "Audit JSONL is empty"
    return json.loads(lines[-1])


@pytest.fixture()
def ai_root(tmp_path):
    return str(tmp_path)


# ---------------------------------------------------------------------------
# AC1: SessionStart → kind=trace-open, actor=system
# ---------------------------------------------------------------------------

class TestSessionStart:
    def test_session_start_kind(self, ai_root):
        event = {"hookEventName": "SessionStart", "session_id": "sess-001"}
        run_hook(event, ai_root)
        rec = last_jsonl_record(ai_root)
        assert rec["kind"] == "trace-open", f"Expected trace-open, got {rec['kind']!r}"

    def test_session_start_actor(self, ai_root):
        event = {"hookEventName": "SessionStart", "session_id": "sess-001"}
        run_hook(event, ai_root)
        rec = last_jsonl_record(ai_root)
        assert rec["actor"] == "system", f"Expected system, got {rec['actor']!r}"

    def test_session_start_has_trace(self, ai_root):
        event = {"hookEventName": "SessionStart", "session_id": "sess-abc"}
        run_hook(event, ai_root)
        rec = last_jsonl_record(ai_root)
        assert rec["trace"] == "sess-abc"

    def test_session_start_has_chronon(self, ai_root):
        event = {"hookEventName": "SessionStart"}
        run_hook(event, ai_root)
        rec = last_jsonl_record(ai_root)
        assert "chronon" in rec
        assert rec["chronon"].endswith("Z")


# ---------------------------------------------------------------------------
# AC2: UserPromptSubmit → kind=request, actor=human, stimulus present
# ---------------------------------------------------------------------------

class TestUserPromptSubmit:
    def test_user_prompt_kind(self, ai_root):
        event = {"hookEventName": "UserPromptSubmit", "prompt": "hello world"}
        run_hook(event, ai_root)
        rec = last_jsonl_record(ai_root)
        assert rec["kind"] == "request"

    def test_user_prompt_actor(self, ai_root):
        event = {"hookEventName": "UserPromptSubmit", "prompt": "hello world"}
        run_hook(event, ai_root)
        rec = last_jsonl_record(ai_root)
        assert rec["actor"] == "human"

    def test_user_prompt_stimulus_present(self, ai_root):
        event = {"hookEventName": "UserPromptSubmit", "prompt": "my prompt text"}
        run_hook(event, ai_root)
        rec = last_jsonl_record(ai_root)
        assert "stimulus" in rec, "stimulus field missing from request record"
        assert "my prompt text" in rec["stimulus"]

    def test_stimulus_truncated_at_2000(self, ai_root):
        long_prompt = "A" * 3000
        event = {"hookEventName": "UserPromptSubmit", "prompt": long_prompt}
        run_hook(event, ai_root)
        rec = last_jsonl_record(ai_root)
        # 2000 chars + truncation marker (e.g. "...[truncated]" is 14 chars)
        assert len(rec["stimulus"]) <= 2020  # 2000 + up to 20 chars for marker
        assert len(rec["stimulus"]) < 3000   # definitely shorter than the original


# ---------------------------------------------------------------------------
# AC3: PreToolUse → kind=invocation-attempt, actor=tool, probe + probe_payload
# ---------------------------------------------------------------------------

class TestPreToolUse:
    def test_pre_tool_use_kind(self, ai_root):
        event = {
            "hookEventName": "PreToolUse",
            "tool_name": "Bash",
            "tool_input": {"command": "ls"},
        }
        run_hook(event, ai_root)
        rec = last_jsonl_record(ai_root)
        assert rec["kind"] == "invocation-attempt"

    def test_pre_tool_use_actor(self, ai_root):
        event = {
            "hookEventName": "PreToolUse",
            "tool_name": "Bash",
            "tool_input": {"command": "ls"},
        }
        run_hook(event, ai_root)
        rec = last_jsonl_record(ai_root)
        assert rec["actor"] == "tool"

    def test_pre_tool_use_probe_present(self, ai_root):
        event = {
            "hookEventName": "PreToolUse",
            "tool_name": "Bash",
            "tool_input": {"command": "ls"},
        }
        run_hook(event, ai_root)
        rec = last_jsonl_record(ai_root)
        assert "probe" in rec
        assert rec["probe"] == "Bash"

    def test_pre_tool_use_probe_payload_present(self, ai_root):
        event = {
            "hookEventName": "PreToolUse",
            "tool_name": "Read",
            "tool_input": {"file_path": "/etc/hosts"},
        }
        run_hook(event, ai_root)
        rec = last_jsonl_record(ai_root)
        assert "probe_payload" in rec


# ---------------------------------------------------------------------------
# AC4: PostToolUse → kind=invocation-result, actor=tool
# ---------------------------------------------------------------------------

class TestPostToolUse:
    def test_post_tool_use_kind(self, ai_root):
        event = {"hookEventName": "PostToolUse", "tool_name": "Bash"}
        run_hook(event, ai_root)
        rec = last_jsonl_record(ai_root)
        assert rec["kind"] == "invocation-result"

    def test_post_tool_use_actor(self, ai_root):
        event = {"hookEventName": "PostToolUse", "tool_name": "Bash"}
        run_hook(event, ai_root)
        rec = last_jsonl_record(ai_root)
        assert rec["actor"] == "tool"


# ---------------------------------------------------------------------------
# AC5: Stop → kind=emission, actor=assistant, emission_marker present
# ---------------------------------------------------------------------------

class TestStop:
    def test_stop_kind(self, ai_root):
        event = {"hookEventName": "Stop", "stopReason": "end_turn"}
        run_hook(event, ai_root)
        rec = last_jsonl_record(ai_root)
        assert rec["kind"] == "emission"

    def test_stop_actor(self, ai_root):
        event = {"hookEventName": "Stop", "stopReason": "end_turn"}
        run_hook(event, ai_root)
        rec = last_jsonl_record(ai_root)
        assert rec["actor"] == "assistant"

    def test_stop_emission_marker(self, ai_root):
        event = {"hookEventName": "Stop", "stopReason": "end_turn"}
        run_hook(event, ai_root)
        rec = last_jsonl_record(ai_root)
        assert "emission_marker" in rec


# ---------------------------------------------------------------------------
# AC6: SessionEnd → kind=trace-close, actor=system
# ---------------------------------------------------------------------------

class TestSessionEnd:
    def test_session_end_kind(self, ai_root):
        event = {"hookEventName": "SessionEnd"}
        run_hook(event, ai_root)
        rec = last_jsonl_record(ai_root)
        assert rec["kind"] == "trace-close"

    def test_session_end_actor(self, ai_root):
        event = {"hookEventName": "SessionEnd"}
        run_hook(event, ai_root)
        rec = last_jsonl_record(ai_root)
        assert rec["actor"] == "system"


# ---------------------------------------------------------------------------
# AC7: SubagentStop → kind=subagent-emission, actor=assistant
# ---------------------------------------------------------------------------

class TestSubagentStop:
    def test_subagent_stop_kind(self, ai_root):
        event = {"hookEventName": "SubagentStop"}
        run_hook(event, ai_root)
        rec = last_jsonl_record(ai_root)
        assert rec["kind"] == "subagent-emission"

    def test_subagent_stop_actor(self, ai_root):
        event = {"hookEventName": "SubagentStop"}
        run_hook(event, ai_root)
        rec = last_jsonl_record(ai_root)
        assert rec["actor"] == "assistant"


# ---------------------------------------------------------------------------
# AC8: PreCompact → kind=compaction-attempt, actor=system
# ---------------------------------------------------------------------------

class TestPreCompact:
    def test_pre_compact_kind(self, ai_root):
        event = {"hookEventName": "PreCompact"}
        run_hook(event, ai_root)
        rec = last_jsonl_record(ai_root)
        assert rec["kind"] == "compaction-attempt"

    def test_pre_compact_actor(self, ai_root):
        event = {"hookEventName": "PreCompact"}
        run_hook(event, ai_root)
        rec = last_jsonl_record(ai_root)
        assert rec["actor"] == "system"


# ---------------------------------------------------------------------------
# AC9: output written to ~/.ai/audit/interactions/<YYYY-MM>.jsonl
# ---------------------------------------------------------------------------

class TestAuditDirectory:
    def test_audit_dir_created(self, ai_root):
        event = {"hookEventName": "SessionStart"}
        run_hook(event, ai_root)
        audit_dir = Path(ai_root) / "audit" / "interactions"
        assert audit_dir.exists(), f"Audit dir not created: {audit_dir}"

    def test_audit_file_is_jsonl(self, ai_root):
        from datetime import datetime, timezone
        event = {"hookEventName": "SessionStart"}
        run_hook(event, ai_root)
        rec = last_jsonl_record(ai_root)
        # If we got here, parse worked — it's valid JSONL
        assert isinstance(rec, dict)


# ---------------------------------------------------------------------------
# AC10: audit failure never causes non-zero exit
# ---------------------------------------------------------------------------

class TestAuditNeverBlocks:
    def test_empty_stdin_exits_zero(self, ai_root):
        result = subprocess.run(
            [sys.executable, AUDIT_PY],
            input="",
            capture_output=True,
            text=True,
            env={**os.environ, "AI_ROOT": ai_root},
        )
        assert result.returncode == 0

    def test_malformed_json_exits_zero(self, ai_root):
        result = subprocess.run(
            [sys.executable, AUDIT_PY],
            input="{not valid json",
            capture_output=True,
            text=True,
            env={**os.environ, "AI_ROOT": ai_root},
        )
        assert result.returncode == 0

    def test_unwritable_dir_exits_zero(self, tmp_path):
        """Even if audit dir is unwritable, hook must exit 0."""
        bad_root = str(tmp_path / "readonly")
        os.makedirs(bad_root, mode=0o444, exist_ok=True)
        event = {"hookEventName": "SessionStart"}
        result = subprocess.run(
            [sys.executable, AUDIT_PY],
            input=json.dumps(event),
            capture_output=True,
            text=True,
            env={**os.environ, "AI_ROOT": bad_root},
        )
        assert result.returncode == 0


# ---------------------------------------------------------------------------
# AC11: stimulus and probe_payload are redacted
# ---------------------------------------------------------------------------

class TestRedaction:
    def test_stimulus_redacted_github_token(self, ai_root):
        """A GitHub token in the prompt must be redacted in the audit record."""
        fake_token = "ghp_" + "A" * 36
        event = {"hookEventName": "UserPromptSubmit", "prompt": f"use token {fake_token}"}
        run_hook(event, ai_root)
        rec = last_jsonl_record(ai_root)
        assert fake_token not in rec.get("stimulus", ""), (
            f"GitHub token was not redacted from stimulus: {rec.get('stimulus')!r}"
        )

    def test_probe_payload_redacted(self, ai_root):
        """A GitHub token in the tool input must be redacted in probe_payload."""
        fake_token = "ghp_" + "B" * 36
        event = {
            "hookEventName": "PreToolUse",
            "tool_name": "Bash",
            "tool_input": {"command": f"echo {fake_token}"},
        }
        run_hook(event, ai_root)
        rec = last_jsonl_record(ai_root)
        assert fake_token not in rec.get("probe_payload", ""), (
            f"Token was not redacted from probe_payload: {rec.get('probe_payload')!r}"
        )


# ---------------------------------------------------------------------------
# Extended event types (must be handled, not fall through to unknown)
# ---------------------------------------------------------------------------

class TestExtendedEventTypes:
    """The live version handles these; embedded must match."""

    def test_post_tool_use_failure(self, ai_root):
        event = {"hookEventName": "PostToolUseFailure", "tool_name": "Bash"}
        run_hook(event, ai_root)
        rec = last_jsonl_record(ai_root)
        assert rec["kind"] == "invocation-failure", (
            f"PostToolUseFailure should map to invocation-failure, got {rec['kind']!r}"
        )

    def test_subagent_start(self, ai_root):
        event = {"hookEventName": "SubagentStart"}
        run_hook(event, ai_root)
        rec = last_jsonl_record(ai_root)
        assert rec["kind"] == "subagent-trace-open", (
            f"SubagentStart should map to subagent-trace-open, got {rec['kind']!r}"
        )

    def test_error_occurred(self, ai_root):
        event = {
            "hookEventName": "ErrorOccurred",
            "error": {"name": "TimeoutError", "message": "Timed out"},
            "recoverable": False,
        }
        run_hook(event, ai_root)
        rec = last_jsonl_record(ai_root)
        assert rec["kind"] == "fault", (
            f"ErrorOccurred should map to fault, got {rec['kind']!r}"
        )

    def test_permission_request(self, ai_root):
        event = {
            "hookEventName": "PermissionRequest",
            "tool_name": "Bash",
            "reason": "needs filesystem access",
        }
        run_hook(event, ai_root)
        rec = last_jsonl_record(ai_root)
        assert rec["kind"] == "permission-prompt", (
            f"PermissionRequest should map to permission-prompt, got {rec['kind']!r}"
        )

    def test_notification(self, ai_root):
        event = {"hookEventName": "Notification", "message": "context limit approaching"}
        run_hook(event, ai_root)
        rec = last_jsonl_record(ai_root)
        assert rec["kind"] == "signal", (
            f"Notification should map to signal, got {rec['kind']!r}"
        )
