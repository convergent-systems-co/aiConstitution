#!/usr/bin/env python3
"""hooks/secret-precommit.py — git pre-commit hook (and CI scanner).

Two modes:

  1. Pre-commit (default):  scans the staged diff
     (`git diff --cached -U0`). Aborts the commit on any match.

  2. CI / range scan (`--ci --base BASE --head HEAD`):
     scans the diff from BASE..HEAD. Same matcher; intended for
     .github/workflows/secret-scan.yml.

Reads the canonical pattern set from hooks/patterns.json
(+ patterns.local.json if present). Per SPEC.md §10.2 + §10.4.

Self-check:
  --self-check    Loads patterns.json and compiles every regex.
"""
from __future__ import annotations

import argparse
import subprocess
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
import _lib  # noqa: E402


def staged_diff() -> str:
    """Return the staged diff with 0 lines of context."""
    result = subprocess.run(
        ["git", "diff", "--cached", "-U0", "--no-color"],
        capture_output=True, text=True, check=False,
    )
    if result.returncode != 0:
        _lib.log("git diff --cached failed:", result.stderr.strip())
        return ""
    return result.stdout


def range_diff(base: str, head: str) -> str:
    """Return the diff between two revs, 0 lines of context."""
    result = subprocess.run(
        ["git", "diff", "-U0", "--no-color", f"{base}...{head}"],
        capture_output=True, text=True, check=False,
    )
    if result.returncode != 0:
        _lib.log(f"git diff {base}...{head} failed:", result.stderr.strip())
        return ""
    return result.stdout


def added_lines(diff: str):
    """Yield (file, lineno, content) for every '+'-prefixed line in a
    unified diff (skipping the '+++ b/<file>' filename markers)."""
    cur_file = None
    cur_lineno = 0
    for line in diff.splitlines():
        if line.startswith("+++ "):
            cur_file = line[6:].strip() if line.startswith("+++ b/") else line[4:].strip()
            cur_lineno = 0
            continue
        if line.startswith("@@"):
            # Hunk header: "@@ -a,b +c,d @@"
            try:
                plus = line.split("+", 1)[1].split(" ", 1)[0]
                cur_lineno = int(plus.split(",", 1)[0]) - 1
            except (IndexError, ValueError):
                cur_lineno = 0
            continue
        if line.startswith("+") and not line.startswith("+++"):
            cur_lineno += 1
            yield (cur_file, cur_lineno, line[1:])
        elif not line.startswith("-") and not line.startswith("\\"):
            cur_lineno += 1


def main(argv: list[str]) -> int:
    parser = argparse.ArgumentParser(add_help=True)
    parser.add_argument("--self-check", action="store_true")
    parser.add_argument("--ci", action="store_true",
                        help="CI mode: scan a diff range instead of staged diff")
    parser.add_argument("--base", default=None,
                        help="(CI) base ref")
    parser.add_argument("--head", default="HEAD",
                        help="(CI) head ref")
    args = parser.parse_args(argv)

    if args.self_check:
        return _lib.self_check_ok()

    if args.ci:
        if not args.base:
            _lib.log("--ci requires --base")
            return 2
        diff = range_diff(args.base, args.head)
    else:
        diff = staged_diff()

    if not diff.strip():
        return 0

    patterns = _lib.load_patterns()
    findings = []
    for file, lineno, content in added_lines(diff):
        for entry in patterns:
            for m in entry["_compiled"].finditer(content):
                findings.append({
                    "file": file,
                    "line": lineno,
                    "pattern": entry["id"],
                    "severity": entry.get("severity", "medium"),
                    "snippet": _lib.redact_snippet(content, m.start(), m.end(),
                                                  entry.get("redaction", "[REDACTED]")),
                })

    if not findings:
        return 0

    _lib.log(f"{len(findings)} secret-like match(es) in the diff. Aborting commit.")
    for f in findings[:25]:
        _lib.log(
            f"  {f['file']}:{f['line']}  pattern={f['pattern']} severity={f['severity']}"
        )
        _lib.log(f"      {f['snippet']}")
    if len(findings) > 25:
        _lib.log(f"  ... and {len(findings) - 25} more")
    _lib.log("")
    _lib.log("Per Common.md §1.P4 (no secrets in artifacts; non-overridable).")
    _lib.log("To fix: remove the secret-shaped content, OR add an exception to")
    _lib.log("hooks/patterns.local.json if this is a false positive (and please")
    _lib.log("file a `finding` issue so the false-positive class can be tracked).")
    return 1


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
