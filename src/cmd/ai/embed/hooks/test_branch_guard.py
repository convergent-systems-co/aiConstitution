"""Tests for branch-guard.py drift and violation audit record writing.

Run with: pytest test_branch_guard.py -v
All tests must be RED before the coder touches production code.
"""
from __future__ import annotations

import os
import sys
import tempfile
from pathlib import Path
from unittest.mock import patch

import importlib.util
import pytest

# branch-guard.py uses a hyphen in its filename, which Python's normal import
# mechanism cannot handle. We use importlib to load it by path directly.
HOOKS_DIR = Path(__file__).resolve().parent
_GUARD_PATH = HOOKS_DIR / "branch-guard.py"

_spec = importlib.util.spec_from_file_location("branch_guard", _GUARD_PATH)
branch_guard = importlib.util.module_from_spec(_spec)
sys.modules["branch_guard"] = branch_guard
_spec.loader.exec_module(branch_guard)  # type: ignore[union-attr]


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

def make_ai_root(tmp_path: Path) -> Path:
    """Create a minimal AI_ROOT under tmp_path and return it."""
    ai_root = tmp_path / "ai_root"
    ai_root.mkdir()
    return ai_root


def violations_in(ai_root: Path) -> list[Path]:
    vdir = ai_root / "audit" / "violations"
    if not vdir.is_dir():
        return []
    return sorted(vdir.glob("*.md"))


def drift_in(ai_root: Path) -> list[Path]:
    ddir = ai_root / "audit" / "drift"
    if not ddir.is_dir():
        return []
    return sorted(ddir.glob("*.md"))


# ---------------------------------------------------------------------------
# #204 — branch-guard writes a violation record when the gate blocks
# ---------------------------------------------------------------------------

class TestViolationRecordWritten:
    """deny_operation must create a violation .md under audit/violations/."""

    def test_commit_on_main_writes_violation_file(self, tmp_path):
        """Committing on 'main' must write a violation audit record."""
        ai_root = make_ai_root(tmp_path)

        with (
            patch.dict(os.environ, {"AI_ROOT": str(ai_root)}),
            patch.object(branch_guard, "current_branch", return_value="main"),
        ):
            rc = branch_guard.check_invocation(["commit", "-m", "oops"])

        assert rc == 1, "check_invocation must return 1 (deny) for commit on main"

        files = violations_in(ai_root)
        assert len(files) == 1, (
            f"Expected exactly 1 violation file in {ai_root}/audit/violations/, "
            f"got {len(files)}: {files}"
        )

    def test_violation_file_contains_branch_name(self, tmp_path):
        """The violation file must reference the blocked branch by name."""
        ai_root = make_ai_root(tmp_path)

        with (
            patch.dict(os.environ, {"AI_ROOT": str(ai_root)}),
            patch.object(branch_guard, "current_branch", return_value="main"),
        ):
            branch_guard.check_invocation(["merge", "--no-ff", "feature/x"])

        files = violations_in(ai_root)
        assert files, "no violation file created"
        content = files[0].read_text(encoding="utf-8")
        assert "main" in content, f"violation file should name branch 'main':\n{content}"

    def test_violation_file_contains_subcommand(self, tmp_path):
        """The violation file must name the blocked git subcommand."""
        ai_root = make_ai_root(tmp_path)

        with (
            patch.dict(os.environ, {"AI_ROOT": str(ai_root)}),
            patch.object(branch_guard, "current_branch", return_value="master"),
        ):
            branch_guard.check_invocation(["rebase", "origin/master"])

        files = violations_in(ai_root)
        assert files, "no violation file created"
        content = files[0].read_text(encoding="utf-8")
        assert "rebase" in content, f"violation file should name subcommand 'rebase':\n{content}"

    def test_violation_file_contains_rule_citation(self, tmp_path):
        """The violation file must cite the governing rule (Common.md §2.2)."""
        ai_root = make_ai_root(tmp_path)

        with (
            patch.dict(os.environ, {"AI_ROOT": str(ai_root)}),
            patch.object(branch_guard, "current_branch", return_value="main"),
        ):
            branch_guard.check_invocation(["commit"])

        files = violations_in(ai_root)
        assert files, "no violation file created"
        content = files[0].read_text(encoding="utf-8")
        assert "2.2" in content or "branch-guard" in content.lower(), (
            f"violation file should cite §2.2 or branch-guard:\n{content}"
        )

    def test_no_violation_file_when_branch_is_safe(self, tmp_path):
        """No violation file should be written when the branch is not protected."""
        ai_root = make_ai_root(tmp_path)

        with (
            patch.dict(os.environ, {"AI_ROOT": str(ai_root)}),
            patch.object(branch_guard, "current_branch", return_value="feature/foo"),
        ):
            rc = branch_guard.check_invocation(["commit", "-m", "ok"])

        assert rc == 0, "check_invocation should allow commit on a feature branch"
        files = violations_in(ai_root)
        assert not files, f"unexpected violation files for safe branch: {files}"

    def test_multiple_blocks_create_multiple_files(self, tmp_path):
        """Each blocking call must produce its own violation file."""
        ai_root = make_ai_root(tmp_path)

        with (
            patch.dict(os.environ, {"AI_ROOT": str(ai_root)}),
            patch.object(branch_guard, "current_branch", return_value="main"),
        ):
            branch_guard.check_invocation(["commit"])
            branch_guard.check_invocation(["commit"])

        files = violations_in(ai_root)
        assert len(files) == 2, (
            f"Expected 2 violation files (one per block), got {len(files)}: {files}"
        )


# ---------------------------------------------------------------------------
# #203 — branch-guard writes a drift record on near-miss (≥90% threshold)
# ---------------------------------------------------------------------------

class TestDriftRecordWritten:
    """write_drift_record must create a drift .md under audit/drift/ when
    a near-miss threshold is reached.  The near-miss concept is currently
    a placeholder in branch-guard (no quantitative check exists yet), so
    we test the write_drift_record() function directly."""

    def test_write_drift_record_creates_file(self, tmp_path):
        """write_drift_record() must create a file in audit/drift/."""
        ai_root = make_ai_root(tmp_path)

        with patch.dict(os.environ, {"AI_ROOT": str(ai_root)}):
            branch_guard.write_drift_record(
                subcmd="commit",
                branch="main",
                detail="synthetic near-miss test",
            )

        files = drift_in(ai_root)
        assert len(files) == 1, (
            f"Expected 1 drift file in {ai_root}/audit/drift/, got {len(files)}: {files}"
        )

    def test_write_drift_record_content(self, tmp_path):
        """Drift record must include subcmd, branch, and the word 'drift'."""
        ai_root = make_ai_root(tmp_path)

        with patch.dict(os.environ, {"AI_ROOT": str(ai_root)}):
            branch_guard.write_drift_record(
                subcmd="push",
                branch="release/1.0",
                detail="test near-miss detail",
            )

        files = drift_in(ai_root)
        assert files, "no drift file created"
        content = files[0].read_text(encoding="utf-8")
        assert "push" in content, f"drift file must name subcmd 'push':\n{content}"
        assert "release/1.0" in content, f"drift file must name branch 'release/1.0':\n{content}"

    def test_write_drift_record_function_exists(self):
        """write_drift_record must be importable from branch_guard."""
        assert hasattr(branch_guard, "write_drift_record"), (
            "branch_guard module must export write_drift_record()"
        )
