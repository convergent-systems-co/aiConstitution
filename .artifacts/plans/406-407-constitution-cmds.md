# Plan: Issues #406 and #407 — `ai constitution setup` + `ai constitution restore --url`

**Objective:** Add two subcommands to `ai constitution`: `setup` (delegates to `ai setup`) and `restore --url` (clones a git repo and copies governance files into `~/.ai/`).

## Scope

**Files to modify:**
- `src/cmd/ai/cmd/constitution_cmd.go` — add `setup` subcommand, extend `restore` subcommand with `--url` and `--dry-run` flags, add `runConstitutionRestoreURL` function

**Files to create:**
- `src/cmd/ai/cmd/constitution_cmd_test.go` entries (file already exists, new tests appended)

## Approach

### Issue #406 — `ai constitution setup`

1. In `newConstitutionCmd()`, add a `setup` subcommand.
2. `setup.RunE` calls `newSetupCmd().RunE(setupCmd, args)` — same package, direct call works.
3. `copyFile` and `copyDir` already exist in `restore.go` with signatures `copyFile(src, dst string, mode os.FileMode)` and `copyDir(src, dst string)`.

### Issue #407 — `ai constitution restore --url`

1. Extend `newConstitutionRestoreCmd()` to add `--url` and `--dry-run` flags.
2. In `RunE`, if `--url` is set, call `runConstitutionRestoreURL`.
3. `runConstitutionRestoreURL`: clone to tempdir, copy governance files, back up existing `~/.ai/` with `os.Rename`, create new `~/.ai/`.
4. Reuse `copyFile(src, dst, mode)` and `copyDir(src, dst)` from `restore.go`.
5. The `AI_ROOT` env var is respected (via `resolveAIRoot()`).

## Testing Strategy

- `TestConstitutionSetup_CommandExists` — verify subcommand registered
- `TestConstitutionRestoreURL_DryRun` — mock git repo, dry-run, assert "would copy"
- `TestConstitutionRestoreURL_Restores` — mock git repo, assert file written
- Build and lint pass

## Risk Assessment

- `copyFile` in `restore.go` takes 3 args: `(src, dst string, mode os.FileMode)`. Use `os.FileInfo.Mode()` when calling from restore URL.
- `git clone` in tests: use `git init` + commit to create a local bare URL with `file://` scheme — no network needed.
