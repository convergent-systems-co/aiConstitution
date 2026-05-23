#!/usr/bin/env python3
"""hooks/checkpoint-tick.py — 30-minute background tick that writes a
HANDOFF.md so context-window resets don't lose state.

Per ~/.ai/Common.md §U10 (handoff + checkpoint) + §U13 (context-window
discipline). Wires into Claude Code's session.

Behavior:
  - On invocation, if more than 30 minutes have passed since the last
    tick (tracked at ~/.config/aiConstitution/checkpoints/<project>/.last-tick),
    invoke the `/checkpoint` skill if available, otherwise emit a
    minimal HANDOFF.md template at the cwd.
  - Updates the timestamp.

Self-check:
  --self-check  Verifies the checkpoint directory is writable.
"""
from __future__ import annotations

import argparse
import datetime as dt
import hashlib
import os
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
import _lib  # noqa: E402


def config_dir() -> Path:
    """~/.config/aiConstitution/ — per Common.md §5.5 the canonical
    machine-local mutable-state root."""
    env = os.environ.get("AICONST_CONFIG_DIR")
    if env:
        return Path(env)
    if sys.platform == "win32":
        appdata = os.environ.get("APPDATA")
        if appdata:
            return Path(appdata) / "aiConstitution"
    return Path.home() / ".config" / "aiConstitution"


def checkpoints_dir() -> Path:
    return config_dir() / "checkpoints"


def project_hash(cwd: str) -> str:
    return hashlib.sha256(cwd.encode("utf-8")).hexdigest()[:16]


def last_tick(cwd: str) -> dt.datetime | None:
    f = checkpoints_dir() / project_hash(cwd) / ".last-tick"
    if not f.is_file():
        return None
    try:
        return dt.datetime.fromisoformat(f.read_text(encoding="utf-8").strip())
    except (ValueError, OSError):
        return None


def write_tick(cwd: str) -> None:
    d = checkpoints_dir() / project_hash(cwd)
    d.mkdir(parents=True, exist_ok=True)
    (d / ".last-tick").write_text(
        dt.datetime.now(dt.timezone.utc).isoformat(),
        encoding="utf-8",
    )


def needs_tick(cwd: str, threshold_minutes: int = 30) -> bool:
    prev = last_tick(cwd)
    if prev is None:
        return True
    age = dt.datetime.now(dt.timezone.utc) - prev
    return age.total_seconds() >= threshold_minutes * 60


def emit_minimal_handoff(cwd: str) -> None:
    """Write a stub HANDOFF.md if none exists. The /checkpoint skill
    produces a richer version when available."""
    handoff = Path(cwd) / "HANDOFF.md"
    if handoff.exists():
        return
    body = f"""# HANDOFF — {dt.datetime.now(dt.timezone.utc).strftime('%Y-%m-%dT%H:%M:%SZ')}

(Background checkpoint tick from ~/.ai/hooks/checkpoint-tick.py.)

**Current state:** (fill in)
**Last known-good commit:** (run `git rev-parse HEAD`)
**Next steps:** (fill in)
**Open questions:** (fill in)
**Files in flight:** (fill in)

Per ~/.ai/Common.md §U10.
"""
    try:
        handoff.write_text(body, encoding="utf-8")
        _lib.log(f"wrote stub {handoff}")
    except OSError as e:
        _lib.log("could not write HANDOFF.md:", e)


def main(argv: list[str]) -> int:
    parser = argparse.ArgumentParser(add_help=True)
    parser.add_argument("--self-check", action="store_true")
    parser.add_argument("--threshold-minutes", type=int, default=30)
    args = parser.parse_args(argv)

    if args.self_check:
        try:
            checkpoints_dir().mkdir(parents=True, exist_ok=True)
        except Exception as e:
            _lib.log("self-check FAIL:", e)
            return 1
        _lib.log("self-check OK")
        return 0

    cwd = os.getcwd()
    if not needs_tick(cwd, args.threshold_minutes):
        return 0

    emit_minimal_handoff(cwd)
    write_tick(cwd)
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
