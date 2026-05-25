"""Unit tests for hooks/branch-guard.py.

Verifies the protected-branch enforcement gate (~/.ai/Common.md §2.2 +
§5.5 hook-driven enforcement). Tests drive the hook in two modes:

  - **Direct invocation** of check_invocation() from a real git repo on
    a real branch — proves the policy/branch-resolution path end to end.
  - **PreToolUse-style** JSON payloads piped to the hook as a subprocess,
    proving the Claude Code wiring.

The hook file's name is hyphenated (branch-guard.py), so we load it via
importlib rather than the bare `import` statement.
"""
from __future__ import annotations

import importlib.util
import json
import os
import subprocess
import sys
from pathlib import Path

import pytest


HOOK_PATH = Path(__file__).resolve().parent.parent / "branch-guard.py"


# ---------------------------------------------------------------------------
# Module loader (hyphenated filename)
# ---------------------------------------------------------------------------

def _load_module():
    spec = importlib.util.spec_from_file_location("branch_guard", HOOK_PATH)
    mod = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(mod)
    return mod


@pytest.fixture(scope="module")
def bg():
    return _load_module()


# ---------------------------------------------------------------------------
# Git fixtures
# ---------------------------------------------------------------------------

def _git(repo, *args, env=None):
    full_env = {**os.environ, **(env or {})}
    return subprocess.run(
        ["git", *args],
        cwd=repo,
        capture_output=True,
        text=True,
        env=full_env,
        check=True,
    )


@pytest.fixture
def repo_on_main(tmp_path, monkeypatch):
    """A real git repo with HEAD on a protected branch (`main`)."""
    repo = tmp_path / "repo"
    repo.mkdir()
    _git(repo, "init", "--initial-branch=main")
    _git(repo, "config", "user.email", "t@example.com")
    _git(repo, "config", "user.name", "T")
    (repo / "f").write_text("x")
    _git(repo, "add", "f")
    _git(repo, "commit", "-m", "init")
    monkeypatch.chdir(repo)
    return repo


@pytest.fixture
def repo_on_feature(tmp_path, monkeypatch):
    """A real git repo with HEAD on a non-protected branch."""
    repo = tmp_path / "repo"
    repo.mkdir()
    _git(repo, "init", "--initial-branch=main")
    _git(repo, "config", "user.email", "t@example.com")
    _git(repo, "config", "user.name", "T")
    (repo / "f").write_text("x")
    _git(repo, "add", "f")
    _git(repo, "commit", "-m", "init")
    _git(repo, "checkout", "-b", "work/feature")
    monkeypatch.chdir(repo)
    return repo


@pytest.fixture
def isolated_ai_root(tmp_path, monkeypatch):
    """Point AI_ROOT at a per-test tmp dir so we test against default policy
    unless a test explicitly seeds an override file."""
    root = tmp_path / "ai_root"
    root.mkdir()
    monkeypatch.setenv("AI_ROOT", str(root))
    return root


# ---------------------------------------------------------------------------
# Tests — guarded subcommands on protected branch (5 required)
# ---------------------------------------------------------------------------

@pytest.mark.parametrize(
    "subcmd",
    ["commit", "merge", "rebase", "cherry-pick", "revert", "am", "pull"],
)
def test_blocks_guarded_subcommand_on_protected_branch(
    bg, repo_on_main, isolated_ai_root, subcmd
):
    rc = bg.check_invocation([subcmd])
    assert rc == 1, f"expected `git {subcmd}` to be blocked on main"


@pytest.mark.parametrize(
    "subcmd",
    ["commit", "merge", "rebase", "cherry-pick", "revert", "am", "pull"],
)
def test_allows_guarded_subcommand_on_feature_branch(
    bg, repo_on_feature, isolated_ai_root, subcmd
):
    rc = bg.check_invocation([subcmd])
    assert rc == 0, f"`git {subcmd}` must be allowed on a non-protected branch"


# ---------------------------------------------------------------------------
# Tests — push refspec parsing
# ---------------------------------------------------------------------------

def test_push_explicit_protected_target_blocked(bg, repo_on_feature, isolated_ai_root):
    # On a feature branch, but the push refspec names main → block.
    rc = bg.check_invocation(["push", "origin", "main"])
    assert rc == 1


