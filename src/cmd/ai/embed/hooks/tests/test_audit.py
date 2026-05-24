"""Unit tests for hooks/audit.py.

The audit hook reads a Claude Code event payload on stdin and appends
one JSONL line per event to ``$AI_ROOT/audit/interactions/<YYYY-MM>.jsonl``.
Tests drive the hook as a subprocess so we exercise the same stdin /
exit-code contract Claude Code uses.

Schema fields verified per ~/.ai/Common.md §5.2: chronon, trace, cwd,
actor, kind, engine, plus the per-kind fields stimulus (truncated 2000),
probe, probe_payload (truncated 1000), emission_marker.
"""
from __future__ import annotations

import datetime as dt
import json
import os
import re
import subprocess
import sys
from pathlib import Path

import pytest


HOOK = Path(__file__).resolve().parent.parent / "audit.py"


# ---------------------------------------------------------------------------
# Fixtures
# ---------------------------------------------------------------------------

@pytest.fixture
def ai_root(tmp_path, monkeypatch):
    """Isolate audit output to a per-test tmp dir via AI_ROOT."""
    monkeypatch.setenv("AI_ROOT", str(tmp_path))
    return tmp_path


def run_audit(payload, *, env=None):
    """Invoke audit.py as a subprocess with `payload` on stdin.

    Returns (exit_code, stderr_text, parsed_jsonl_lines).
    """
    full_env = os.environ.copy()
    if env:
        full_env.update(env)
    proc = subprocess.run(
        [sys.executable, str(HOOK)],
        input=json.dumps(payload),
        capture_output=True,
        text=True,
        env=full_env,
    )
    audit_dir = Path(full_env["AI_ROOT"]) / "audit" / "interactions"
    month = dt.datetime.now(dt.timezone.utc).strftime("%Y-%m")
    target = audit_dir / f"{month}.jsonl"
    lines = []
    if target.is_file():
        lines = [json.loads(line) for line in target.read_text().splitlines() if line.strip()]
    return proc.returncode, proc.stderr, lines


# ---------------------------------------------------------------------------
# Per-event tests (covers all 7 issue-listed event kinds)
# ---------------------------------------------------------------------------

def test_session_start_event(ai_root):
    rc, _, lines = run_audit(
        {"hook_event_name": "SessionStart", "session_id": "s1", "engine": "opus-4.7"},
        env={"AI_ROOT": str(ai_root)},
    )
    assert rc == 0
    assert len(lines) == 1
    e = lines[0]
    assert e["kind"] == "trace-open"
    assert e["trace"] == "s1"
    assert e["engine"] == "opus-4.7"
    # Per-kind fields stay absent.
    assert "stimulus" not in e
    assert "probe" not in e


def test_user_prompt_submit_event(ai_root):
    rc, _, lines = run_audit(
        {
            "hook_event_name": "UserPromptSubmit",
            "session_id": "s2",
            "prompt": "hello world",
            "engine": "opus-4.7",
        },
        env={"AI_ROOT": str(ai_root)},
    )
    assert rc == 0
    e = lines[0]
    assert e["kind"] == "request"
    assert e["stimulus"] == "hello world"
    assert e["actor"] == "human"


def test_pre_tool_use_event(ai_root):
    rc, _, lines = run_audit(
        {
            "hook_event_name": "PreToolUse",
            "session_id": "s3",
            "tool_name": "Bash",
            "tool_input": {"command": "ls"},
        },
        env={"AI_ROOT": str(ai_root)},
    )
    assert rc == 0
    e = lines[0]
    assert e["kind"] == "invocation-attempt"
    assert e["probe"] == "Bash"
    assert e["probe_payload"] == json.dumps({"command": "ls"})
    assert e["actor"] == "tool"


def test_post_tool_use_event(ai_root):
    rc, _, lines = run_audit(
        {
            "hook_event_name": "PostToolUse",
            "session_id": "s4",
            "tool_name": "Bash",
            "tool_input": {"command": "ls"},
        },
        env={"AI_ROOT": str(ai_root)},
    )
    assert rc == 0
    e = lines[0]
    assert e["kind"] == "invocation-result"
    assert e["probe"] == "Bash"


def test_stop_event(ai_root):
    rc, _, lines = run_audit(
        {
            "hook_event_name": "Stop",
            "session_id": "s5",
            "stop_reason": "end_turn",
        },
        env={"AI_ROOT": str(ai_root)},
    )
    assert rc == 0
    e = lines[0]
    assert e["kind"] == "emission"
    assert e["emission_marker"] == "end_turn"
    assert e["actor"] == "assistant"


