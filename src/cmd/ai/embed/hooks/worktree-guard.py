#!/usr/bin/env python3
"""hooks/worktree-guard.py — enforce canonical worktree placement.

Implements ~/.ai/Common.md §U17 (Worktree placement). Two canonical
locations only:

  <repo>/.worktrees/<name>/   (single-repo, dies with the repo)
  ~/.ai/worktrees/<name>/     (cross-repo, persistent)

Ad-hoc placement (`../<branch>/`, `/tmp/worktree-X/`, sibling
directories of the repo, etc.) is forbidden.

Triggers on `git worktree add <path> ...`. Other `git worktree`
subcommands (list, remove, prune) are passed through.

Output schema: emit a JSON `permissionDecision: deny` on stdout on
violation, silent (exit 0, empty stdout) otherwise.

Wire into ~/.claude/settings.json under hooks.PreToolUse.

Self-check:
  --self-check
"""
from __future__ import annotations

import json
import os
import shlex
import subprocess
import sys
from pathlib import Path
from typing import List, Optional

sys.path.insert(0, str(Path(__file__).resolve().parent))
import _lib  # noqa: E402

# Flags on `git worktree add` that consume the next token as a value.
# Anything else starting with `-` is treated as value-less.
VALUE_FLAGS = {"-b", "-B", "--reason"}


def deny(reason: str) -> None:
    """Emit a permission-deny decision via JSON stdout and exit 0."""
    print(json.dumps({
        "hookSpecificOutput": {
            "hookEventName": "PreToolUse",
            "permissionDecision": "deny",
            "permissionDecisionReason": reason,
        }
    }))
    sys.exit(0)


def ai_worktrees_root() -> Path:
    """Return the cross-repo canonical worktree root."""
    ai_root = os.environ.get("AI_ROOT", str(Path.home() / ".ai"))
    return Path(ai_root) / "worktrees"


def resolve_repo_root(cwd: str) -> Optional[Path]:
    """Return the repo root for the given working directory, or None."""
    try:
        out = subprocess.run(
            ["git", "-C", cwd, "rev-parse", "--show-toplevel"],
            capture_output=True, text=True, timeout=5,
        )
        if out.returncode == 0 and out.stdout.strip():
            return Path(out.stdout.strip()).resolve()
    except Exception:
        pass
    return None


def find_worktree_add_path(tokens: List[str]) -> Optional[str]:
    """Return the target path for `git worktree add <path>`.

    Returns None if this is not a `git worktree add` invocation or if
    no positional path argument can be located.
    """
    # Expect: git worktree add [flags] <path> [<commit-ish>]
    if len(tokens) < 3:
        return None
    if tokens[0] != "git" or tokens[1] != "worktree" or tokens[2] != "add":
        return None

    i = 3
    while i < len(tokens):
        t = tokens[i]
        if t == "--":
            i += 1
            break
        if not t.startswith("-"):
            # First non-flag positional after `add` is the path.
            return t
        if t in VALUE_FLAGS and i + 1 < len(tokens):
            i += 2
            continue
        # Value-less flag (e.g. --detach, --lock, --quiet)
        i += 1

    if i < len(tokens):
        return tokens[i]
    return None


def is_canonical(raw_path: str, cwd: str) -> bool:
    """Return True if raw_path resolves to a canonical worktree location."""
    target = Path(raw_path)
    if not target.is_absolute():
        target = Path(cwd) / target
    try:
        resolved = target.resolve()
    except Exception:
        resolved = Path(os.path.normpath(str(target)))

    # Check: ~/.ai/worktrees/<name>/ (or $AI_ROOT/worktrees/)
    ai_root = ai_worktrees_root()
    try:
        resolved.relative_to(ai_root.resolve())
        return True
    except (ValueError, FileNotFoundError):
        pass

    # Check: <repo>/.worktrees/<name>/
    repo_root = resolve_repo_root(cwd)
    if repo_root is not None:
        try:
            resolved.relative_to(repo_root / ".worktrees")
            return True
        except ValueError:
            pass

    return False


def main() -> int:
    # --self-check mode
    if "--self-check" in sys.argv:
        return _lib.self_check_ok()

    raw_input = sys.stdin.read()
    if not raw_input.strip():
        return 0

    try:
        event = json.loads(raw_input)
    except json.JSONDecodeError:
        return 0

    hook_event = event.get("hookEventName") or event.get("hook_event_name") or ""
    if hook_event != "PreToolUse":
        return 0

    tool_name = event.get("tool_name") or event.get("toolName") or ""
    if tool_name not in ("Bash", "shell", "execute"):
        return 0

    tool_input = event.get("tool_input") or event.get("toolInput") or {}
    if not isinstance(tool_input, dict):
        return 0

    command = tool_input.get("command") or ""
    if not isinstance(command, str) or "worktree" not in command:
        return 0

    # Use event cwd for repo-root resolution; fall back to process cwd.
    cwd = event.get("cwd") or os.getcwd()

    # Parse the command using shlex for correctness with quoted paths.
    try:
        tokens = shlex.split(command, comments=False, posix=True)
    except ValueError:
        return 0

    target_path = find_worktree_add_path(tokens)
    if target_path is None:
        # Not a `git worktree add` invocation — pass through.
        return 0

    if not is_canonical(target_path, cwd):
        # Compute the shown (resolved) target for the deny message.
        p = Path(target_path)
        if not p.is_absolute():
            p = Path(cwd) / p
        try:
            shown = p.resolve()
        except Exception:
            shown = p

        repo_root = resolve_repo_root(cwd)
        ai_root = ai_worktrees_root()
        canonical_hints = []
        if repo_root:
            canonical_hints.append(f"  {repo_root}/.worktrees/<name>/  — single-repo")
        canonical_hints.append(f"  {ai_root}/<name>/  — cross-repo or persistent")

        hints = "\n".join(canonical_hints)
        deny(
            f"Worktree placement violates Common.md §U17.\n"
            f"  Target: {shown}\n"
            "Canonical roots:\n"
            f"{hints}\n"
            "Choose by lifecycle (§U17.1) and re-run.\n"
            "Preferred surface: `ai worktree add <name>` (with `--global` for cross-repo)."
        )

    return 0


if __name__ == "__main__":
    sys.exit(main())
