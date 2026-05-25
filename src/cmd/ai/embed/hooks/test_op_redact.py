"""Tests for op-redact.py PreToolUse hook.

All tests MUST be RED before op-redact.py is implemented.
Run with: pytest src/cmd/ai/embed/hooks/test_op_redact.py -v
"""
from __future__ import annotations

import json
import os
import subprocess
import sys
import textwrap
from pathlib import Path
import tempfile

import pytest

HOOKS_DIR = Path(__file__).resolve().parent
SCRIPT = HOOKS_DIR / "op-redact.py"


def run_hook(payload: dict | str, *, env: dict | None = None) -> subprocess.CompletedProcess:
    """Run op-redact.py with the given payload on stdin."""
    if isinstance(payload, dict):
        stdin_data = json.dumps(payload)
    else:
        stdin_data = payload
    e = dict(os.environ)
    if env:
        e.update(env)
    return subprocess.run(
        [sys.executable, str(SCRIPT)],
        input=stdin_data,
        capture_output=True,
        text=True,
        env=e,
    )


# ---------------------------------------------------------------------------
# Test 1: gho_ GitHub token is redacted
# ---------------------------------------------------------------------------

class TestGitHubTokenRedaction:
    def test_gho_token_replaced(self):
        """gho_ token in a string field becomes [REDACTED:github-token].
        Real GitHub tokens are 40 chars total (gho_ + 36 alphanumeric chars).
        """
        # 36 alphanumeric chars after gho_ = valid GitHub OAuth token length.
        token = "gho_" + "A" * 36
        payload = {
            "tool_name": "Bash",
            "tool_input": {
                "command": f"curl -H 'Authorization: token {token}' https://api.github.com/user"
            }
        }
        result = run_hook(payload)
        assert result.returncode == 0, f"hook must exit 0, got stderr: {result.stderr}"
        out = json.loads(result.stdout)
        command = out["tool_input"]["command"]
        assert "gho_" not in command, f"raw token must not appear in output: {command}"
        assert "[REDACTED:github-token]" in command, f"expected redaction marker: {command}"

    def test_ghp_token_replaced(self):
        """ghp_ token is redacted (36+ chars after prefix)."""
        payload = {
            "tool_name": "Bash",
            "tool_input": {
                "command": "echo ghp_" + "A" * 36
            }
        }
        result = run_hook(payload)
        assert result.returncode == 0
        out = json.loads(result.stdout)
        assert "ghp_" not in out["tool_input"]["command"]
        assert "[REDACTED:github-token]" in out["tool_input"]["command"]

    def test_github_pat_replaced(self):
        """github_pat_ fine-grained token is redacted."""
        payload = {
            "tool_name": "Bash",
            "tool_input": {
                "command": "git clone https://github_pat_" + "A" * 60 + "@github.com/org/repo"
            }
        }
        result = run_hook(payload)
        assert result.returncode == 0
        out = json.loads(result.stdout)
        assert "github_pat_" not in out["tool_input"]["command"]


# ---------------------------------------------------------------------------
# Test 2: op:// references are redacted
# ---------------------------------------------------------------------------

class TestOpRefRedaction:
    def test_op_ref_replaced(self):
        """op:// reference is replaced with [REDACTED:op-ref]."""
        payload = {
            "tool_name": "Bash",
            "tool_input": {
                "command": "echo op://Private/MyDB/password"
            }
        }
        result = run_hook(payload)
        assert result.returncode == 0
        out = json.loads(result.stdout)
        assert "op://" not in out["tool_input"]["command"]
        assert "[REDACTED:op-ref]" in out["tool_input"]["command"]

    def test_op_ref_in_nested_field(self):
        """op:// in a deeply nested field is still redacted."""
        payload = {
            "level1": {
                "level2": {
                    "level3": "connect op://Work/Prod/connection_string to db"
                }
            }
        }
        result = run_hook(payload)
        assert result.returncode == 0
        out = json.loads(result.stdout)
        assert "op://" not in out["level1"]["level2"]["level3"]
        assert "[REDACTED:op-ref]" in out["level1"]["level2"]["level3"]


# ---------------------------------------------------------------------------
# Test 3: Bearer token is redacted
# ---------------------------------------------------------------------------

class TestBearerTokenRedaction:
    def test_bearer_token_replaced(self):
        """Bearer token (20+ chars after 'Bearer ') is redacted."""
        payload = {
            "tool_input": {
                "command": "curl -H 'Authorization: Bearer eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.payload.sig' https://api.example.com"
            }
        }
        result = run_hook(payload)
        assert result.returncode == 0
        out = json.loads(result.stdout)
        command = out["tool_input"]["command"]
        # The raw token value after 'Bearer ' should be gone.
        assert "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9" not in command
        assert "[REDACTED:bearer-token]" in command


