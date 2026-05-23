#!/usr/bin/env python3
"""hooks/branch-guard.py — deny direct mutation of protected branches.

Implements ~/.ai/Common.md §2.2 (protected-branch gate) and
~/.ai/Common.md §5.5 hook-driven enforcement.

Default protected set: main, master, release/*.
Override list at ~/.ai/governance/policy/branch-guard.json:
    {"names": ["main", "master"], "patterns": ["release/*"]}

Triggers on `git {commit,merge,rebase,cherry-pick,revert,am,pull}`
when HEAD resolves to a protected branch, and on `git push` whose
refspec targets one.

Two invocation modes:

  - **Claude PreToolUse**:  reads the tool payload on stdin (JSON).
    Inspects the command for `git` invocations and exits 1 on
    violation.

  - **Command-wrapper preHook**: invoked by ~/.ai/bin/git via
    hooks/command-wrappers.toml. Receives argv as positional args.

Self-check:
  --self-check  loads the override JSON (if present), compiles
                the patterns, prints OK.
"""
from __future__ import annotations

import argparse
import fnmatch
import json
import os
import subprocess
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
import _lib  # noqa: E402


GUARDED_SUBCOMMANDS = {
    "commit", "merge", "rebase", "cherry-pick", "revert", "am", "pull",
}


def policy_path() -> Path:
    root = os.environ.get("AI_ROOT", str(Path.home() / ".ai"))
    return Path(root) / "governance" / "policy" / "branch-guard.json"


def load_policy() -> dict:
    """Return {"names": [...], "patterns": [...]}. Defaults if absent."""
    p = policy_path()
    if p.is_file():
        try:
            return json.loads(p.read_text(encoding="utf-8"))
        except json.JSONDecodeError as e:
            _lib.log("branch-guard policy parse error:", e)
    return {"names": ["main", "master"], "patterns": ["release/*"]}


def is_protected(branch: str, policy: dict) -> bool:
    if not branch:
        return False
    if branch in (policy.get("names") or []):
        return True
    for pat in policy.get("patterns") or []:
        if fnmatch.fnmatch(branch, pat):
            return True
    return False


def current_branch() -> str:
    try:
        result = subprocess.run(
            ["git", "symbolic-ref", "--short", "HEAD"],
            capture_output=True, text=True, check=False,
        )
        if result.returncode == 0:
            return result.stdout.strip()
    except FileNotFoundError:
        pass
    return ""


def parse_push_refspec(args: list[str]) -> list[str]:
    """Return the list of destination ref names targeted by a `git push`.

    Recognizes: `git push origin main`, `git push origin local:remote`,
    `git push --all`, and bare `git push` (which pushes the current
    branch's tracking ref). Best-effort; the goal is to refuse the
    common case, not to match git's full refspec grammar."""
    # Filter out flags to find positional args.
    pos = [a for a in args if not a.startswith("-")]
    if not pos:
        # Bare `git push` — destination is the current branch's
        # upstream, equivalent to the current branch.
        return [current_branch()]
    # First positional is the remote (typically); subsequent are
    # refspecs.
    refspecs = pos[1:] if len(pos) >= 2 else [current_branch()]
    targets = []
    for r in refspecs:
        # local:remote → take the right-hand side.
        if ":" in r:
            targets.append(r.split(":", 1)[1])
        else:
            targets.append(r)
    return [t for t in targets if t]


def violation(subcmd: str, branch: str) -> int:
    _lib.log(f"blocking — `git {subcmd}` would mutate protected branch '{branch}'.")
    _lib.log("Per ~/.ai/Common.md §2.2 (protected-branch gate).")
    _lib.log("Resolution: branch off (e.g. `git checkout -b work/<slug>`), commit there, and open a PR.")
    return 1


def from_claude_payload() -> int:
    """PreToolUse mode: read JSON payload on stdin, inspect command."""
    raw = sys.stdin.read()
    if not raw.strip():
        return 0
    try:
        payload = json.loads(raw)
    except json.JSONDecodeError:
        return 0
    cmd = (
        payload.get("command")
        or payload.get("tool_input", {}).get("command")
        or payload.get("input", {}).get("command")
        or ""
    ) if isinstance(payload, dict) else ""
    if not cmd.strip().startswith("git "):
        return 0
    # Tokenize roughly. We don't need shell-exact tokenization; the
    # subcommand is the second word.
    parts = cmd.split()
    if len(parts) < 2:
        return 0
    return check_invocation(parts[1:])


def check_invocation(argv: list[str]) -> int:
    """Common code for command-wrapper and PreToolUse paths.

    argv is the args AFTER `git` (so argv[0] is the subcommand)."""
    if not argv:
        return 0
    subcmd = argv[0]
    policy = load_policy()
    branch = current_branch()

    if subcmd in GUARDED_SUBCOMMANDS and is_protected(branch, policy):
        return violation(subcmd, branch)

    if subcmd == "push":
        for target in parse_push_refspec(argv[1:]):
            if is_protected(target, policy):
                return violation("push", target)

    return 0


def main(argv: list[str]) -> int:
    parser = argparse.ArgumentParser(add_help=True)
    parser.add_argument("--self-check", action="store_true")
    parser.add_argument("--mode", choices=["claude", "wrapper"], default=None,
                        help="invocation mode (auto-detected by default)")
    parser.add_argument("rest", nargs=argparse.REMAINDER)
    args = parser.parse_args(argv)

    if args.self_check:
        _ = load_policy()
        return _lib.self_check_ok()

    # Auto-detect: if there's data on stdin, it's a Claude payload.
    if args.mode == "claude" or (args.mode is None and not sys.stdin.isatty()):
        return from_claude_payload()

    # Wrapper mode: argv is the git command line after "git".
    return check_invocation(args.rest)


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
