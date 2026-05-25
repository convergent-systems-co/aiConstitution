# TL-2 Plan: Wave 2 — `domain:cli` batch (#171, #172, #173, #174, #175, #176)

**Tech Lead:** TL-2 (Wave 2 — `domain:cli`)
**Repo:** `convergent-systems-co/aiConstitution`
**Base branch:** `main` (`d3303df`)
**Module roots:** `src/cmd/ai`, `src/internal` (wired via `go.work`)

---

## Strategy — two PRs

### Wave A (independent — start immediately) → `task/tl2-wave-a`
- **#173** `ai version` — print `ai <buildinfo.Version()>`, `Code.md <ver>`, `questions.yaml <ver>`.
- **#174** `ai mode list/current/clear/<name>` — write `LoadMode/SaveMode` in `src/internal/state/state.go`, wire mode subcommands.
- **#175** `ai sync push/pull` — `exec.Command("git", ...)` in `paths.AIRoot()`. Remote from `AI_SYNC_REMOTE` env, default `origin`.
- **#176** `ai backup` — dated tar.gz under `paths.ConfigDir()/backups/`, skip `.git/` and `audit/interactions/`.

### Wave B (after TL-1's audit/memory PR merges) → `task/tl2-wave-b`
- **#171** `ai memory list/show/archive` — read `MemoryDir()/MEMORY.md`, cat memory file, move to `archived/`.
- **#172** `ai audit list/show/rotate` — list violations + overrides, show file, drop interactions/<YYYY-MM>.jsonl older than 30 days.

---

## Acceptance criteria per task

### #173 — `ai version`
- Output lines:
  ```
  ai <buildinfo.Version()>
  Code.md <ver>          # from **Version:** N.N in paths.AIRoot()/Code.md, or "not found"
  questions.yaml <ver>   # wizard.ParseTaxonomy(embed.QuestionsYAML()).Version
  ```
- Extract via regexp `**Version:** ([0-9.]+)`.
- Tests: temp `AI_ROOT` with mock `Code.md` containing `**Version:** 0.7`; assert output contains `Code.md 0.7`.

### #174 — `ai mode`
- **First**: complete `LoadMode()` / `SaveMode()` in `src/internal/state/state.go`.
  - `LoadMode`: `os.ReadFile(paths.ModeJSON())`; if `os.IsNotExist`, return `Mode{}, nil`; else `json.Unmarshal`.
  - `SaveMode`: `json.Marshal`, `os.MkdirAll(filepath.Dir(...), 0o750)`, atomic write via temp+rename (`0o600`).
- `ai mode list`: `os.ReadDir(paths.GovernanceDir()+"/personas/agentic/")`; print basenames (`.md` trimmed).
- `ai mode current`: print `state.LoadMode().Name`; if empty, print `(none)`.
- `ai mode clear`: `state.SaveMode(state.Mode{})`.
- `ai mode <name>`: confirm `<name>.md` exists under `personas/agentic/`; build `Mode{Name, Type:"persona", Source:"shipped", ActivatedAt:time.Now().UTC(), ActivatedVia:"cli"}`, `SaveMode`, print `Mode set: <name>`.
- Tests: temp `AI_ROOT` + `AICONST_CONFIG_DIR` with mock persona; verify list/set/current/clear round-trip.

### #175 — `ai sync push/pull`
- Helper: `syncRemote()` reads `AI_SYNC_REMOTE` env, defaults to `"origin"`.
- `ai sync push` (in `paths.AIRoot()`):
  1. `git add -A`
  2. `git diff --cached --quiet` — if exit 0, nothing staged, skip commit step.
  3. `git commit -m "chore: sync push <UTC ISO-8601>"`.
  4. `git push <remote> HEAD:main`.
- `ai sync pull`: `git pull <remote> main`.
- All commands use `exec.Command("git", ...)`. Wire `Stdout`/`Stderr`. Return non-zero exit as wrapped error.
- Indirection for tests: package-level `var runGit = func(dir string, args ...string) error { ... }`.
- Tests: stub `runGit`, drive push (clean repo / dirty repo paths) + pull, assert correct arg sequences.

### #176 — `ai backup`
- `dest := paths.ConfigDir() + "/backups/backup-" + UTC("20060102-150405") + ".tar.gz"`.
- `os.MkdirAll(filepath.Dir(dest), 0o750)`.
- `archive/tar` over `compress/gzip` (`gzip.BestCompression`). Output file mode `0o600`.
- `filepath.WalkDir(paths.AIRoot())`:
  - Skip top-level `.git/` and `audit/interactions/` (`fs.SkipDir` on directory match).
  - Each file's tar header path is `filepath.Rel(AIRoot(), path)`.
- Print archive path.
- Tests: temp `AI_ROOT` populated with `Code.md`, `audit/violations/x.md`, `audit/interactions/2026-05.jsonl`, `.git/HEAD`, etc. Run backup; open archive; assert `.git/`, `audit/interactions/` excluded and other files included.

### #171 — `ai memory list/show/archive`
- `list`: read `paths.MemoryDir()/MEMORY.md`; print contents; or `(no memories)` if absent.
- `show <name>`: read `paths.MemoryDir()/<name>.md`; print to stdout.
- `archive <name>`:
  1. `os.MkdirAll(paths.MemoryDir()/archived, 0o750)`.
  2. `os.Rename(<name>.md, archived/<name>.md)`.
  3. Rewrite MEMORY.md without the line referencing `<name>`.
- Tests: temp `AI_ROOT` with mock MEMORY.md + memory file; verify each subcommand.

### #172 — `ai audit list/show/rotate`
- `list`: glob `paths.AuditDir()/violations/*.md` + `overrides/*.md`; sort newest-first by name (UTC timestamp prefix sorts lexically); print.
- `show <filename>`: cat that file from the audit dir (look in `violations/` then `overrides/`).
- `rotate`: walk `paths.AuditDir()/interactions/`; parse `<YYYY-MM>.jsonl` (and `.jsonl.gz`); if older than 30 days from today (compare by month boundary), `os.Remove`.
- Tests: temp `AI_ROOT` with mock violation files + old/new interaction logs; verify list + rotate.

---

## Tests

Each task gets a test file under the package owning the change:

| Task | Test path |
|---|---|
| #173 | `src/cmd/ai/cmd/version_test.go` |
| #174 | `src/cmd/ai/cmd/mode_test.go` + `src/internal/state/state_test.go` |
| #175 | `src/cmd/ai/cmd/sync_test.go` |
| #176 | `src/cmd/ai/cmd/backup_test.go` |
| #171 | `src/cmd/ai/cmd/memory_test.go` |
| #172 | `src/cmd/ai/cmd/audit_test.go` |

Run command (from each module root):
```bash
cd src/cmd/ai && go test ./...
cd src/internal && go test ./...
```

## Risks
- `ai version` needs the `wizard` package — `src/cmd/ai` doesn't currently import it. Add the `require` (transitively present via `go.work`); confirm `go mod tidy` once after.
- `ai sync` test seam: keep `runGit` exported-for-test (lowercase package var; tests use a setter or test build tag).
- `ai backup` walk must NOT include `.config/` (it's outside `AIRoot()`, but tests should confirm).

## Out of scope
- `ai sync status` (issue scoped to push/pull only).
- `ai memory codify` (task #170 — TL-1 owns).
- Mode resolution from drafts/atoms (issue scoped to `personas/agentic/` only).
