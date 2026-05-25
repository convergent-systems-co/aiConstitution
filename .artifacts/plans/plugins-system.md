# Plan: Plugin System — Manifest, Install, Enable/Disable/Status, Update

**Issues:** #235, #236, #237, #238 (Epic #28 Plugin System)
**Branch:** feature/plugins-system
**Worktree:** /Users/itsfwcp/workspace/convergent-system-co/aiConstitution/.worktrees/feature/plugins-system
**Date:** 2026-05-24

---

## Objective

Implement the `ai plugins` subcommands (install, enable, disable, status, update) with a manifest
parser for `~/.ai/plugins/<name>/manifest.yaml`, replacing the existing stubs.

---

## Alternatives Table

| Alternative | Pros | Cons | Verdict |
|---|---|---|---|
| PluginManifest in internal/plugins + cmd/ai imports it via go workspace | Clean separation; manifest logic is reusable outside cmd; follows paths/config pattern | Slightly more files | Chosen |
| Embed all logic in plugins.go with no internal package | Fewer files | Manifest parsing not reusable; mixes I/O with CLI glue | Rejected |
| Use gopkg.in/yaml.v3 for manifest parsing | Correct structured parsing, handles all YAML edge cases | Requires adding dep to src/internal go.mod | Chosen — dep is already present in main-branch internal go.mod; idiomatic |
| Hand-parse YAML manifest | No new dep | Fragile; reinvents yaml parser; breaks on multiline strings | Rejected |
| Store plugin enabled state in settings.toml [plugins].enabled | Consistent with existing PluginsSettings struct | Settings.toml is user-managed config, not runtime state; comingling state is fragile | Rejected |
| Store enabled state in ~/.config/aiConstitution/plugins.json | Dedicated state file, easy to read/write atomically | Separate file | Chosen — spec §11 requires this |

---

## Scope

**Files to create:**
- `src/internal/plugins/manifest.go` — PluginManifest struct, ParseManifest, plugin directory helpers (#235)
- `src/internal/plugins/manifest_test.go` — TDD tests for ParseManifest (#235)
- `src/cmd/ai/cmd/plugins_test.go` — TDD tests for install/enable/disable/status/update (#236–238)

**Files to modify:**
- `src/cmd/ai/cmd/plugins.go` — replace stubs with real implementation (#236, #237, #238)
- `src/internal/go.mod` — add gopkg.in/yaml.v3 direct dependency
- `src/internal/paths/paths.go` — add PluginsDir() and PluginsStateFile() helpers

---

## Approach

1. Add `PluginsDir()` and `PluginsStateFile()` to `paths.go`.
2. Add `gopkg.in/yaml.v3` to `src/internal/go.mod` via `go get`.
3. Create `src/internal/plugins/manifest.go`:
   - `PluginManifest` struct with `name`, `version`, `description`, `source`, `skills` fields.
   - `ParseManifest(path string) (*PluginManifest, error)` — reads yaml, validates required fields.
4. Create `src/internal/plugins/manifest_test.go` (TDD Writer step — RED first).
5. Create `src/cmd/ai/cmd/plugins_test.go` (TDD Writer step — RED first).
6. Implement `src/cmd/ai/cmd/plugins.go`:
   - `plugins install <url-or-path> [--force]` — download/copy, unpack .tar.gz, validate manifest.
   - `plugins enable <name>` — add to plugins.json enabled list.
   - `plugins disable <name>` — remove from plugins.json enabled list.
   - `plugins status` — list all installed plugins with enabled marker.
   - `plugins update <name>` — re-install from manifest source URL.
7. Wire cmd/ai go.mod to reference internal module (already handled by go.work).

---

## Testing Strategy

- `manifest_test.go`: real temp dirs, no HTTP; tests ParseManifest with valid + invalid yaml.
- `plugins_test.go`: real temp dirs; creates local tar.gz fixtures; tests install/enable/disable/status/update.
  - Install: creates a .tar.gz fixture with a manifest.yaml, verifies it unpacks correctly.
  - Enable/disable: manipulates plugins.json in a temp ConfigDir, verifies state.
  - Status: verifies output formatting for empty and populated plugin lists.
  - Update: verifies re-install from a local source path and version change reporting.
- No real HTTP in any test; --force flag tested via local reinstall.

---

## Risk Assessment

| Risk | Mitigation |
|---|---|
| tar.gz extraction path traversal | Validate each archive entry path; reject `..` entries |
| plugins.json concurrent write corruption | Atomic write via temp file + rename |
| install with malformed manifest leaves partial state | Clean up partially-unpacked dir on error |
| yaml.v3 not yet in worktree go.mod | Run `go get gopkg.in/yaml.v3` in src/internal before writing code |

---

## Dependencies

- `gopkg.in/yaml.v3` in `src/internal/go.mod` (must run before Coder A writes manifest.go)
- `src/internal/plugins/` package must exist before Coder B imports it in plugins.go

---

## Backward Compatibility

- Existing `plugins list` stub is left in place (spec §11.5 — list is out of scope for these issues).
- `enable`/`disable`/`status`/`update`/`install` stubs are replaced.
- The new `plugins.json` state file is created fresh on first use; absence is not an error.
