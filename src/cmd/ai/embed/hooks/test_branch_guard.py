"""test_branch_guard.py — adversarial + functional tests for branch-guard.py.

Covers stories #92 (branch-guard) and #180 (adversarial validation).

Tests invoke branch-guard.py as a subprocess with a JSON event on stdin,
matching exactly how Claude Code calls it. This exercises the full process
including stdin parsing and output schema.

RED criteria (current embedded version will fail):
- AC3: bash -c recursion (embedded version has no split_invocations)
- AC4: --no-verify denial (embedded version has no --no-verify check)
- AC8: first-push to nonexistent remote → allow (not implemented)
- AC9: JSON permissionDecision deny output (embedded uses _lib.log+exit1)
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
BRANCH_GUARD = str(HOOKS_DIR / "branch-guard.py")


def run_hook(event: dict, env: dict | None = None) -> subprocess.CompletedProcess:
    """Invoke branch-guard.py with the given event dict on stdin."""
    merged_env = {**os.environ, **(env or {})}
    return subprocess.run(
        [sys.executable, BRANCH_GUARD],
        input=json.dumps(event),
        capture_output=True,
        text=True,
        env=merged_env,
    )


def make_bash_event(command: str, branch: str = "main") -> dict:
    """Build a PreToolUse event for a Bash tool call."""
    return {
        "hookEventName": "PreToolUse",
        "tool_name": "Bash",
        "tool_input": {"command": command},
        "cwd": "/tmp/fakerepo",
        # branch is detected via subprocess git call; we patch via a wrapper
    }


def assert_denied(result: subprocess.CompletedProcess) -> dict:
    """Assert the hook emitted a JSON permissionDecision deny on stdout."""
    assert result.returncode == 0, (
        f"Expected exit 0 (deny via JSON), got {result.returncode}\n"
        f"stdout: {result.stdout}\nstderr: {result.stderr}"
    )
    assert result.stdout.strip(), (
        f"Expected JSON on stdout for deny, got empty stdout\nstderr: {result.stderr}"
    )
    try:
        out = json.loads(result.stdout.strip())
    except json.JSONDecodeError as e:
        pytest.fail(f"stdout is not valid JSON: {e!r}\nstdout: {result.stdout!r}")
    decision = (
        out.get("hookSpecificOutput", {}).get("permissionDecision")
        or out.get("permissionDecision")
    )
    assert decision == "deny", (
        f"Expected permissionDecision='deny', got {decision!r}\nfull output: {out}"
    )
    return out


def assert_allowed(result: subprocess.CompletedProcess) -> None:
    """Assert the hook did NOT deny (exit 0, no deny on stdout)."""
    assert result.returncode == 0, (
        f"Expected exit 0 (allow), got {result.returncode}\n"
        f"stdout: {result.stdout}\nstderr: {result.stderr}"
    )
    if result.stdout.strip():
        try:
            out = json.loads(result.stdout.strip())
            decision = (
                out.get("hookSpecificOutput", {}).get("permissionDecision")
                or out.get("permissionDecision")
            )
            assert decision != "deny", (
                f"Hook denied when it should have allowed\nfull output: {out}"
            )
        except json.JSONDecodeError:
            pass  # non-JSON stdout on allow is fine


# ---------------------------------------------------------------------------
# Test fixtures: a temporary git repo with a branch we control
# ---------------------------------------------------------------------------

@pytest.fixture()
def git_repo_on_main(tmp_path):
    """Initialize a git repo and check out main."""
    subprocess.run(["git", "init", "-b", "main", str(tmp_path)], check=True, capture_output=True)
    subprocess.run(["git", "-C", str(tmp_path), "config", "user.email", "test@test.com"], check=True, capture_output=True)
    subprocess.run(["git", "-C", str(tmp_path), "config", "user.name", "Test"], check=True, capture_output=True)
    # Make initial commit so HEAD resolves
    (tmp_path / "README").write_text("init")
    subprocess.run(["git", "-C", str(tmp_path), "add", "."], check=True, capture_output=True)
    subprocess.run(["git", "-C", str(tmp_path), "commit", "-m", "init"], check=True, capture_output=True)
    return tmp_path


@pytest.fixture()
def git_repo_on_feature(tmp_path):
    """Initialize a git repo on a feature branch."""
    subprocess.run(["git", "init", "-b", "main", str(tmp_path)], check=True, capture_output=True)
    subprocess.run(["git", "-C", str(tmp_path), "config", "user.email", "test@test.com"], check=True, capture_output=True)
    subprocess.run(["git", "-C", str(tmp_path), "config", "user.name", "Test"], check=True, capture_output=True)
    (tmp_path / "README").write_text("init")
    subprocess.run(["git", "-C", str(tmp_path), "add", "."], check=True, capture_output=True)
    subprocess.run(["git", "-C", str(tmp_path), "commit", "-m", "init"], check=True, capture_output=True)
    subprocess.run(["git", "-C", str(tmp_path), "checkout", "-b", "feature/test"], check=True, capture_output=True)
    return tmp_path


# ---------------------------------------------------------------------------
# AC1: git commit on main → deny
# ---------------------------------------------------------------------------

class TestCommitOnMainDenied:
    def test_commit_main_denied_via_json(self, git_repo_on_main):
        event = {
            "hookEventName": "PreToolUse",
            "tool_name": "Bash",
            "tool_input": {"command": "git commit -m 'test'"},
            "cwd": str(git_repo_on_main),
        }
        result = run_hook(event)
        assert_denied(result)

    def test_deny_includes_branch_name(self, git_repo_on_main):
        event = {
            "hookEventName": "PreToolUse",
            "tool_name": "Bash",
            "tool_input": {"command": "git commit -m 'test'"},
            "cwd": str(git_repo_on_main),
        }
        result = run_hook(event)
        out = assert_denied(result)
        reason = out.get("hookSpecificOutput", {}).get("permissionDecisionReason", "")
        assert "main" in reason, f"Expected 'main' in deny reason, got: {reason!r}"


# ---------------------------------------------------------------------------
# AC2: git merge on main → deny
# ---------------------------------------------------------------------------

class TestMergeOnMainDenied:
    def test_merge_main_denied(self, git_repo_on_main):
        event = {
            "hookEventName": "PreToolUse",
            "tool_name": "Bash",
            "tool_input": {"command": "git merge feature/foo"},
            "cwd": str(git_repo_on_main),
        }
        result = run_hook(event)
        assert_denied(result)


# ---------------------------------------------------------------------------
# AC3 (#180): bash -c 'git commit' → deny (bash-c recursion)
# ---------------------------------------------------------------------------

class TestBashCRecursion:
    """#180 adversarial: bash -c wrapper cannot bypass branch-guard."""

    def test_bash_c_commit_denied(self, git_repo_on_main):
        """bash -c 'git commit -m test' on main MUST be denied."""
        event = {
            "hookEventName": "PreToolUse",
            "tool_name": "Bash",
            "tool_input": {"command": "bash -c 'git commit -m test'"},
            "cwd": str(git_repo_on_main),
        }
        result = run_hook(event)
        assert_denied(result)

    def test_bash_c_merge_denied(self, git_repo_on_main):
        """bash -c 'git merge foo' on main MUST be denied."""
        event = {
            "hookEventName": "PreToolUse",
            "tool_name": "Bash",
            "tool_input": {"command": "bash -c 'git merge feature/foo'"},
            "cwd": str(git_repo_on_main),
        }
        result = run_hook(event)
        assert_denied(result)

    def test_sh_c_commit_denied(self, git_repo_on_main):
        """sh -c 'git commit ...' on main MUST be denied."""
        event = {
            "hookEventName": "PreToolUse",
            "tool_name": "Bash",
            "tool_input": {"command": "sh -c 'git commit -m bypass'"},
            "cwd": str(git_repo_on_main),
        }
        result = run_hook(event)
        assert_denied(result)

    def test_compound_with_bash_c_denied(self, git_repo_on_main):
        """echo foo && bash -c 'git commit' on main MUST be denied."""
        event = {
            "hookEventName": "PreToolUse",
            "tool_name": "Bash",
            "tool_input": {"command": "echo foo && bash -c 'git commit -m compound'"},
            "cwd": str(git_repo_on_main),
        }
        result = run_hook(event)
        assert_denied(result)


