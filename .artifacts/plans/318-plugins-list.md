# Plan: feat/318 — Implement `ai plugins list`

**Objective:** Replace the `plugins list` stub with a real implementation that lists installed plugins in a tabwriter-formatted table.

## Rationale

| Alternative | Pros | Cons | Verdict |
|---|---|---|---|
| tabwriter table (NAME/VERSION/STATUS/SOURCE) | Aligned output, matches spec, consistent with rest of codebase | Slightly more code than plain fmt.Fprintf | **Chosen** |
| Plain fmt.Fprintf (like `status` does now) | Simpler | Not aligned, doesn't match acceptance criteria | Rejected |
| Do nothing (keep stub) | Zero effort | Breaks `ai plugins list` | Rejected |

## Scope

Files to modify:
- `src/cmd/ai/cmd/plugins.go` — replace the list stub command with a real `runPluginsList` function
- `src/cmd/ai/cmd/plugins_test.go` — add `TestPluginsList_NoPlugins`, `TestPluginsList_OneEnabled`, `TestPluginsList_OneDisabled`

## Approach

1. **TDD Writer**: Write three failing tests in `plugins_test.go`:
   - `TestPluginsList_NoPlugins` — empty plugins dir → `(no plugins installed)`
   - `TestPluginsList_OneEnabled` — one plugin installed+enabled → row with `enabled` and column headers
   - `TestPluginsList_OneDisabled` — one plugin installed+disabled → row with `disabled`

2. **Coder**: Replace the stub in `newPluginsCmd()` list sub-command with a real `runPluginsList(cmd)` function that:
   - Reads `paths.PluginsDir()` (same as `runPluginsStatus`)
   - Filters entries to directories with a `manifest.yaml`
   - Prints `(no plugins installed)` if none found
   - Otherwise prints a tabwriter table with headers: `NAME`, `VERSION`, `STATUS`, `SOURCE`
   - STATUS = `enabled` if in state.Enabled, else `disabled`
   - SOURCE comes from `manifest.Source`
   - Uses `cmd.OutOrStdout()` for all output
   - Always returns nil (exit 0)

## Testing strategy

Three table-driven tests using the same fixture pattern as existing tests:
- Set `AI_ROOT` and `AICONST_CONFIG_DIR` env vars via `t.Setenv`
- Create plugin directories and manifest.yaml files manually
- Write `plugins.json` for enable/disable state
- Execute via `cmd.NewRootCmd()` with `root.SetOut(out)`
- Assert output strings

## Risk assessment

- Low risk: the data path (`paths.PluginsDir`, `plugins.ParseManifest`, `loadPluginsState`) is already tested by status tests.
- The tabwriter flush must be called or output is empty — easy mistake to make.

## Dependencies

- `text/tabwriter` (standard library, no new imports needed beyond what's already present)
- All helpers already exist (`loadPluginsState`, `paths.PluginsDir`, `plugins.ParseManifest`)

## Backward compatibility

This replaces a stub that returned an error. Any caller that previously got an error will now get table output — strictly additive.
