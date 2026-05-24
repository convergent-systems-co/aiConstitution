"""test_secret_block.py — tests for secret-block.py (story #91 + #180).

Tests invoke secret-block.py as a subprocess with a JSON event on stdin.

RED criteria (current embedded version will fail):
- AC8: deny output must NOT echo full secret (current version uses hardcoded inline
  patterns but doesn't use _lib.load_patterns())
- AC9: must use _lib.load_patterns(), not hardcoded inline patterns
- The embedded version also uses exit-1 output schema instead of JSON permissionDecision
"""
from __future__ import annotations

import json
import os
import subprocess
import sys
from pathlib import Path

import pytest

HOOKS_DIR = Path(__file__).resolve().parent
SECRET_BLOCK = str(HOOKS_DIR / "secret-block.py")


def run_hook(event: dict) -> subprocess.CompletedProcess:
    return subprocess.run(
        [sys.executable, SECRET_BLOCK],
        input=json.dumps(event),
        capture_output=True,
        text=True,
        env={**os.environ},
    )


def make_bash_event(command: str) -> dict:
    return {
        "hookEventName": "PreToolUse",
        "tool_name": "Bash",
        "tool_input": {"command": command},
    }


def assert_denied(result: subprocess.CompletedProcess) -> dict:
    """secret-block must emit JSON permissionDecision deny on stdout."""
    assert result.returncode == 0, (
        f"Expected exit 0 (deny via JSON), got {result.returncode}\n"
        f"stdout: {result.stdout}\nstderr: {result.stderr}"
    )
    assert result.stdout.strip(), (
        f"Expected JSON deny on stdout, got empty stdout\nstderr: {result.stderr}"
    )
    out = json.loads(result.stdout.strip())
    decision = (
        out.get("hookSpecificOutput", {}).get("permissionDecision")
        or out.get("permissionDecision")
    )
    assert decision == "deny", f"Expected deny, got {decision!r}\nfull: {out}"
    return out


def assert_allowed(result: subprocess.CompletedProcess) -> None:
    assert result.returncode == 0, (
        f"Expected exit 0 (allow), got {result.returncode}\n"
        f"stdout: {result.stdout}\nstderr: {result.stderr}"
    )
    if result.stdout.strip():
        out = json.loads(result.stdout.strip())
        decision = (
            out.get("hookSpecificOutput", {}).get("permissionDecision")
            or out.get("permissionDecision")
        )
        assert decision != "deny", f"Unexpectedly denied: {out}"


# ---------------------------------------------------------------------------
# AC1: GitHub OAuth token (gho_) → deny
# ---------------------------------------------------------------------------

class TestGitHubOAuthTokenDenied:
    def test_gho_token_denied(self):
        token = "gho_" + "A" * 36
        result = run_hook(make_bash_event(f"curl -H 'Authorization: token {token}'"))
        assert_denied(result)

    def test_gho_minimum_length(self):
        """Minimum 36 chars after prefix."""
        token = "gho_" + "B" * 36
        result = run_hook(make_bash_event(f"echo {token}"))
        assert_denied(result)

    def test_gho_short_not_denied(self):
        """Too short — should not match."""
        token = "gho_" + "C" * 5
        result = run_hook(make_bash_event(f"echo {token}"))
        assert_allowed(result)


# ---------------------------------------------------------------------------
# AC2: GitHub PAT (ghp_) → deny
# ---------------------------------------------------------------------------

class TestGitHubPATDenied:
    def test_ghp_token_denied(self):
        token = "ghp_" + "D" * 36
        result = run_hook(make_bash_event(f"git clone https://{token}@github.com/org/repo"))
        assert_denied(result)


# ---------------------------------------------------------------------------
# AC3: GitHub fine-grained PAT (github_pat_) → deny
# ---------------------------------------------------------------------------

class TestGitHubFineGrainedPATDenied:
    def test_github_pat_denied(self):
        token = "github_pat_" + "E" * 60
        result = run_hook(make_bash_event(f"curl -H 'Authorization: Bearer {token}'"))
        assert_denied(result)


# ---------------------------------------------------------------------------
# AC4: AWS access key (AKIA) → deny
# ---------------------------------------------------------------------------

class TestAWSAccessKeyDenied:
    def test_akia_key_denied(self):
        key = "AKIA" + "F" * 16
        result = run_hook(make_bash_event(f"AWS_ACCESS_KEY_ID={key} aws s3 ls"))
        assert_denied(result)

    def test_asia_session_key_denied(self):
        """ASIA prefix (session key) also matches."""
        key = "ASIA" + "G" * 16
        result = run_hook(make_bash_event(f"export AWS_ACCESS_KEY_ID={key}"))
        assert_denied(result)


# ---------------------------------------------------------------------------
# AC5: Bearer token (via credentialed URL pattern) → deny
# ---------------------------------------------------------------------------

class TestBearerTokenDenied:
    def test_bearer_token_via_credentialed_url(self):
        """Credentialed HTTPS URL (which would include a bearer-style token) → deny.
        patterns.json covers this via the https-credentialed pattern."""
        result = run_hook(make_bash_event(
            "curl https://mytoken:x@api.example.com/endpoint"
        ))
        assert_denied(result)

    def test_jwt_bearer_token_denied(self):
        """JWT token in a command → deny (jwt-token pattern in patterns.json)."""
        # A valid JWT-shaped token: three base64 segments separated by dots
        jwt = "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJ1c2VyMTIzNDU2Nzg5MCJ9.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c"
        result = run_hook(make_bash_event(f"curl -H 'Authorization: Bearer {jwt}'"))
        assert_denied(result)


# ---------------------------------------------------------------------------
# AC6: PEM private key block → deny
# ---------------------------------------------------------------------------

