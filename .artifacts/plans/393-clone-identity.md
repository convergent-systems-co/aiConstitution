# Plan: #393 — ai clone identity routing

**Objective:** Wire `ai clone` to read `metadata/projects.json` and apply per-project git identity (name, email, signing key) after a successful clone, based on URL pattern matching or an explicit `--identity` flag.

**Rationale:**

| Alternative | Rejected because |
|---|---|
| Shell post-clone hook | Harder to test, harder to surface errors, not integrated with cobra command lifecycle |
| New top-level `ai identity apply` cmd | Doesn't solve auto-routing on clone; user still has to run it manually |
| projects.json via env var only | No URL-pattern matching; forces explicit flag every time |

**Scope:**
- CREATE `src/cmd/ai/internal/identity/identity.go`
- CREATE `src/cmd/ai/internal/identity/identity_test.go`
- MODIFY `src/cmd/ai/cmd/clone.go` — add `applyIdentityRouting`, `xdgConfigDir`, replace stub, add `io` import
- MODIFY `src/cmd/ai/cmd/clone_test.go` (or create) — add `TestClone_IdentityApplied`
- MODIFY `src/cmd/ai/cmd/export_test.go` — export `ApplyIdentityRouting` for test access

**Approach:**
1. Write failing tests for `identity` package
2. Implement `identity` package to pass tests
3. Write failing integration test in `clone_test.go`
4. Add `applyIdentityRouting` and `xdgConfigDir` to `clone.go`
5. Replace stub in `RunE`
6. Export helper via `export_test.go` for external test access
7. Verify build + lint + tests

**Testing strategy:**
- Unit: `TestNormalizeURL`, `TestMatch_*`, `TestFindByName`, `TestLoad_*` in `identity_test.go`
- Integration: `TestClone_IdentityApplied` calls `applyIdentityRouting` against a `git init` temp dir

**Risk assessment:**
- `filepath.Match` does not support `**` globs — patterns must use single `*` for path segments. Document this.
- `os.UserConfigDir()` returns platform-specific path — tests must override via env var or package var.

**Dependencies:** None (no new external dependencies).

**Backward compatibility:** Fully additive. No projects.json → silent no-op. Existing clone behavior unchanged.
