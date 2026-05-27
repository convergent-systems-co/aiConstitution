# Plan: feat/346 — Implement `ai update`

**Objective:** Replace the `ai update` stub with a real implementation that
(1) runs `git pull --ff-only` in `~/.ai/` when it is a git repo, and
(2) queries the GitHub Releases API to compare the current binary version
against the latest release tag and prints an actionable upgrade notice.

**Status:** approved

---

## Rationale

| Alternative | Pros | Cons | Verdict |
|---|---|---|---|
| Shell out to `git pull` via package-level var (mirrors sync.go) | DI-seam already proven by sync tests; tests never spawn real git | None | **Chosen** |
| Embed a full git library (go-git) | No external dependency at runtime | Heavy; overkill for a single pull | Rejected |
| HTTP fetch via package-level var (mirrors pattern in atoms.go) | Trivially mockable in tests | None | **Chosen** |
| Hardcode `http.Get` | Simpler | Cannot unit-test without a real network | Rejected |

---

## Scope

**Files modified:**
- `src/cmd/ai/cmd/update.go` — replace stub with `runUpdate()`; add
  `runGitUpdate` and `githubLatestRelease` package-level vars for DI.

**Files created:**
- `src/cmd/ai/cmd/update_test.go` — four tests (RED first, then GREEN).

**Files NOT touched:**
- `runMigrate` / `executeMigrationSteps` / `isV2Layout` — `--migrate` path
  is fully implemented; zero changes.
- Any file outside `src/cmd/ai/cmd/`.

---

## Approach

### Step 1 — Add DI seams to update.go

```go
// runGitUpdate shells out to git for the update pull. Tests substitute a
// fake so assertions verify the recorded call without spawning real git.
var runGitUpdate = func(dir string, args ...string) error { ... real impl ... }

// githubLatestRelease fetches the latest release tag from the GitHub API.
// Returns ("", nil) on non-fatal failures (network down, rate-limit, etc.)
// so the caller can print a warning and continue.
var githubLatestRelease = func() (string, error) { ... real impl ... }
```

### Step 2 — Implement `runUpdate`

```
1. Resolve aiRoot := aiRoot()
2. Check if filepath.Join(aiRoot, ".git") exists
   - YES: fmt.Fprintf(out, "Pulling ~/.ai/ ...\n")
           runGitUpdate(aiRoot, "pull", "--ff-only")
           on error → return fmt.Errorf("update: git pull failed: %w", err)
   - NO:  fmt.Fprintln(out, "~/.ai/ is not a git repo — skipping pull")
3. Fetch latest release tag via githubLatestRelease()
   - on error → fmt.Fprintf(out, "warning: could not check latest version: %v\n", err)
                continue (non-fatal)
4. Compare buildinfo.Raw() against latest tag (string equality after
   normalizing leading "v")
   - equal → fmt.Fprintf(out, "ai %s is up to date.\n", buildinfo.Raw())
   - newer available → fmt.Fprintf(out, "New version %s available.\n...", latest)
                       print brew/go install instructions
```

### Step 3 — Wire into RunE

Replace:
```go
notice("update:", ...)
return stub("update", ...)
```
With:
```go
return runUpdate(cmd)
```

---

## Testing strategy

Four tests in `update_test.go` (package `cmd`):

| Test | What it asserts |
|---|---|
| `TestUpdate_NoGitRepo` | No `.git` dir → output contains "not a git repo", no git call made |
| `TestUpdate_GitPullFails` | runGitUpdate returns error → command returns non-nil error |
| `TestUpdate_VersionUpToDate` | latest == current → output contains "up to date" |
| `TestUpdate_NewVersionAvailable` | latest > current → output contains "New version" + upgrade instructions |

DI: replace `runGitUpdate` and `githubLatestRelease` package-level vars in
`t.Cleanup` blocks (same pattern as `sync_test.go`).

---

## Risk assessment

| Risk | Mitigation |
|---|---|
| GitHub API rate-limit (60 req/hr unauthenticated) | Non-fatal: print warning + continue |
| Network unavailable | Same non-fatal path |
| Version string mismatch (v-prefix vs bare) | Normalize both sides with `strings.TrimPrefix(s, "v")` |
| `--migrate` path broken by refactor | Zero changes to `runMigrate`, `isV2Layout`, `executeMigrationSteps` |

---

## Backward compatibility

`--migrate`, `--skip-migrate`, `--blocking`, `--non-interactive` flags: unchanged.
`ai update` (no flags): previously returned an error stub; now returns nil on success.
No API changes; no callers outside the cobra tree.

---

## Dependencies

None. All imports already present in the repo (`os`, `os/exec`, `net/http`,
`encoding/json`, `fmt`, `path/filepath`, `strings`).
`buildinfo` package already imported in `version.go` (same package `cmd`).