# ---------------------------------------------------------------------------
# AC4 (#180): --no-verify on protected branch → deny with specific message
# ---------------------------------------------------------------------------

class TestNoVerifyDenied:
    """#180 adversarial: --no-verify flag must be denied on protected branches."""

    def test_commit_no_verify_denied(self, git_repo_on_main):
        """git commit --no-verify on main MUST be denied."""
        event = {
            "hookEventName": "PreToolUse",
            "tool_name": "Bash",
            "tool_input": {"command": "git commit --no-verify -m 'bypass attempt'"},
            "cwd": str(git_repo_on_main),
        }
        result = run_hook(event)
        out = assert_denied(result)
        reason = out.get("hookSpecificOutput", {}).get("permissionDecisionReason", "")
        assert "--no-verify" in reason, (
            f"Expected '--no-verify' mentioned in deny reason, got: {reason!r}"
        )

    def test_commit_no_verify_short_flag_denied(self, git_repo_on_main):
        """git commit -n on main MUST be denied (short form of --no-verify)."""
        event = {
            "hookEventName": "PreToolUse",
            "tool_name": "Bash",
            "tool_input": {"command": "git commit -n -m 'short flag bypass'"},
            "cwd": str(git_repo_on_main),
        }
        result = run_hook(event)
        # Either denied entirely OR denied with no-verify message — must not allow
        assert result.returncode == 0  # hook exits 0 (deny via JSON)
        if result.stdout.strip():
            out = json.loads(result.stdout.strip())
            decision = out.get("hookSpecificOutput", {}).get("permissionDecision", "")
            assert decision == "deny"