def test_push_local_colon_remote_blocked_when_remote_is_protected(
    bg, repo_on_feature, isolated_ai_root
):
    rc = bg.check_invocation(["push", "origin", "work/feature:main"])
    assert rc == 1


def test_push_release_pattern_blocked(bg, repo_on_feature, isolated_ai_root):
    rc = bg.check_invocation(["push", "origin", "release/2026.05"])
    assert rc == 1


def test_bare_push_on_protected_branch_blocked(bg, repo_on_main, isolated_ai_root):
    rc = bg.check_invocation(["push"])
    assert rc == 1


def test_bare_push_on_feature_branch_allowed(bg, repo_on_feature, isolated_ai_root):
    rc = bg.check_invocation(["push"])
    assert rc == 0


def test_push_to_non_protected_target_allowed(bg, repo_on_feature, isolated_ai_root):
    rc = bg.check_invocation(["push", "origin", "work/feature"])
    assert rc == 0


# ---------------------------------------------------------------------------
# Tests — policy override file
# ---------------------------------------------------------------------------

def test_policy_override_extends_protected_names(
    bg, repo_on_feature, isolated_ai_root, monkeypatch
):
    """If branch-guard.json adds `work/feature` to names, a commit on it
    should be blocked."""
    policy_dir = isolated_ai_root / "governance" / "policy"
    policy_dir.mkdir(parents=True)
    (policy_dir / "branch-guard.json").write_text(
        json.dumps({"names": ["main", "master", "work/feature"], "patterns": []})
    )
    rc = bg.check_invocation(["commit"])
    assert rc == 1


def test_policy_override_removes_default_protection(
    bg, repo_on_main, isolated_ai_root
):
    """If the policy override drops `main` from names, commit on main
    should be allowed."""
    policy_dir = isolated_ai_root / "governance" / "policy"
    policy_dir.mkdir(parents=True)
    (policy_dir / "branch-guard.json").write_text(
        json.dumps({"names": ["master"], "patterns": []})
    )
    rc = bg.check_invocation(["commit"])
    assert rc == 0


def test_corrupt_policy_falls_back_to_defaults(
    bg, repo_on_main, isolated_ai_root
):
    policy_dir = isolated_ai_root / "governance" / "policy"
    policy_dir.mkdir(parents=True)
    (policy_dir / "branch-guard.json").write_text("not json {")
    rc = bg.check_invocation(["commit"])
    # default policy still protects `main`.
    assert rc == 1


# ---------------------------------------------------------------------------
# Tests — block message
# ---------------------------------------------------------------------------

def test_block_message_cites_common_md(bg, repo_on_main, isolated_ai_root, capsys):
    bg.check_invocation(["commit"])
    captured = capsys.readouterr()
    assert "Common.md" in captured.err
    assert "§2.2" in captured.err
    assert "main" in captured.err


# ---------------------------------------------------------------------------
# Tests — PreToolUse subprocess mode
# ---------------------------------------------------------------------------

def test_pretooluse_blocks_commit(repo_on_main, isolated_ai_root):
    payload = {"tool_input": {"command": "git commit -m 'x'"}}
    proc = subprocess.run(
        [sys.executable, str(HOOK_PATH)],
        input=json.dumps(payload),
        capture_output=True,
        text=True,
        cwd=str(repo_on_main),
        env={**os.environ, "AI_ROOT": str(isolated_ai_root)},
    )
    assert proc.returncode == 1
    assert "Common.md" in proc.stderr


def test_pretooluse_ignores_non_git_command(repo_on_main, isolated_ai_root):
    payload = {"tool_input": {"command": "ls -la"}}
    proc = subprocess.run(
        [sys.executable, str(HOOK_PATH)],
        input=json.dumps(payload),
        capture_output=True,
        text=True,
        cwd=str(repo_on_main),
        env={**os.environ, "AI_ROOT": str(isolated_ai_root)},
    )
    assert proc.returncode == 0


def test_pretooluse_ignores_safe_git_command(repo_on_main, isolated_ai_root):
    """`git status` on main is not a mutation; the hook must let it through."""
    payload = {"tool_input": {"command": "git status"}}
    proc = subprocess.run(
        [sys.executable, str(HOOK_PATH)],
        input=json.dumps(payload),
        capture_output=True,
        text=True,
        cwd=str(repo_on_main),
        env={**os.environ, "AI_ROOT": str(isolated_ai_root)},
    )
    assert proc.returncode == 0
