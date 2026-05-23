"""Shared helpers for the hooks at this directory.

Every hook is expected to:

  - Be runnable on stdlib python3 only (no third-party deps).
  - Honor --self-check, returning exit 0 on the canonical self-test
    and exit 1 otherwise. The self-check verifies the hook can load
    its dependencies, finds its config files, and compiles its
    regexes. It does NOT exercise the hook's main behavior.
  - Print structured `[ai/<hook-name>]` lines to stderr, never to
    stdout (stdout is reserved for downstream tools).
  - Redact secrets before logging — `_lib.redact()` does this.

Per SPEC.md §3.10 and §10."""

from __future__ import annotations

import json
import os
import re
import sys
from pathlib import Path
from typing import Iterable

HOOKS_DIR = Path(__file__).resolve().parent


def patterns_path() -> Path:
    """Return the path to hooks/patterns.json, honoring ${AI_ROOT}/hooks/
    when invoked from a wrapper outside the repo tree."""
    candidates = [
        HOOKS_DIR / "patterns.json",
        Path(os.environ.get("AI_ROOT", str(Path.home() / ".ai"))) / "hooks" / "patterns.json",
    ]
    for p in candidates:
        if p.is_file():
            return p
    return HOOKS_DIR / "patterns.json"


def patterns_local_path() -> Path:
    """Return the optional local-patterns path; may not exist."""
    return patterns_path().with_name("patterns.local.json")


def load_patterns() -> list[dict]:
    """Load patterns.json plus optional patterns.local.json. Returns a
    list of pattern dicts (the union of both). Local entries override
    canonical entries with the same id."""
    base: list[dict] = []
    p = patterns_path()
    if p.is_file():
        data = json.loads(p.read_text(encoding="utf-8"))
        base = data.get("patterns", [])

    local: list[dict] = []
    lp = patterns_local_path()
    if lp.is_file():
        data = json.loads(lp.read_text(encoding="utf-8"))
        local = data.get("patterns", [])

    # local overrides by id
    by_id = {p["id"]: p for p in base}
    for entry in local:
        by_id[entry["id"]] = entry
    out = list(by_id.values())
    # compile regexes once for the lifetime of the process
    for entry in out:
        entry["_compiled"] = re.compile(entry["regex"])
    return out


def scan_lines(lines: Iterable[str], patterns: list[dict]) -> list[dict]:
    """Walk every line through every pattern; return a list of hits."""
    hits = []
    for lineno, line in enumerate(lines, start=1):
        for entry in patterns:
            for m in entry["_compiled"].finditer(line):
                hits.append({
                    "pattern_id": entry["id"],
                    "severity": entry.get("severity", "medium"),
                    "redaction": entry.get("redaction", "[REDACTED]"),
                    "line": lineno,
                    "col": m.start() + 1,
                    "snippet": redact_snippet(line, m.start(), m.end(), entry.get("redaction", "[REDACTED]")),
                })
    return hits


def redact(input_str: str, patterns: list[dict] | None = None) -> str:
    """Apply every pattern's redaction to the input."""
    if patterns is None:
        patterns = load_patterns()
    out = input_str
    for entry in patterns:
        out = entry["_compiled"].sub(entry.get("redaction", "[REDACTED]"), out)
    return out


def redact_snippet(line: str, start: int, end: int, redaction: str, ctx: int = 20) -> str:
    a = max(0, start - ctx)
    b = min(len(line), end + ctx)
    return line[a:start] + redaction + line[end:b]


def hook_name() -> str:
    """Best-effort hook name derived from argv[0]. Used in log prefixes."""
    base = os.path.basename(sys.argv[0]) if sys.argv else "hook"
    if base.endswith(".py"):
        base = base[:-3]
    return base


def log(*parts) -> None:
    """Structured stderr log line."""
    print(f"[ai/{hook_name()}]", *parts, file=sys.stderr, flush=True)


def self_check_ok() -> int:
    """The default --self-check pass. Hooks override or extend this."""
    try:
        load_patterns()
    except Exception as e:
        log("self-check FAIL:", e)
        return 1
    log("self-check OK")
    return 0
