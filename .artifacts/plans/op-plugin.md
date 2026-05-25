# Plan: op Plugin — Issues #239–#242

**Objective:** Implement `ai op` subcommands (env, signin, signout, whoami, clip) and an `op-redact.py` PreToolUse hook that redacts 1Password/secret refs from Claude Code tool-use events.

---

## Rationale + Alternatives Table

| Alternative | Pros | Cons | Verdict |
|---|---|---|---|
| Shell wrappers (bash scripts) | Fast to write | Not testable with existing Go test infra; breaks cross-platform; inconsistent with CLI patterns | Rejected |
| **Go cobra subcommands + Python hook (chosen)** | Consistent with every other `ai` verb; Go tests for subprocess mocking; Python hook follows established hook pattern (_lib.py) | More code | Chosen |
| Single `ai op` dispatch to `op` binary verbatim | Minimal code | No formatting, no clipboard safety, no PATH error handling | Rejected |

At least two alternatives considered; "do nothing" is not applicable — these are tracked issues.

---

## Scope

### Files to create
- `src/cmd/ai/cmd/op.go` — cobra command tree: `ai op {env,signin,signout,whoami,clip}`
- `src/cmd/ai/cmd/op_test.go` — Go tests
- `src/cmd/ai/embed/hooks/op-redact.py` — PreToolUse hook
- `src/cmd/ai/embed/hooks/test_op_redact.py` — pytest tests

### Files to modify
- `src/cmd/ai/cmd/root.go` — register `newOpCmd()` in `NewRootCmd()`

### Files NOT touched
- skills.go, plugins.go, restore.go, init.go (per TL2 directive)

---

## Approach

### Step 1: Failing tests (TDD Writer)
Write `op_test.go` and `test_op_redact.py` that are fully RED before any implementation.

### Step 2: Coder A — op.go + op_test.go (#239, #240, #241)

**`ai op env`**
- Run `op item list --format json` (via `exec.Command`)
- Parse JSON array; each item: `TITLE=op://VAULT_NAME/ITEM_UUID/field`
- `--vault <name>` filters by vault name
- `--format dotenv|export` flag (default: dotenv). Export prefix: `export KEY=value`
- When `op` not on PATH: `fmt.Errorf("op CLI not found — install 1Password CLI")`
- Tests mock `op` by placing a fake script on a temp PATH

**`ai op signin`**
- No `--address` flag: print `eval $(op signin)` instruction (cannot exec in parent shell)
- With `--address <addr>`: run `op account add --address <addr>`
- When `op` not on PATH: error clearly

**`ai op signout`**
- Run `op signout --forget`
- When `op` not on PATH: error clearly

**`ai op whoami`**
- Run `op whoami --format json`
- Print output to stdout
- When `op` not on PATH: error clearly

**`ai op clip <ref>`**
- Accept `op://` ref or item name
- Run `op read <ref>` to get the value (never print to stdout)
- Detect clipboard command: macOS=pbcopy, Linux X11=xclip, Wayland=wl-copy
- Pipe output to clipboard command
- Print: `Secret copied to clipboard.`
- Error when `op` not found or clipboard command not found

### Step 3: Coder B — op-redact.py + test_op_redact.py (#242)

**`op-redact.py`**
- Read Claude Code tool-use JSON from stdin (PreToolUse event)
- Walk all string fields recursively using a depth-first traversal
- Pattern matching (inline, not loading patterns.json since these are op-specific):
  - `gho_`, `ghp_`, `github_pat_` → `[REDACTED:github-token]`
  - `Bearer ` + 20+ chars → `[REDACTED:bearer-token]`
  - `op://` references → `[REDACTED:op-ref]`
  - `sk-` + 40+ chars (OpenAI-style) → `[REDACTED:openai-key]`
  - PEM blocks `-----BEGIN` → `[REDACTED:pem-block]`
- Write violation record to `$AI_ROOT/audit/violations/<UTC>-secret-detected.md`
- Output cleaned JSON to stdout (exit 0 — never blocks)
- Supports `--self-check`

---

## Testing Strategy

### Go tests (op_test.go)
- Create a temp dir with a fake `op` shell script; prepend it to PATH
- Test `op env` output format (dotenv and export)
- Test `op env --vault` filtering
- Test error when `op` not on PATH
- Test `op clip` pipes to a mock pbcopy (fake script that writes to a temp file)
- Test `op signin` without flag prints eval instruction
- Test `op whoami` streams op output

### Python tests (test_op_redact.py)
- Test: gho_ token in a string field is replaced with `[REDACTED:github-token]`
- Test: op:// ref is replaced with `[REDACTED:op-ref]`
- Test: violation file written to temp AI_ROOT/audit/violations/
- Test: clean payload passes through unchanged
- Test: deeply nested JSON fields are redacted
- Test: exit code is always 0 (never blocks)
- All tests use real temp dirs and subprocess mock via monkeypatch

---

## Risk Assessment

| Risk | Mitigation |
|---|---|
| `op` JSON format changes | Parse defensively; only required fields used |
| Clipboard detection on non-macOS | Use `shutil.which` / `exec.LookPath`; error clearly |
| Regex false positives in redact hook | Patterns anchored where possible; exit 0 means no blocking |
| Pattern overlap with existing secret-block.py | op-redact.py complements, does not replace — different scope (redact+log vs block) |

---

## Dependencies
- `op` CLI binary at runtime (not at build time)
- `pbcopy`/`xclip`/`wl-copy` at runtime
- `github.com/spf13/cobra` (already in go.mod)
- Python stdlib only (no third-party) for op-redact.py

## Backward Compatibility
- `ai op` is a new subcommand; no existing behavior broken
- op-redact.py must be registered as a PreToolUse hook in `~/.claude/settings.json` by the user (out-of-scope for this PR; documented in hook file header)

## Out of Scope
- `op inject` (template injection) — not in #239–#242
- Auto-registration of op-redact.py as a hook
- Windows clipboard support (not macOS/Linux in spec)
