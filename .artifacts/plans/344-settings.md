# Plan — feat/344: implement `ai settings get/set/edit/reset`

## Objective
Replace the four stub subcommands in `settings.go` with fully-functional
implementations so `ai settings get/set/edit/reset` operate on
`~/.config/aiConstitution/settings.toml`.

## Rationale

| Alternative | Pros | Cons | Verdict |
|---|---|---|---|
| Use existing `internal/config` package | Reuses Load/Save, TOML library already wired | Dot-path navigation over generic `any` map needed on top | Chosen |
| Re-implement TOML I/O in cmd package | No cross-module concern | Duplicates Load/Save logic, TOML lib indirect in cmd/ai | Rejected |
| Use reflect-based field lookup | Clean dot-notation | Complex, over-engineered for get/set of scalar leaves | Rejected for v1 — use map-based approach |

## Scope

Files to create:
- `.artifacts/plans/344-settings.md` (this file)
- `src/cmd/ai/cmd/settings_test.go`

Files to modify:
- `src/cmd/ai/cmd/settings.go`

## Approach

### settings get `<key>`
1. Call `paths.SettingsTOML()` (honors `$AICONST_CONFIG_DIR` for tests).
2. Decode the TOML into a `map[string]any` — preserves all keys without
   needing the typed struct for read-only navigation.
3. Walk dot-path segments into nested tables.
4. Print the leaf value via `fmt.Fprintf(cmd.OutOrStdout(), "%v\n", val)`.
5. Return error if file absent or key not found.

### settings set `<key>=<value>`
1. Parse the single arg on `=` (first `=` only; value may contain `=`).
2. Load existing file into `map[string]any` (or start empty).
3. Navigate to parent table, creating sub-tables as needed.
4. Set leaf to the parsed value: try `strconv.ParseBool`, then
   `strconv.ParseInt`, then keep as string. This mirrors TOML scalar types.
5. Re-encode to TOML and write atomically.
6. Validate round-trip: decode written bytes.

### settings edit
1. Resolve `$EDITOR` (fallback: `vi`), following the same `execEditor`
   pattern already in `amend.go`.
2. If settings.toml doesn't exist, write the defaults from
   `settings.toml.example` (embedded via `//go:embed`).
3. Open editor.
4. After editor exits, validate that the result parses as TOML; if not,
   print error and offer to re-edit (one re-edit loop).

### settings reset `[--accept-defaults]`
1. Source of defaults: embedded `settings.toml.example` (same embed
   used by `edit`).
2. Without `--accept-defaults`: print a diff-style summary of what would
   change vs current file (or "no current file"), ask `[y/N]`.
3. With `--accept-defaults`: write immediately.
4. Write atomically using `os.CreateTemp` + `os.Rename`.

## Testing strategy
- `TestSettingsGet_ExistingKey` — write temp settings.toml, run get, assert output.
- `TestSettingsGet_MissingKey` — expect error containing "not found".
- `TestSettingsGet_FileAbsent` — no settings.toml, expect error.
- `TestSettingsSet_NewKey` — set a new leaf, read back, assert value.
- `TestSettingsSet_ExistingKey` — update existing key, file must be valid TOML after.
- `TestSettingsReset_AcceptDefaults` — writes defaults, file parses, schemaVersion present.

## Risk assessment
- TOML comment loss on `settings set`: `BurntSushi/toml` encoder does not
  preserve inline comments. Accepted for v1 — spec says "rewrite clean TOML".
- Nested-table auto-creation: walking `a.b.c` where `a` doesn't exist requires
  careful `map[string]any` insertion to avoid type assertion panics.

## Backward compatibility
- No existing persistent state changes; settings.toml is user-owned.
- `settings.go` currently returns `stub(...)` errors; replacing stubs is
  the purpose of this PR.

## Dependencies
- `github.com/BurntSushi/toml` already indirect in `cmd/ai/go.mod`; must
  promote to direct (add explicit import in settings.go).
- `//go:embed` for `settings.toml.example` requires the file to be
  accessible from the package directory — will embed via a helper in
  `settings.go` pointing at `../../../../settings.toml.example`.