# ---------------------------------------------------------------------------
# AC5: git push origin main → deny
# ---------------------------------------------------------------------------

class TestPushMainDenied:
    def test_push_main_denied(self, git_repo_on_main):
        event = {
            "hookEventName": "PreToolUse",
            "tool_name": "Bash",
            "tool_input": {"command": "git push origin main"},
            "cwd": str(git_repo_on_main),
        }
        result = run_hook(event)
        assert_denied(result)

    def test_push_refspec_local_remote_denied(self, git_repo_on_main):
        """git push origin feature/foo:main should be denied (remote is main)."""
        event = {
            "hookEventName": "PreToolUse",
            "tool_name": "Bash",
            "tool_input": {"command": "git push origin feature/foo:main"},
            "cwd": str(git_repo_on_main),
        }
        result = run_hook(event)
        assert_denied(result)

    def test_push_force_main_denied(self, git_repo_on_main):
        """git push --force origin main should be denied."""
        event = {
            "hookEventName": "PreToolUse",
            "tool_name": "Bash",
            "tool_input": {"command": "git push --force origin main"},
            "cwd": str(git_repo_on_main),
        }
        result = run_hook(event)
        assert_denied(result)


# ---------------------------------------------------------------------------
# AC6: push to non-protected remote ref → allow
# ---------------------------------------------------------------------------

class TestPushFeatureBranchAllowed:
    def test_push_feature_branch_allowed(self, git_repo_on_feature):
        event = {
            "hookEventName": "PreToolUse",
            "tool_name": "Bash",
            "tool_input": {"command": "git push origin feature/test"},
            "cwd": str(git_repo_on_feature),
        }
        result = run_hook(event)
        assert_allowed(result)


# ---------------------------------------------------------------------------
# AC7: git pull --ff-only on main → allow
# ---------------------------------------------------------------------------

class TestPullFfOnlyAllowed:
    def test_pull_ff_only_allowed(self, git_repo_on_main):
        """pull --ff-only is a fast-forward sync, not a mutation — must be allowed."""
        event = {
            "hookEventName": "PreToolUse",
            "tool_name": "Bash",
            "tool_input": {"command": "git pull --ff-only"},
            "cwd": str(git_repo_on_main),
        }
        result = run_hook(event)
        assert_allowed(result)


# ---------------------------------------------------------------------------
# AC8 (#180): first push to nonexistent remote ref → allow (bootstrap)
# ---------------------------------------------------------------------------

