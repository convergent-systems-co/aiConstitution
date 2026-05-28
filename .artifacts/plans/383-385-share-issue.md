# Plan: Issues #383 and #385 — Share Subcommands + ai issue file

## Objective
Implement the `runShareUpstream` helper and wire it into 5 share subcommands (#383),
then implement `ai issue file` (#385).

## Approach

### #383 — share subcommands

1. Create `src/cmd/ai/cmd/share.go` with:
   - `runShareUpstream(name, filePath, upstreamRepo, token string) error`
   - `getShareToken() (string, error)` — resolves GH_TOKEN → GITHUB_TOKEN → settings
   - Reads local file, checks settings.upstream.shareEnabled (to be added)
   - POSTs to GitHub Issues API

2. Add `ShareEnabled bool` to `UpstreamSettings` in `src/internal/config/config.go`
   (default true)

3. Wire into existing stubs in:
   - hooks.go → `hooks share`
   - mode.go → `mode share` (find the stub)
   - persona.go → `persona share`
   - profile.go → `profile share`
   - skills.go → `skills share`

4. Create `src/cmd/ai/cmd/share_test.go` with:
   - Mock HTTP server: verify title + body
   - Config gate: shareEnabled=false → no HTTP, exit 0
   - File-not-found → error

### #385 — ai issue file

1. Rewrite `src/cmd/ai/cmd/issue.go`:
   - `--type` (default "task"): bug|finding|task|story → label `kind/<type>`
   - `--major`: adds label `severity/major`
   - `--title` (required): issue title
   - `--body`: issue body (from stdin if absent)
   - `--from-audit <path>`: read audit file, use as body
   - Detect repo from `git remote get-url origin`
   - POST to GitHub Issues API

2. Create `src/cmd/ai/cmd/issue_test.go` with:
   - Happy path with --title/--body/--type
   - --from-audit reads file correctly
   - Missing --title → error
   - No token → clear error

## Files to Create/Modify

- `src/cmd/ai/cmd/share.go` (NEW)
- `src/cmd/ai/cmd/share_test.go` (NEW)
- `src/cmd/ai/cmd/issue.go` (REWRITE)
- `src/cmd/ai/cmd/issue_test.go` (NEW)
- `src/internal/config/config.go` (ADD ShareEnabled field)
- `src/cmd/ai/cmd/hooks.go` (wire share)
- `src/cmd/ai/cmd/mode.go` (wire share)
- `src/cmd/ai/cmd/persona.go` (wire share)
- `src/cmd/ai/cmd/profile.go` (wire share)
- `src/cmd/ai/cmd/skills.go` (wire share)
