#!/usr/bin/env python3
"""hooks/branch-guard.py — deny direct mutation of protected branches.

Implements ~/.ai/Common.md §2.2 (protected-branch gate) and
~/.ai/Common.md §5.5 hook-driven enforcement.

Denied (when current HEAD resolves to a protected branch):
  git commit, merge, rebase, cherry-pick, revert, am, pull

  Exception: `git pull --ff-only` is allowed. The flag makes git refuse
  any non-fast-forward, so the operation can only bring local in line
  with remote canonical state — it cannot rewrite history or invent state.
  Worst case: the pull is rejected and nothing changes.

Denied (regardless of current HEAD):
  git push whose destination refspec targets a protected branch

  Exception: first push to an empty remote (the remote ref does not yet
  exist). This allows the initial `git push -u origin main` from a freshly
  initialised repository. Subsequent pushes to an existing protected ref
  remain blocked.

Default protected names:    main, master
Default protected patterns: release/*

Override via ~/.ai/branch-guard.json:
  { "names": ["main","master","prod"], "patterns": ["release/*","hotfix/*"] }

Output schema: emit a JSON `permissionDecision: deny` on stdout on
violation, silent (exit 0, empty stdout) otherwise.

Wire into ~/.claude/settings.json under hooks.PreToolUse (matcher=Bash).

Self-check:
  --self-check  loads the override JSON (if present), compiles
                the patterns, prints OK.
"""
from __future__ import annotations

import fnmatch
import json
import os
import shlex
import subprocess
import sys
from pathlib import Path
from typing import List, Optional, Tuple

MUTATING_SUBCOMMANDS = {
    "commit", "merge", "rebase", "cherry-pick", "revert", "am", "pull",
}

SHELL_SEPARATORS = {"&&", "||", ";", "|", "&"}

# Shell interpreters whose -c argument is itself a command string to inspect.
SHELL_COMMANDS = {"bash", "sh", "zsh", "dash", "ksh"}

DEFAULT_NAMES = ["main", "master"]
DEFAULT_PATTERNS = ["release/*"]


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


def load_protected() -> Tuple[List[str], List[str]]:
    """Load the branch policy. Falls back to defaults if absent or malformed."""
    cfg = Path.home() / ".ai" / "branch-guard.json"
    if cfg.exists():
        try:
            data = json.loads(cfg.read_text())
            return (
                list(data.get("names", DEFAULT_NAMES)),
                list(data.get("patterns", DEFAULT_PATTERNS)),
            )
        except Exception:
            pass
    return list(DEFAULT_NAMES), list(DEFAULT_PATTERNS)


def is_protected(branch: str, names: List[str], patterns: List[str]) -> bool:
    if branch in names:
        return True
    for p in patterns:
        if fnmatch.fnmatch(branch, p):
            return True
    return False


def current_branch(cwd: str) -> Optional[str]:
    """Resolve HEAD to a branch name in the given working directory."""
    try:
        out = subprocess.run(
            ["git", "-C", cwd, "symbolic-ref", "--short", "HEAD"],
            capture_output=True, text=True, timeout=5,
        )
        if out.returncode == 0:
            return out.stdout.strip()
    except Exception:
        pass
    return None


def split_invocations(command: str, _depth: int = 0) -> List[List[str]]:
    """Split a compound shell command into per-segment token lists.

    Recursively descends into `bash -c '...'` (and sh/zsh/etc.) so that
    git commands nested inside a shell wrapper cannot bypass branch checks.
    Depth-limited to 5 to prevent infinite recursion on adversarial input.
    """
    if _depth > 5:
        return []
    try:
        tokens = shlex.split(command, comments=False, posix=True)
    except ValueError:
        return []
    segments: List[List[str]] = []
    current: List[str] = []
    for t in tokens:
        if t in SHELL_SEPARATORS:
            if current:
                segments.append(current)
                current = []
        else:
            current.append(t)
    if current:
        segments.append(current)

    # Recursively inspect shell invocations: bash -c '...', sh -c '...', etc.
    # A command like `bash -c 'git merge ...'` hides the inner git call from
    # the top-level token scan; descend into the -c argument to catch it.
    expanded: List[List[str]] = []
    for seg in segments:
        if seg and seg[0] in SHELL_COMMANDS:
            i = 1
            while i < len(seg):
                if seg[i] == "-c" and i + 1 < len(seg):
                    inner = seg[i + 1]
                    expanded.extend(split_invocations(inner, _depth + 1))
                    break
                i += 1
            else:
                # No -c flag found — keep the segment (may contain other git calls)
                expanded.append(seg)
        else:
            expanded.append(seg)
    return expanded


def parse_git_call(tokens: List[str]) -> Tuple[Optional[str], List[str], Optional[str]]:
    """Return (subcommand, subcommand_args, override_cwd_from_-C).

    Skips global git flags like `-C <path>`, `-c key=value`, `--git-dir=...`.
    Returns (None, [], None) if not a git invocation.
    """
    if not tokens or tokens[0] != "git":
        return None, [], None

    override_cwd: Optional[str] = None
    i = 1
    while i < len(tokens):
        tok = tokens[i]
        if not tok.startswith("-"):
            break
        if tok == "-C" and i + 1 < len(tokens):
            override_cwd = tokens[i + 1]
            i += 2
            continue
        if tok == "-c" and i + 1 < len(tokens):
            i += 2
            continue
        # --key=value or other value-less flag
        i += 1

    if i >= len(tokens):
        return None, [], override_cwd
    return tokens[i], tokens[i + 1:], override_cwd


