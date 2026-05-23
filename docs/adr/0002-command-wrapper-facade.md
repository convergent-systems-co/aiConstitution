# 0002. Command Wrapper Facade — cross-tool enforcement at the command layer

- Status: accepted
- Date: 2026-05-23
- Source: `SPEC.md §10.5`

## Context

Claude Code has clean native hooks (`PreToolUse`, `PostToolUse`,
`SessionStart`, `SessionEnd`, `Stop`, `SubagentStop`, `PreCompact`,
`UserPromptSubmit`). Copilot CLI's hook surface is less mature.
Cursor's surface is different again. The result: "enforce a rule
across all AI tools" is hard.

A second pressure: `git commit --no-verify` can bypass the pre-commit
secret scanner introduced for `SPEC.md §10`.

## Decision

Move enforcement to the **command layer**, not the AI-tool layer.

`~/.ai/bin/<command>` wrappers (`git`, `gh`, opt-in `terraform`,
`kubectl`) sit early on `PATH`. When any AI tool — or any human shell
session — runs `git commit`, the binary it actually exec's is
`~/.ai/bin/git`. That wrapper is identical regardless of caller. It
fires:

- **`preHooks`** before the real command runs (any non-zero exit
  aborts).
- **`commandHooks`** on the specific subcommand match.
- **`postHooks`** after, with the captured result (audit logging).

Configuration: `hooks/command-wrappers.toml` (per
`SPEC.md §10.5.2`).

The wrapper strips `--no-verify` from `git commit` per
`settings.secret_scanning.allowNoVerifyBypass = false`, fires the
branch-guard hook on protected-branch mutations, fires the worktree
guard on `git worktree`, fires `secret-precommit.py` on `git commit`,
and records every invocation to the audit log.

## Consequences

- **Cross-tool enforcement** — Copilot now gets the same enforcement
  Claude does, not because Copilot grew new hook surfaces, but
  because the surface moved to the command layer where both tools
  converge.
- **Defense-in-depth, not security control.** The spec is explicit
  about this. Bypass via absolute path (`/usr/bin/git`) or
  `PATH` override is always possible. The wrapper's value is catching
  the common case and producing an **audit signal when it's bypassed**
  — `postHooks` runs `audit-command.py`, so a missing audit line is
  itself visible.
- One PATH manipulation. `hash -r` / `rehash` is required after
  install to invalidate shell cache.
- The wrapper preserves exit codes, stdin/stdout/stderr, and signal
  handling — must not alter program semantics for commands that pass
  all hooks.

## Alternatives considered

- **Native-only enforcement per AI tool.** Rejected — Copilot's gap
  reopens every time the user switches tools.
- **Git-only wrapper for `--no-verify`.** Rejected as too narrow —
  the same pattern generalizes to `gh repo delete`,
  `terraform destroy`, `kubectl delete`, etc.
- **Mandatory; no bypass possible.** Rejected — the bypass paths are
  fundamental properties of PATH lookup; pretending otherwise is
  dishonest.
