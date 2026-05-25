# Plan: cli-portability — ai restore (local+url) + ai init

**Objective:** Implement `ai restore` (local tar.gz + `--from-url`) and `ai init`
(project.yaml scaffold + integration-file generation) so engineers can restore a
canonical tree from a snapshot and scaffold a new project with AI governance files
in one command.

**Issues:** #211, #212, #213, #214

---

## Alternatives Table

| Alternative | Pros | Cons | Verdict |
|---|---|---|---|
| Reuse/extend current restore.go stub in-place | Single file, no rename | Current stub is CLI-flag oriented to a sync URL; new spec is snapshot-extract oriented — conflicting semantics | Chosen (rewrite body, keep file) |
| New file restore2.go, rename old | Clean separation | Breaks AddCommand (two newRestoreCmd) | Rejected |
| Shell out to `tar` for extraction | Simple | Not portable (Windows); no control over errors | Rejected — use `archive/tar` |
| `ai init` as subcommand of `ai setup` | Fewer top-level verbs | Violates task spec; setup is TUI-oriented | Rejected |
| Write init.go as a new file | Clean, spec-required separation | Adds file | Chosen (spec requires it, don't touch clone.go) |
| Do nothing (document alternative) | Zero effort | Leaves #211-#214 open | Rejected |

---

## Scope

**Files to create:**
- `src/cmd/ai/cmd/restore_test.go`
- `src/cmd/ai/cmd/init.go`
- `src/cmd/ai/cmd/init_test.go`

**Files to modify:**
- `src/cmd/ai/cmd/restore.go` — replace stub body with real implementation
- `src/cmd/ai/cmd/root.go` — add `newInitCmd()` to AddCommand block

---

## Approach

### Step A — TDD Writer produces failing tests (RED)

Tests cover:
1. `restore` extracts tar.gz to a temp AIRoot
2. `restore` backs up existing AIRoot before extract
3. `restore --from-url` with a local file:// URL downloads + extracts
4. `init` writes project.yaml with correct stack detection (Go, Node, Python, unknown)
5. `init` writes `.claude/CLAUDE.md` @-include
6. `init --dry-run` prints but does not write

### Step B (Coder A) — restore.go (#211, #212)

`restore.go` rewrite:
- Remove `--dest` and `--no-hooks` flags (were stub-only).
- Add `--from-url` flag.
- `cobra.ExactArgs(1)` kept for local path; `--from-url` replaces positional when set.
- Logic:
  1. Determine source: `--from-url` → download/clone logic (#212), else positional arg (#211).
  2. Validate: source must exist and end in `.tar.gz` (or directory — directory case is a future extension; for now only `.tar.gz` accepted per spec).
  3. Backup: if AIRoot exists, copy to `filepath.Dir(AIRoot) + "/.ai-backup-" + UTC`.
  4. Extract using `archive/tar` + `compress/gzip`.
  5. Print "Restored from <path>. Previous backed up to <backup-path>." (or "No existing ~/.ai/ to back up.")
- `--from-url` download: if URL ends in `.tar.gz`, HTTP GET to temp file, then extract. If URL looks like a git URL (contains `.git` or `github.com`/`gitlab.com`), `git clone` to temp dir, find `*.tar.gz` in root, extract.

### Step C (Coder B) — init.go (#213, #214)

New `init.go`:
- `cobra.NoArgs()`.
- Stack detection: check cwd for `go.mod` → Go, `package.json` → Node, `pyproject.toml` or `requirements.txt` → Python, else "unknown".
- Write `project.yaml` in cwd: name = `filepath.Base(cwd)`, stack, tooling.test_command.
- Idempotent: if `project.yaml` exists, print and exit 0.
- Write `.claude/CLAUDE.md`: `@~/.ai/Constitution.md\n` (idempotent — check for line presence).
- Create `.cursor/rules/` dir; write `constitution.md` as symlink → `~/.ai/Constitution.runtime.md`.
- Write `.github/copilot-instructions.md`: header + @-include line (idempotent).
- `--dry-run` flag: print actions without executing.

### Step D — root.go update

Add `newInitCmd()` to the `root.AddCommand(...)` block (after `newCloneCmd()`).

---

## Testing Strategy

- All tests use `os.MkdirTemp` for isolation; `t.TempDir()` for auto-cleanup.
- AIRoot override via `AI_ROOT` env var (paths package honors it).
- No network calls — `--from-url` tests use a local path beginning with `file://` or just a constructed temp `.tar.gz`.
- Tests run with `go test ./cmd/...` from `src/cmd/ai/`.

---

## Risk Assessment

| Risk | Mitigation |
|---|---|
| `archive/tar` extraction path traversal | Validate each entry's Name with `filepath.Clean` + check for `..` |
| Symlink creation fails on platforms without symlink support | Wrap in error with clear message; not a failure for the test suite (macOS+Linux target) |
| HTTP download in `--from-url` requires network in CI | Mock using local file path; real HTTP tested manually |
| `init` cwd writes during test pollute repo | Use `t.TempDir()` as cwd for every init test |

---

## Backward Compatibility

- Existing `restore.go` stub flags (`--dest`, `--no-hooks`) are removed. They were documented as stub-only and never implemented. Any caller relying on them at this stage has no behavior to break.
- `root.go` AddCommand block gains one entry; no existing entries are modified.

---

## Dependencies

- None external to the standard library + cobra (already a dep).
- `archive/tar`, `compress/gzip`, `net/http`, `os/exec` (for git clone in url mode) — all stdlib.