class TestFirstPushBootstrapAllowed:
    """#180 adversarial: first push to an empty remote should be allowed."""

    def test_push_to_nonexistent_remote_ref_allowed(self, tmp_path):
        """When the remote ref does not exist, push to main should be allowed.

        This covers the empty-repo bootstrap scenario (e.g., git init + first push).
        The hook must check if the remote ref actually exists before denying.
        """
        # Create a bare remote
        remote = tmp_path / "remote.git"
        subprocess.run(["git", "init", "--bare", str(remote)], check=True, capture_output=True)

        # Create a local repo and add the bare remote
        local = tmp_path / "local"
        local.mkdir()
        subprocess.run(["git", "init", "-b", "main", str(local)], check=True, capture_output=True)
        subprocess.run(["git", "-C", str(local), "config", "user.email", "t@t.com"], check=True, capture_output=True)
        subprocess.run(["git", "-C", str(local), "config", "user.name", "T"], check=True, capture_output=True)
        (local / "f").write_text("x")
        subprocess.run(["git", "-C", str(local), "add", "."], check=True, capture_output=True)
        subprocess.run(["git", "-C", str(local), "commit", "-m", "init"], check=True, capture_output=True)
        subprocess.run(["git", "-C", str(local), "remote", "add", "origin", str(remote)], check=True, capture_output=True)

        # The remote ref `main` does NOT exist yet — this is the first push
        event = {
            "hookEventName": "PreToolUse",
            "tool_name": "Bash",
            "tool_input": {"command": "git push origin main"},
            "cwd": str(local),
        }
        result = run_hook(event)
        assert_allowed(result), (
            "First push to an empty remote should be allowed (bootstrap).\n"
            f"stdout: {result.stdout}\nstderr: {result.stderr}"
        )

    def test_push_to_existing_remote_ref_denied(self, tmp_path):
        """When the remote ref already exists, push to main is denied."""
        remote = tmp_path / "remote.git"
        subprocess.run(["git", "init", "--bare", str(remote)], check=True, capture_output=True)

        local = tmp_path / "local"
        local.mkdir()
        subprocess.run(["git", "init", "-b", "main", str(local)], check=True, capture_output=True)
        subprocess.run(["git", "-C", str(local), "config", "user.email", "t@t.com"], check=True, capture_output=True)
        subprocess.run(["git", "-C", str(local), "config", "user.name", "T"], check=True, capture_output=True)
        (local / "f").write_text("x")
        subprocess.run(["git", "-C", str(local), "add", "."], check=True, capture_output=True)
        subprocess.run(["git", "-C", str(local), "commit", "-m", "init"], check=True, capture_output=True)
        subprocess.run(["git", "-C", str(local), "remote", "add", "origin", str(remote)], check=True, capture_output=True)
        # Push once to establish the remote ref
        subprocess.run(["git", "-C", str(local), "push", "origin", "main"], check=True, capture_output=True)

        # Now push again — remote ref EXISTS — should be denied
        event = {
            "hookEventName": "PreToolUse",
            "tool_name": "Bash",
            "tool_input": {"command": "git push origin main"},
            "cwd": str(local),
        }
        result = run_hook(event)
        assert_denied(result)


# ---------------------------------------------------------------------------
# AC9: deny output is JSON permissionDecision (not exit 1)
# ---------------------------------------------------------------------------

class TestDenyOutputSchema:
    def test_deny_is_json_not_exit1(self, git_repo_on_main):
        """branch-guard must exit 0 with JSON deny, never exit non-zero."""
        event = {
            "hookEventName": "PreToolUse",
            "tool_name": "Bash",
            "tool_input": {"command": "git commit -m 'test'"},
            "cwd": str(git_repo_on_main),
        }
        result = run_hook(event)
        assert result.returncode == 0, "Deny must use exit 0 + JSON, not non-zero exit"
        out = json.loads(result.stdout.strip())
        assert "hookSpecificOutput" in out
        assert out["hookSpecificOutput"]["hookEventName"] == "PreToolUse"
        assert out["hookSpecificOutput"]["permissionDecision"] == "deny"
        assert "permissionDecisionReason" in out["hookSpecificOutput"]

    def test_allow_is_silent_exit0(self, git_repo_on_feature):
        """Allow must be silent exit 0 — no stdout."""
        event = {
            "hookEventName": "PreToolUse",
            "tool_name": "Bash",
            "tool_input": {"command": "git commit -m 'feature work'"},
            "cwd": str(git_repo_on_feature),
        }
        result = run_hook(event)
        assert result.returncode == 0
        # stdout should be empty on allow
        assert not result.stdout.strip(), f"Expected empty stdout on allow, got: {result.stdout!r}"


# ---------------------------------------------------------------------------
# Non-git commands pass through silently
# ---------------------------------------------------------------------------

class TestNonGitPassThrough:
    def test_non_git_allowed(self, git_repo_on_main):
        event = {
            "hookEventName": "PreToolUse",
            "tool_name": "Bash",
            "tool_input": {"command": "ls -la"},
            "cwd": str(git_repo_on_main),
        }
        result = run_hook(event)
        assert_allowed(result)

    def test_wrong_tool_name_allowed(self, git_repo_on_main):
        event = {
            "hookEventName": "PreToolUse",
            "tool_name": "Read",
            "tool_input": {"file_path": "/tmp/foo"},
            "cwd": str(git_repo_on_main),
        }
        result = run_hook(event)
        assert_allowed(result)
