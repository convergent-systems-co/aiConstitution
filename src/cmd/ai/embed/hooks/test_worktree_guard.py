"""test_worktree_guard.py — tests for worktree-guard.py (story #93 + #180).

Tests invoke worktree-guard.py as a subprocess with a JSON event on stdin.

RED criteria (current embedded version will fail):
- AC6: embedded version uses exit 1 + _lib.log() instead of JSON permissionDecision
- AC4/AC5: the is_canonical() logic in embedded uses subprocess `repo_root()` without
  the -C flag (uses CWD which may differ from event cwd)
- Compound command handling: embedded uses cmd.split() not shlex (fragile)
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
WORKTREE_GUARD = str(HOOKS_DIR / "worktree-guard.py")


def run_hook(event: dict, env: dict | None = None) -> subprocess.CompletedProcess:
    merged = {**os.environ, **(env or {})}
    return subprocess.run(
        [sys.executable, WORKTREE_GUARD],
        input=json.dumps(event),
        capture_output=True,
        text=True,
        env=merged,
    )


def make_event(command: str, cwd: str | None = None) -> dict:
    return {
        "hookEventName": "PreToolUse",
        "tool_name": "Bash",
        "tool_input": {"command": command},
        "cwd": cwd or "/tmp/fakerepo",
    }


def assert_denied(result: subprocess.CompletedProcess) -> dict:
    assert result.returncode == 0, (
        f"Expected exit 0 (deny via JSON), got {result.returncode}\n"
        f"stdout: {result.stdout}\nstderr: {result.stderr}"
    )
    assert result.stdout.strip(), (
        f"Expected JSON on stdout, got empty\nstderr: {result.stderr}"
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


@pytest.fixture()
def git_repo(tmp_path):
    """Minimal git repo for canonical path tests."""
    subprocess.run(["git", "init", str(tmp_path)], check=True, capture_output=True)
    return tmp_path


# ---------------------------------------------------------------------------
# AC1: /tmp/foo → deny (#180)
# ---------------------------------------------------------------------------

class TestAdHocPathsDenied:
    def test_tmp_path_denied(self, git_repo):
        result = run_hook(make_event("git worktree add /tmp/my-worktree", cwd=str(git_repo)))
        assert_denied(result)

    def test_tmp_subdirectory_denied(self, git_repo):
        result = run_hook(make_event("git worktree add /tmp/feature/foo", cwd=str(git_repo)))
        assert_denied(result)

    def test_var_tmp_denied(self, git_repo):
        result = run_hook(make_event("git worktree add /var/tmp/worktree", cwd=str(git_repo)))
        assert_denied(result)


# ---------------------------------------------------------------------------
# AC2: ../sibling → deny
# ---------------------------------------------------------------------------

class TestRelativeSiblingDenied:
    def test_dotdot_sibling_denied(self, git_repo):
        result = run_hook(make_event("git worktree add ../sibling-worktree", cwd=str(git_repo)))
        assert_denied(result)

    def test_dotdot_slash_denied(self, git_repo):
        result = run_hook(make_event("git worktree add ../foo/bar", cwd=str(git_repo)))
        assert_denied(result)


# ---------------------------------------------------------------------------
# AC3: .git/worktrees/<name> → deny
# ---------------------------------------------------------------------------

class TestDotGitWorktreesDenied:
    def test_git_worktrees_dir_denied(self, git_repo):
        bad_path = str(git_repo / ".git" / "worktrees" / "myname")
        result = run_hook(make_event(f"git worktree add {bad_path}", cwd=str(git_repo)))
        assert_denied(result)


# ---------------------------------------------------------------------------
# AC4: <repo>/.worktrees/<name> → allow (#180)
# ---------------------------------------------------------------------------

class TestCanonicalRepoDotWorktreesAllowed:
    def test_repo_worktrees_subdir_allowed(self, git_repo):
        canonical = str(git_repo / ".worktrees" / "feature-foo")
        result = run_hook(make_event(f"git worktree add {canonical}", cwd=str(git_repo)))
        assert_allowed(result)

    def test_repo_worktrees_nested_allowed(self, git_repo):
        canonical = str(git_repo / ".worktrees" / "feature" / "bar")
        result = run_hook(make_event(f"git worktree add {canonical}", cwd=str(git_repo)))
        assert_allowed(result)

    def test_repo_worktrees_with_branch_flag_allowed(self, git_repo):
        """git worktree add -b branch <canonical-path> should be allowed."""
        canonical = str(git_repo / ".worktrees" / "my-branch")
        result = run_hook(make_event(f"git worktree add -b mybranch {canonical}", cwd=str(git_repo)))
        assert_allowed(result)


# ---------------------------------------------------------------------------
# AC5: ~/.ai/worktrees/<name> → allow (#180)
# ---------------------------------------------------------------------------

class TestCanonicalAiWorktreesAllowed:
    def test_ai_worktrees_allowed(self, tmp_path):
        ai_root = str(tmp_path / ".ai")
        canonical = str(Path(ai_root) / "worktrees" / "cross-repo-thing")

        # Create a git repo for CWD
        repo = tmp_path / "repo"
        repo.mkdir()
        subprocess.run(["git", "init", str(repo)], check=True, capture_output=True)

        result = run_hook(
            make_event(f"git worktree add {canonical}", cwd=str(repo)),
            env={"AI_ROOT": ai_root},
        )
        assert_allowed(result)

    def test_home_ai_worktrees_allowed(self, git_repo):
        """~/.ai/worktrees/<name> is canonical."""
        home = Path.home()
        canonical = str(home / ".ai" / "worktrees" / "global-worktree")
        result = run_hook(make_event(f"git worktree add {canonical}", cwd=str(git_repo)))
        assert_allowed(result)


# ---------------------------------------------------------------------------
# AC6: deny output is JSON permissionDecision
# ---------------------------------------------------------------------------

class TestDenyOutputSchema:
    def test_deny_is_json(self, git_repo):
        result = run_hook(make_event("git worktree add /tmp/bad", cwd=str(git_repo)))
        assert result.returncode == 0, "Deny must be exit 0 + JSON"
        out = json.loads(result.stdout.strip())
        assert "hookSpecificOutput" in out
        assert out["hookSpecificOutput"]["permissionDecision"] == "deny"
        assert "permissionDecisionReason" in out["hookSpecificOutput"]

    def test_deny_reason_mentions_canonical_paths(self, git_repo):
        result = run_hook(make_event("git worktree add /tmp/bad", cwd=str(git_repo)))
        out = assert_denied(result)
        reason = out["hookSpecificOutput"]["permissionDecisionReason"]
        # Reason must mention at least one canonical path
        assert ".worktrees" in reason or "worktrees" in reason, (
            f"Deny reason should mention canonical paths: {reason!r}"
        )

    def test_allow_is_silent(self, git_repo):
        canonical = str(git_repo / ".worktrees" / "ok")
        result = run_hook(make_event(f"git worktree add {canonical}", cwd=str(git_repo)))
        assert result.returncode == 0
        assert not result.stdout.strip(), f"Expected silent allow, got: {result.stdout!r}"


# ---------------------------------------------------------------------------
# Non-worktree-add commands pass through
# ---------------------------------------------------------------------------

class TestPassThrough:
    def test_worktree_list_allowed(self, git_repo):
        result = run_hook(make_event("git worktree list", cwd=str(git_repo)))
        assert_allowed(result)

    def test_worktree_remove_allowed(self, git_repo):
        result = run_hook(make_event("git worktree remove .worktrees/old", cwd=str(git_repo)))
        assert_allowed(result)

    def test_non_git_allowed(self, git_repo):
        result = run_hook(make_event("ls -la", cwd=str(git_repo)))
        assert_allowed(result)

    def test_git_commit_allowed(self, git_repo):
        result = run_hook(make_event("git commit -m test", cwd=str(git_repo)))
        assert_allowed(result)

    def test_empty_stdin_exits_zero(self):
        result = subprocess.run(
            [sys.executable, WORKTREE_GUARD],
            input="",
            capture_output=True,
            text=True,
        )
        assert result.returncode == 0
