# Plan: Remove hook .py scripts from embed (#423)

**Objective:** Remove the 12 hook `.py` scripts (+ 5 test files) from `src/cmd/ai/embed/hooks/`, since the ai-atoms.com catalog now serves all hook scripts via `script` fields. Infrastructure files stay in embed.

## Scope

### Files to delete
- `src/cmd/ai/embed/hooks/audit.py`
- `src/cmd/ai/embed/hooks/audit-command.py`
- `src/cmd/ai/embed/hooks/branch-guard.py`
- `src/cmd/ai/embed/hooks/checkpoint-tick.py`
- `src/cmd/ai/embed/hooks/destructive-gh-guard.py`
- `src/cmd/ai/embed/hooks/destructive-kubectl-guard.py`
- `src/cmd/ai/embed/hooks/destructive-terraform-guard.py`
- `src/cmd/ai/embed/hooks/no-verify-strip.py`
- `src/cmd/ai/embed/hooks/op-redact.py`
- `src/cmd/ai/embed/hooks/secret-block.py`
- `src/cmd/ai/embed/hooks/secret-precommit.py`
- `src/cmd/ai/embed/hooks/worktree-guard.py`
- `src/cmd/ai/embed/hooks/test_audit.py`
- `src/cmd/ai/embed/hooks/test_branch_guard.py`
- `src/cmd/ai/embed/hooks/test_op_redact.py`
- `src/cmd/ai/embed/hooks/test_secret_block.py`
- `src/cmd/ai/embed/hooks/test_worktree_guard.py`

### Keep in embed
- `_lib.py` — shared library (infrastructure)
- `patterns.json` — secret scan patterns (infrastructure)
- `patterns.local.json.example` — example (infrastructure)
- `command-wrappers.toml` — wiring config (infrastructure)
- `tests/__init__.py` — Python package marker

### Files to modify
- `src/cmd/ai/cmd/hooks.go` — update installOneHook, installAllHooksAndWire, runHooksAvailable, runHooksEvaluate, hookFilesFromEmbed

## Approach

1. Delete 12 hook .py scripts + 5 test .py files from embed
2. Update `installOneHook`: remove embed fallback; return error if not in catalog
3. Update `installAllHooksAndWire`: require catalog success, use `extractInfrastructureFiles` for embed
4. Add `extractInfrastructureFiles` helper
5. Update `runHooksAvailable`: change "Embedded hooks" section to show only infrastructure files
6. Update `runHooksEvaluate`: read from `~/.ai/hooks/` instead of embed FS
7. Update `hookFilesFromEmbed`: no longer used (remove or leave for now)
8. Update tests that check embedded hook content

## Testing Strategy
- `go test ./src/cmd/ai/...` must pass
- `golangci-lint run ./src/cmd/ai/...` must be clean
- `go build -o /dev/null ./src/cmd/ai/.` must succeed

## Risk Assessment
- `TestEvaluateProducesPerHookOutput` currently expects at least one `[✓]` or `[✗]` — with no embed hooks, evaluate now reads from `~/.ai/hooks/` which will be empty in test env → test will still pass as evaluate returns success with 0 hooks, but output won't have `[✓]`/`[✗]`. Need to update this test.
- `TestHooksAvailable*` tests check for "Embedded hooks" header — will need updating.

## Backward Compatibility
- `ai hooks install <name>` now requires catalog access; no embed fallback. This is intentional per issue.
- `ai hooks install --all` still works; catalog is the source for hook scripts, embed provides infrastructure files.