def remote_ref_exists(remote: str, branch: str, cwd: str) -> bool:
    """Return True if the given branch already exists on the remote.

    Uses `git ls-remote --exit-code --heads <remote> <branch>`:
      exit 0  → ref found (exists)
      exit 2  → connection succeeded, ref not found (bootstrap allowed)
      other   → error (remote unreachable, not configured, etc.) → fail safe (deny)

    Returns True (exists = deny) on any error so we fail safe.
    """
    try:
        result = subprocess.run(
            ["git", "-C", cwd, "ls-remote", "--exit-code", "--heads", remote, branch],
            capture_output=True, text=True, timeout=10,
        )
        if result.returncode == 0:
            # Ref exists on remote — deny.
            return True
        if result.returncode == 2:
            # git ls-remote --exit-code returns 2 when the ref is not found
            # but the remote was reachable. First push is allowed.
            return False
        # Any other non-zero (128 = remote not configured/unreachable, etc.)
        # → fail safe: assume ref exists so we deny.
        return True
    except Exception:
        # Python exception (git not found, timeout, etc.) → fail safe.
        return True


def push_targets_protected(
    args: List[str],
    cwd: str,
    names: List[str],
    patterns: List[str],
) -> Optional[str]:
    """If `git push` args target a protected branch that already exists on the
    remote, return the branch name. Return None to allow.

    The existence check is the bootstrap carve-out: the very first push to an
    empty remote must be allowed so `git init` + `git push origin main` works
    for new repos.
    """
    # Strip flags to find positional args: remote + refspecs.
    positional: List[str] = []
    for a in args:
        if a.startswith("-"):
            continue
        positional.append(a)

    if len(positional) < 2:
        # No refspec — `git push` / `git push <remote>` defaults to the
        # current branch under push.default=simple (git >= 2.0). Deny if
        # HEAD is protected and the remote ref already exists.
        branch = current_branch(cwd)
        if branch and is_protected(branch, names, patterns):
            remote = positional[0] if positional else "origin"
            if remote_ref_exists(remote, branch, cwd):
                return branch
        return None

    # positional[0] is the remote; positional[1:] are refspecs.
    remote = positional[0]
    for r in positional[1:]:
        stripped = r.lstrip("+")
        dst = stripped.split(":", 1)[1] if ":" in stripped else stripped
        # Strip refs/heads/ prefix if present.
        if dst.startswith("refs/heads/"):
            dst = dst[len("refs/heads/"):]
        if is_protected(dst, names, patterns):
            if remote_ref_exists(remote, dst, cwd):
                return dst
    return None


def main() -> int:
    # --self-check mode (invoked by `ai hooks evaluate` or manually)
    if "--self-check" in sys.argv:
        _ = load_protected()
        print("[ai/branch-guard] self-check OK", file=sys.stderr)
        return 0

    try:
        event = json.load(sys.stdin)
    except Exception:
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
    if not isinstance(command, str) or "git" not in command:
        return 0

    segments = split_invocations(command)
    if not segments:
        return 0

    names, patterns = load_protected()
    base_cwd = event.get("cwd") or os.getcwd()

    for tokens in segments:
        subcommand, subargs, override_cwd = parse_git_call(tokens)
        if subcommand is None:
            continue
        cwd = override_cwd or base_cwd

        if subcommand == "push":
            target = push_targets_protected(subargs, cwd, names, patterns)
            if target:
                deny(
                    f"`git push` would publish to protected branch '{target}'.\n"
                    "Per Common.md §2.2 — direct push to a protected branch "
                    "requires explicit approval.\n"
                    "Push from a feature branch via PR instead.\n"
                    f"  Branch: {target}\n"
                    f"  Cwd:    {cwd}"
                )
            continue

        if subcommand in MUTATING_SUBCOMMANDS:
            branch = current_branch(cwd)
            if branch is None:
                continue
            if is_protected(branch, names, patterns):
                # Carve-out: `git pull --ff-only` is a strict fast-forward
                # sync from remote canonical state, not a mutation. Git
                # itself refuses non-FF, so this cannot rewrite history
                # or invent state.
                if subcommand == "pull" and "--ff-only" in subargs:
                    continue

                # Block --no-verify on protected branches: that flag strips
                # git's own hook layer, removing a second enforcement line.
                if "--no-verify" in subargs or "-n" in subargs:
                    deny(
                        f"`git {subcommand} --no-verify` on protected branch '{branch}'.\n"
                        "--no-verify strips git hooks, which are a second enforcement layer.\n"
                        "Per Common.md §2.2 — direct mutation of a protected branch requires\n"
                        "explicit approval and cannot bypass hooks.\n"
                        f"  Branch: {branch}\n"
                        f"  Cwd:    {cwd}"
                    )

                deny(
                    f"`git {subcommand}` would mutate protected branch '{branch}'.\n"
                    "Per Common.md §2.2 — operations whose blast radius "
                    "extends beyond the current working directory require "
                    "approval. Direct mutations of protected branches "
                    "bypass PR review and change canonical state.\n"
                    f"  Branch: {branch}\n"
                    f"  Cwd:    {cwd}\n"
                    "Move work to a feature branch first:\n"
                    "  git switch -c <feature-branch>"
                )

    return 0


if __name__ == "__main__":
    sys.exit(main())