class TestPEMPrivateKeyDenied:
    def test_rsa_private_key_denied(self):
        result = run_hook(make_bash_event("echo '-----BEGIN RSA PRIVATE KEY-----'"))
        assert_denied(result)

    def test_ec_private_key_denied(self):
        result = run_hook(make_bash_event("echo '-----BEGIN EC PRIVATE KEY-----'"))
        assert_denied(result)

    def test_openssh_private_key_denied(self):
        result = run_hook(make_bash_event("echo '-----BEGIN OPENSSH PRIVATE KEY-----'"))
        assert_denied(result)

    def test_generic_private_key_denied(self):
        result = run_hook(make_bash_event("echo '-----BEGIN PRIVATE KEY-----'"))
        assert_denied(result)


# ---------------------------------------------------------------------------
# AC7: clean command → exit 0, no stdout (#180)
# ---------------------------------------------------------------------------

class TestCleanCommandAllowed:
    def test_simple_ls_allowed(self):
        result = run_hook(make_bash_event("ls -la /tmp"))
        assert_allowed(result)
        assert not result.stdout.strip(), f"Expected silent allow, got stdout: {result.stdout!r}"

    def test_git_status_allowed(self):
        result = run_hook(make_bash_event("git status"))
        assert_allowed(result)

    def test_echo_allowed(self):
        result = run_hook(make_bash_event("echo hello world"))
        assert_allowed(result)

    def test_short_token_prefix_allowed(self):
        """Short string with token-like prefix must not be denied."""
        result = run_hook(make_bash_event("echo gho_short"))
        assert_allowed(result)


# ---------------------------------------------------------------------------
# AC8: deny output does NOT echo the full secret value
# ---------------------------------------------------------------------------

class TestDenyDoesNotLeakSecret:
    def test_full_token_not_in_deny_reason(self):
        token = "gho_" + "X" * 36
        result = run_hook(make_bash_event(f"curl -H 'Authorization: token {token}'"))
        out = assert_denied(result)
        reason = out.get("hookSpecificOutput", {}).get("permissionDecisionReason", "")
        assert token not in reason, (
            f"Full secret token was leaked in deny reason!\n"
            f"Token: {token}\nReason: {reason!r}"
        )

    def test_aws_key_not_in_deny_reason(self):
        key = "AKIA" + "Y" * 16
        result = run_hook(make_bash_event(f"AWS_ACCESS_KEY_ID={key} aws s3 ls"))
        out = assert_denied(result)
        reason = out.get("hookSpecificOutput", {}).get("permissionDecisionReason", "")
        assert key not in reason, (
            f"Full AWS key was leaked in deny reason!\nKey: {key}\nReason: {reason!r}"
        )


# ---------------------------------------------------------------------------
# AC9: uses _lib.load_patterns() (not hardcoded inline patterns)
# ---------------------------------------------------------------------------

class TestUsesLibPatterns:
    def test_uses_patterns_json_anthropic_key(self):
        """Anthropic API key pattern (sk-ant-) must be caught via patterns.json."""
        token = "sk-ant-" + "Z" * 30
        result = run_hook(make_bash_event(f"curl -H 'x-api-key: {token}'"))
        assert_denied(result)

    def test_uses_patterns_json_stripe_key(self):
        """Stripe live secret key (sk_live_) must be caught via patterns.json."""
        token = "sk_live_" + "W" * 25
        result = run_hook(make_bash_event(f"STRIPE_KEY={token} node deploy.js"))
        assert_denied(result)

    def test_uses_patterns_json_slack_token(self):
        """Slack token (xoxb-) must be caught via patterns.json."""
        token = "xoxb-" + "1234567890-ABCDEFGHIJ"
        result = run_hook(make_bash_event(f"curl -H 'Authorization: Bearer {token}'"))
        assert_denied(result)


# ---------------------------------------------------------------------------
# Output schema tests
# ---------------------------------------------------------------------------

class TestOutputSchema:
    def test_deny_is_json_not_exit1(self):
        """Deny must be emitted via JSON on stdout (exit 0), not exit 1."""
        token = "gho_" + "A" * 36
        result = run_hook(make_bash_event(f"echo {token}"))
        assert result.returncode == 0, "Deny must be exit 0 + JSON, not exit 1"
        out = json.loads(result.stdout.strip())
        assert "hookSpecificOutput" in out
        assert out["hookSpecificOutput"]["hookEventName"] == "PreToolUse"
        assert out["hookSpecificOutput"]["permissionDecision"] == "deny"
        assert "permissionDecisionReason" in out["hookSpecificOutput"]

    def test_non_bash_tool_allowed(self):
        """Non-Bash tool events must pass through."""
        event = {
            "hookEventName": "PreToolUse",
            "tool_name": "Read",
            "tool_input": {"file_path": "/etc/hosts"},
        }
        result = run_hook(event)
        assert_allowed(result)

    def test_empty_stdin_exits_zero(self):
        result = subprocess.run(
            [sys.executable, SECRET_BLOCK],
            input="",
            capture_output=True,
            text=True,
        )
        assert result.returncode == 0

    def test_malformed_json_exits_zero(self):
        result = subprocess.run(
            [sys.executable, SECRET_BLOCK],
            input="{not valid json",
            capture_output=True,
            text=True,
        )
        assert result.returncode == 0

    def test_wrong_event_type_allowed(self):
        """PostToolUse events are not guarded by secret-block."""
        token = "gho_" + "A" * 36
        event = {
            "hookEventName": "PostToolUse",
            "tool_name": "Bash",
            "tool_input": {"command": f"echo {token}"},
        }
        result = run_hook(event)
        assert_allowed(result)
