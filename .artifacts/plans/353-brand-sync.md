# Plan — feat/353: ai brand fetch/list + ai sync status

**Issue:** #353
**Branch:** feat/353-brand-sync
**Author:** Claude Sonnet 4.6 (1M context)
**Date:** 2026-05-27

## Objective

Replace the three stub implementations in `brand.go` (fetch, list) and
`sync.go` (status) with real behavior that queries GitHub API, downloads
brand files to the cache, and reports `~/.ai/` git sync state.

## Alternatives

| Alternative | Pros | Cons | Verdict |
|---|---|---|---|
| Implement via `ai atoms fetch --kind=brand` (as brand.go Long doc suggests) | Reuses existing fetch pipeline | atoms fetch requires full tar.gz archive; GitHub contents API returns individual files — not tar.gz shaped; the sugar layer would still need the GitHub directory walk | Rejected for now; wire up properly once atom kind registry matures |
| Parse brand.toml via BurntSushi/toml (already in go.mod) | Consistent with config package | brand.toml schema is simple and only a few keys are needed | Chosen — full BurntSushi parse is overkill but is already in the module |
| Shell out to git for sync status | Simple strings | Creates tight coupling, hard to test | Rejected — use exec.Command with a test seam, same pattern as runGit |

## Scope

Files to **modify:**
- `src/cmd/ai/cmd/brand.go` — replace fetch and list stubs
- `src/cmd/ai/cmd/sync.go` — replace status stub

Files to **create:**
- `src/cmd/ai/cmd/brand_test.go` — TDD tests (RED first, then GREEN)

## Approach

1. Write `brand_test.go` with four tests covering brand list success,
   list API failure, brand fetch (files written to cache), and the two
   sync status cases (not-a-git-repo + up-to-date).
2. Implement `brand list` in brand.go:
   - httpFetchBrandList(url) → []brandEntry (using package-level httpClient seam)
   - tabwriter output: SLUG | VERSION
3. Implement `brand fetch <slug>`:
   - httpFetchBrandContents(url) → []githubContentEntry
   - download each file to cache dir via package-level httpGet seam
   - if brand.toml present: parse voice/tone/name keys, write to settings.toml
4. Implement `sync status` in sync.go:
   - runGitOutput seam (returns stdout string) alongside existing runGit
   - doSyncStatus(w io.Writer) that calls git commands and formats output
   - graceful "not a git repository" on non-zero exit

## Testing strategy

- All tests use `t.Setenv` for AI_ROOT / AICONST_CONFIG_DIR to temp dirs.
- GitHub API calls go to `httptest.Server` instances — no real network.
- `sync status` tests use a stub runGitOutput seam (same pattern as
  the existing `runGit` seam in sync_test.go).
- Tests are written RED (stubs in place) then GREEN after implementation.

## Risk assessment

| Risk | Mitigation |
|---|---|
| GitHub API changes Content-Type or shape | Tests mock the API; real failures surface at runtime with clear error messages |
| brand.toml key collision with existing settings.toml sections | Only well-known keys (voice, tone, brandName) are applied; unknown keys are skipped |
| sync status git invocations fail on machines without ~/.ai/ | Graceful "not a git repository" path; tests use temp dirs |

## Dependencies

- BurntSushi/toml — already in go.mod (used by config package)
- encoding/json — stdlib
- text/tabwriter — stdlib
- net/http, os/exec — stdlib

## Backward compatibility

- brand.go stubs are replaced; no callers of the stub error exist outside tests
- sync.go status stub is replaced; no callers depend on the stub error
