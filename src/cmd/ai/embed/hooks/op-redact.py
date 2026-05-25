#!/usr/bin/env python3
"""hooks/op-redact.py — PreToolUse hook that redacts 1Password and other
secret-shaped values from Claude Code tool-use payloads.

Unlike secret-block.py (which DENIES the tool call), this hook:
  - Redacts matching strings in-place across all string fields.
  - Writes a violation record to $AI_ROOT/audit/violations/<UTC>-secret-detected.md.
  - Outputs the cleaned JSON to stdout.
  - ALWAYS exits 0 — it never blocks the tool call.

Per Common.md §4 (non-overridable: secrets must not appear in artifacts)
and SPEC.md §10.1.

Redaction patterns (inline — no dependency on patterns.json):
  gho_ / ghp_ / ghu_ / ghs_ / ghr_  + 36+ chars → [REDACTED:github-token]
  github_pat_ + 60+ chars             → [REDACTED:github-token]
  Bearer  + 20+ chars                 → [REDACTED:bearer-token]
  op://                               → [REDACTED:op-ref]
  sk- + 40+ chars                     → [REDACTED:openai-key]
  -----BEGIN                          → [REDACTED:pem-block]

Input contract (Claude Code PreToolUse event):
  - Full tool-use payload arrives on stdin as JSON.
  - Exit 0 always (redact + log, never block).
  - stdout: cleaned JSON.
  - stderr: human-readable diagnostic lines.

Self-check:
  --self-check  Verifies regex compilation; exits 0 on success.
"""
from __future__ import annotations

import json
import os
import re
import sys
from datetime import datetime, timezone
from pathlib import Path

# ---------------------------------------------------------------------------
# Inline redaction patterns — no dependency on patterns.json.
# Each tuple: (compiled_regex, replacement_string, kind_label)
# ---------------------------------------------------------------------------

_RAW_PATTERNS: list[tuple[str, str, str]] = [
    # GitHub classic tokens
    (r"(ghp|gho|ghu|ghs|ghr)_[A-Za-z0-9]{36,}", "[REDACTED:github-token]", "github-token"),
    # GitHub fine-grained PAT
    (r"github_pat_[A-Za-z0-9_]{60,}", "[REDACTED:github-token]", "github-token"),
    # Bearer tokens (20+ chars after 'Bearer ')
    (r"Bearer [A-Za-z0-9._\-]{20,}", "[REDACTED:bearer-token]", "bearer-token"),
    # op:// references — redact the entire op:// URI
    (r"op://[^\s\"']+", "[REDACTED:op-ref]", "op-ref"),
    # OpenAI-style keys: sk- followed by 40+ chars (excludes sk-ant- Anthropic keys
    # which have their own label, but this is belt-and-suspenders)
    (r"sk-[A-Za-z0-9_\-]{40,}", "[REDACTED:openai-key]", "openai-key"),
    # PEM block headers
    (r"-----BEGIN[^\n\r]*", "[REDACTED:pem-block]", "pem-block"),
]

_PATTERNS: list[tuple[re.Pattern, str, str]] = [
    (re.compile(raw), repl, kind)
    for raw, repl, kind in _RAW_PATTERNS
]


def redact_string(value: str) -> tuple[str, list[str]]:
    """Apply all patterns to value. Returns (redacted_value, list_of_kind_hits)."""
    hits: list[str] = []
    out = value
    for pattern, replacement, kind in _PATTERNS:
        new, n = pattern.subn(replacement, out)
        if n > 0:
            hits.append(kind)
            out = new
    return out, hits


def redact_recursive(obj: object) -> tuple[object, list[str]]:
    """Walk obj depth-first and redact all string leaves.
    Returns (cleaned_obj, all_hit_kinds)."""
    all_hits: list[str] = []
    if isinstance(obj, str):
        cleaned, hits = redact_string(obj)
        return cleaned, hits
    if isinstance(obj, dict):
        out = {}
        for k, v in obj.items():
            cleaned_v, hits = redact_recursive(v)
            out[k] = cleaned_v
            all_hits.extend(hits)
        return out, all_hits
    if isinstance(obj, list):
        out_list = []
        for item in obj:
            cleaned_item, hits = redact_recursive(item)
            out_list.append(cleaned_item)
            all_hits.extend(hits)
        return out_list, all_hits
    # int, float, bool, None — pass through
    return obj, []


def ai_root() -> Path:
    """Return the canonical ~/.ai/ root, honoring $AI_ROOT."""
    env = os.environ.get("AI_ROOT", "")
    if env:
        return Path(env)
    return Path.home() / ".ai"


def write_violation(kinds: list[str], payload_summary: str) -> None:
    """Write a violation record to $AI_ROOT/audit/violations/<UTC>-secret-detected.md."""
    now = datetime.now(tz=timezone.utc)
    ts = now.strftime("%Y-%m-%dT%H%M%SZ")
    violations_dir = ai_root() / "audit" / "violations"
    try:
        violations_dir.mkdir(parents=True, exist_ok=True)
        path = violations_dir / f"{ts}-secret-detected.md"
        unique_kinds = sorted(set(kinds))
        body = f"""# Violation — {ts}

- **File / Rule violated:** Common.md/§4 — No Secrets In Artifacts
- **What happened:** op-redact.py PreToolUse hook detected {len(kinds)} secret-like match(es) in a Claude Code tool-use payload and redacted them before the payload was processed. Pattern kinds: {', '.join(unique_kinds)}.
- **How noticed:** tool-flagged (op-redact.py)
- **Remediation:** Values were replaced with [REDACTED:<kind>] in the cleaned payload written to stdout. The original payload was not passed downstream.
- **Payload summary (redacted):** {payload_summary[:200]}
"""
        path.write_text(body, encoding="utf-8")
    except OSError as exc:
        # Logging failures must not cause the hook to block.
        print(f"[ai/op-redact] WARNING: could not write violation file: {exc}", file=sys.stderr)


def hook_name() -> str:
    return "op-redact"


def log(*parts) -> None:
    print(f"[ai/{hook_name()}]", *parts, file=sys.stderr, flush=True)


def self_check_ok() -> int:
    """Compile all patterns; exit 0 if OK."""
    try:
        for raw, _, _ in _RAW_PATTERNS:
            re.compile(raw)
    except re.error as exc:
        log(f"self-check FAIL: regex compile error: {exc}")
        return 1
    log("self-check OK")
    return 0


def main(argv: list[str]) -> int:
    if "--self-check" in argv:
        return self_check_ok()

    raw = sys.stdin.read()
    if not raw.strip():
        # Empty payload — nothing to redact; pass through empty.
        sys.stdout.write("")
        return 0

    # Parse payload. Fall back to wrapping raw in a dict so we can still
    # scan it for patterns.
    try:
        payload = json.loads(raw)
        parse_ok = True
    except json.JSONDecodeError:
        payload = {"_raw": raw}
        parse_ok = False

    cleaned, hits = redact_recursive(payload)

    if hits:
        log(f"{len(hits)} secret-like match(es) redacted: {sorted(set(hits))}")
        log("Per Common.md §1.P4 (no secrets in artifacts; non-overridable).")
        # Summarize the cleaned output (already redacted) for the violation record.
        summary = json.dumps(cleaned)[:200]
        write_violation(hits, summary)

    # Always output the cleaned payload as JSON so the tool call proceeds.
    if parse_ok:
        sys.stdout.write(json.dumps(cleaned))
    else:
        # We were given unparseable JSON; output what we managed to clean.
        sys.stdout.write(cleaned.get("_raw", raw) if isinstance(cleaned, dict) else raw)
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
