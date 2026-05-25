# Plan — Notification System (#243–#246)

**Objective:** Deliver cross-platform notification scripts (`notify-me`, `notify-me.ps1`, `notify-me.cmd`) and a `ai doctor` terminal-notifier check that together give the `ai` tool first-class OS notification support on macOS and Windows, with an optional ntfy push channel for urgent events.

---

## Rationale

| Alternative | Pros | Cons | Verdict |
|---|---|---|---|
| Shell scripts embedded in binary (chosen) | Portable, no extra build step, follows existing wrapper pattern (gh, git) | Requires embed; shell tests are coarser | Chosen — consistent with existing convention |
| Go binary subcommand `ai notify` | Type-safe, cross-platform | Over-engineering for a thin OS-dispatch script; no benefit over shell for macOS | Rejected |
| Do nothing | Zero risk | Users get no OS feedback; urgent events go unnoticed | Rejected |

---

## Scope

### Files to create
- `src/cmd/ai/embed/wrappers/notify-me` — macOS shell script (#243 + #246)
- `src/cmd/ai/embed/wrappers/notify-me.ps1` — PowerShell script (#245)
- `src/cmd/ai/embed/wrappers/notify-me.cmd` — CMD batch shim (#245)
- `src/cmd/ai/cmd/doctor_test.go` — new test file for doctor check (#244)

### Files to modify
- `src/cmd/ai/cmd/doctor.go` — add terminal-notifier check to `runDoctor()` (#244)

### Files not touched
- `skills.go`, `plugins.go`, `op.go`, `restore.go`, `integrate.go` — out of scope

---

## Approach

### #243 — notify-me macOS shell script
1. Create `src/cmd/ai/embed/wrappers/notify-me` as executable shell script.
2. Parse args: `--title <str>`, `--message <str>`, `[--level info|warn|urgent]`.
3. Priority order:
   - `terminal-notifier` on PATH → use it with `-sound default`.
   - Fallback → `osascript -e 'display notification ...'`.
4. On success: write nothing to stdout (silent).
5. On failure: exit non-zero, message to stderr.

### #244 — doctor terminal-notifier check
1. Add `runDoctor()` to `doctor.go` replacing the stub.
2. New check: `runtime.GOOS == "darwin"` guard wraps a PATH probe for `terminal-notifier`.
3. Print `[✓] terminal-notifier: found at <path>` if found.
4. Print `[⚠] terminal-notifier: not found — run: brew install terminal-notifier` if missing.
5. Add `doctor_test.go` with a test that manipulates `PATH` to control visibility.

### #245 — Windows shims
1. Create `notify-me.ps1` with BurntToast probe (`Get-Command New-BurntToastNotification`), Windows.Forms fallback.
2. Create `notify-me.cmd` that delegates to the `.ps1` via `powershell -File "%~dp0notify-me.ps1" %*`.

### #246 — ntfy push fallback
1. Extend `notify-me` after local notification dispatch.
2. When `--level urgent`, additionally HTTP POST to ntfy.
3. Topic source: `AI_NTFY_TOPIC` env var first; if unset, read `ntfy_topic` from `~/.config/aiConstitution/settings.toml` (plain TOML key-value parse via `grep`/`sed` — no toml parser dependency in shell).
4. Use `curl` if available, else `wget --post-data`.
5. If no topic configured → silent no-op (not an error).

---

## Testing strategy

1. **Shell tests** (`tests/notify-me/`) — bash scripts that invoke `notify-me` with a mock PATH, capture stdout/stderr, assert exit codes and argument forwarding.
2. **Go tests** (`doctor_test.go`) — table-driven tests using `t.TempDir()` and `t.Setenv("PATH", ...)` to control PATH; assert stdout/stderr output contains the expected marker strings (`[✓]` / `[⚠]`).

---

## Risk assessment

| Risk | Mitigation |
|---|---|
| `osascript` not available (headless CI) | Tests mock the binary via PATH override — real dispatch not called in CI |
| `terminal-notifier` PATH probe fails on CI | Same PATH override pattern; doctor test uses a temp dir |
| ntfy POST fails silently | That is the intended behavior; tests validate silent path with `AI_NTFY_TOPIC` unset |
| Windows PATH delimiter differences in `.cmd` | `%~dp0` is CMD-native and reliable; `powershell -File` is well-tested pattern |

---

## Dependencies

- #243 must land before #246 (ntfy extends the same file).
- #244 is independent of #243/#245/#246.
- #245 is independent of #243/#244/#246.

---

## Backward compatibility

All additions. No existing interfaces modified. `doctor.go` replaces a `stub()` call with real logic; the overall cobra command surface is unchanged.