def test_subagent_stop_event(ai_root):
    rc, _, lines = run_audit(
        {
            "hook_event_name": "SubagentStop",
            "session_id": "s6",
            "stop_reason": "end_turn",
        },
        env={"AI_ROOT": str(ai_root)},
    )
    assert rc == 0
    e = lines[0]
    assert e["kind"] == "subagent-emission"
    assert e["actor"] == "assistant"


def test_pre_compact_event(ai_root):
    rc, _, lines = run_audit(
        {"hook_event_name": "PreCompact", "session_id": "s7"},
        env={"AI_ROOT": str(ai_root)},
    )
    assert rc == 0
    e = lines[0]
    assert e["kind"] == "compaction-attempt"


# ---------------------------------------------------------------------------
# Schema-level tests
# ---------------------------------------------------------------------------

def test_chronon_is_iso8601_ms_z(ai_root):
    rc, _, lines = run_audit(
        {"hook_event_name": "SessionStart", "session_id": "s"},
        env={"AI_ROOT": str(ai_root)},
    )
    assert rc == 0
    chronon = lines[0]["chronon"]
    # YYYY-MM-DDTHH:MM:SS.mmmZ
    assert re.match(r"^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d{3}Z$", chronon), chronon


def test_required_fields_always_present(ai_root):
    rc, _, lines = run_audit(
        {"hook_event_name": "SessionStart", "session_id": "s"},
        env={"AI_ROOT": str(ai_root)},
    )
    assert rc == 0
    e = lines[0]
    for field in ("chronon", "trace", "cwd", "actor", "kind", "engine"):
        assert field in e, f"missing field {field}"


def test_stimulus_truncated_at_2000(ai_root):
    big = "x" * 5000
    rc, _, lines = run_audit(
        {"hook_event_name": "UserPromptSubmit", "session_id": "s", "prompt": big},
        env={"AI_ROOT": str(ai_root)},
    )
    assert rc == 0
    stim = lines[0]["stimulus"]
    # Truncated string keeps first 2000 chars + a "[truncated]" marker.
    assert stim.startswith("x" * 2000)
    assert "truncated" in stim
    # And it should NOT have all 5000 chars verbatim.
    assert stim != big


def test_probe_payload_truncated_at_1000(ai_root):
    big_cmd = "y" * 3000
    rc, _, lines = run_audit(
        {
            "hook_event_name": "PreToolUse",
            "session_id": "s",
            "tool_name": "Bash",
            "tool_input": {"command": big_cmd},
        },
        env={"AI_ROOT": str(ai_root)},
    )
    assert rc == 0
    payload = lines[0]["probe_payload"]
    assert "truncated" in payload
    # The first 1000 chars come from the JSON-encoded dict — they
    # include the leading `{"command": "y...`. Just assert length cap.
    # The truncation suffix adds beyond the 1000; the raw cap is 1000.
    head = payload.split("…[truncated]")[0]
    assert len(head) == 1000


def test_creates_audit_dir_if_absent(tmp_path):
    # AI_ROOT points to a path that does not yet contain audit/interactions
    rc, _, lines = run_audit(
        {"hook_event_name": "SessionStart", "session_id": "s"},
        env={"AI_ROOT": str(tmp_path)},
    )
    assert rc == 0
    assert (tmp_path / "audit" / "interactions").is_dir()
    assert len(lines) == 1


def test_jsonl_appends_not_overwrites(ai_root):
    for i in range(3):
        rc, _, _ = run_audit(
            {"hook_event_name": "SessionStart", "session_id": f"s{i}"},
            env={"AI_ROOT": str(ai_root)},
        )
        assert rc == 0
    month = dt.datetime.now(dt.timezone.utc).strftime("%Y-%m")
    f = ai_root / "audit" / "interactions" / f"{month}.jsonl"
    lines = [json.loads(line) for line in f.read_text().splitlines() if line.strip()]
    assert len(lines) == 3
    assert [l["trace"] for l in lines] == ["s0", "s1", "s2"]


def test_audit_never_blocks_on_garbage_input(ai_root):
    # Non-JSON stdin — hook still must exit 0.
    proc = subprocess.run(
        [sys.executable, str(HOOK)],
        input="this is not json",
        capture_output=True,
        text=True,
        env={**os.environ, "AI_ROOT": str(ai_root)},
    )
    assert proc.returncode == 0


def test_audit_silent_on_empty_stdin(ai_root):
    proc = subprocess.run(
        [sys.executable, str(HOOK)],
        input="",
        capture_output=True,
        text=True,
        env={**os.environ, "AI_ROOT": str(ai_root)},
    )
    assert proc.returncode == 0
    # No file should have been created.
    month = dt.datetime.now(dt.timezone.utc).strftime("%Y-%m")
    f = ai_root / "audit" / "interactions" / f"{month}.jsonl"
    assert not f.exists()