# ---------------------------------------------------------------------------
# Test 4: PEM block is redacted
# ---------------------------------------------------------------------------

class TestPEMRedaction:
    def test_pem_block_replaced(self):
        """-----BEGIN ... is replaced."""
        payload = {
            "tool_input": {
                "command": "echo '-----BEGIN RSA PRIVATE KEY-----'"
            }
        }
        result = run_hook(payload)
        assert result.returncode == 0
        out = json.loads(result.stdout)
        assert "-----BEGIN" not in out["tool_input"]["command"]
        assert "[REDACTED:pem-block]" in out["tool_input"]["command"]


# ---------------------------------------------------------------------------
# Test 5: sk- OpenAI-style key is redacted
# ---------------------------------------------------------------------------

class TestOpenAIKeyRedaction:
    def test_sk_key_replaced(self):
        """sk- followed by 40+ chars is redacted."""
        key = "sk-" + "A" * 45
        payload = {
            "tool_input": {"command": f"OPENAI_API_KEY={key} python run.py"}
        }
        result = run_hook(payload)
        assert result.returncode == 0
        out = json.loads(result.stdout)
        assert key not in out["tool_input"]["command"]
        assert "[REDACTED:openai-key]" in out["tool_input"]["command"]


# ---------------------------------------------------------------------------
# Test 6: Violation file is written
# ---------------------------------------------------------------------------

class TestViolationFileWritten:
    def test_violation_file_created_on_match(self, tmp_path):
        """When a secret is found, a violation file is written under
        $AI_ROOT/audit/violations/."""
        # 36 chars after gho_ = minimum valid GitHub OAuth token length.
        token = "gho_" + "A" * 36
        payload = {
            "tool_input": {"command": f"echo {token}"}
        }
        result = run_hook(payload, env={"AI_ROOT": str(tmp_path)})
        assert result.returncode == 0

        violations_dir = tmp_path / "audit" / "violations"
        files = list(violations_dir.glob("*-secret-detected.md"))
        assert len(files) >= 1, f"expected violation file in {violations_dir}, found none"

        content = files[0].read_text(encoding="utf-8")
        assert "github-token" in content or "secret" in content.lower()

    def test_no_violation_file_on_clean_payload(self, tmp_path):
        """When no secret is detected, no violation file is written."""
        payload = {
            "tool_input": {"command": "echo hello world"}
        }
        result = run_hook(payload, env={"AI_ROOT": str(tmp_path)})
        assert result.returncode == 0

        violations_dir = tmp_path / "audit" / "violations"
        if violations_dir.exists():
            files = list(violations_dir.glob("*-secret-detected.md"))
            assert len(files) == 0, f"unexpected violation files: {files}"


# ---------------------------------------------------------------------------
# Test 7: Clean payload passes through unchanged
# ---------------------------------------------------------------------------

class TestCleanPayloadPassthrough:
    def test_clean_payload_unchanged(self):
        """A payload with no secrets passes through byte-for-byte (modulo
        JSON serialization)."""
        payload = {
            "tool_name": "Read",
            "tool_input": {"file_path": "/tmp/foo.txt"}
        }
        result = run_hook(payload)
        assert result.returncode == 0
        out = json.loads(result.stdout)
        assert out["tool_name"] == "Read"
        assert out["tool_input"]["file_path"] == "/tmp/foo.txt"


# ---------------------------------------------------------------------------
# Test 8: Exit code is always 0 (hook never blocks)
# ---------------------------------------------------------------------------

class TestExitCodeAlwaysZero:
    def test_exit_zero_with_secret(self):
        """Even when a secret is found, exit code is 0 (redact, never block)."""
        payload = {"command": "gho_" + "A" * 36}
        result = run_hook(payload)
        assert result.returncode == 0, f"hook exited {result.returncode}: {result.stderr}"

    def test_exit_zero_with_empty_payload(self):
        """Empty stdin → exit 0."""
        result = run_hook("")
        assert result.returncode == 0

    def test_exit_zero_with_malformed_json(self):
        """Malformed JSON → exit 0 (scan raw text, output what we can)."""
        result = run_hook("{not valid json")
        assert result.returncode == 0


# ---------------------------------------------------------------------------
# Test 9: Self-check
# ---------------------------------------------------------------------------

class TestSelfCheck:
    def test_self_check_exits_zero(self):
        """--self-check must exit 0."""
        result = subprocess.run(
            [sys.executable, str(SCRIPT), "--self-check"],
            capture_output=True,
            text=True,
        )
        assert result.returncode == 0, f"self-check failed: {result.stderr}"
