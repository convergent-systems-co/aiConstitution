# Plan — atoms fetch/fork/publish/list (#261–#265)

**Date:** 2026-05-24
**Branch:** feature/atoms
**Spec:** SPEC.md §7.9 + §7.10; Code.md §11.1

---

## Objective

Implement `ai atoms fetch`, `ai atoms fork`, `ai atoms publish --dry-run`, and `ai atoms list` as real, working commands replacing the existing stubs. Atoms are constitution-layer bundles distributed as `.tar.gz` archives with an `atom.toml` manifest.

---

## Alternatives Table

| Alternative | Pros | Cons | Verdict |
|---|---|---|---|
| Add everything to existing `src/internal/atoms/atoms.go` | One file | Conflates existing Ref/Kind/Meta system with new lifecycle types | Rejected |
| New subpackage `src/internal/atoms/lifecycle/` | Clean isolation | Extra module complexity given the monorepo internal module structure | Rejected |
| Extend `src/internal/atoms/atoms.go` with new types only, add lifecycle helpers in same file | Clean, discoverable, no module changes | File grows but remains focused on atoms domain | **Chosen** |
| Separate `src/internal/atomsindex/` package | Very clean | Overkill for four functions | Rejected |

---

## Scope

### Files to create
- `src/internal/atoms/manifest.go` — `AtomManifest`, `ParseAtomTOML`, `UpdateAtomsIndex`, `ReadAtomsIndex`
- `src/cmd/ai/cmd/atoms_test.go` — failing tests (TDD writer phase)

### Files to modify
- `src/cmd/ai/cmd/atoms.go` — replace stubs with implementations for fetch, fork, publish, list
- `src/internal/go.mod` — add `github.com/BurntSushi/toml` (TOML parsing)
- `src/cmd/ai/go.mod` — add `github.com/BurntSushi/toml` (indirect, needed by atoms.go which uses internal pkg)

### Files to verify (read-only)
- `src/internal/paths/paths.go` — already has `AIRoot()`, `ConfigDir()`
- `src/cmd/ai/cmd/root.go` — `newAtomsCmd()` already registered

---

## Approach

### Phase A (Coder A) — internal/atoms/manifest.go

1. Define `AtomManifest` struct (name, version, sha256, upstream_url, upstream_ref, files)
2. Implement `ParseAtomTOML(path string) (AtomManifest, error)` — reads atom.toml via BurntSushi/toml
3. Implement `WriteAtomTOML(path string, m AtomManifest) error`
4. Define `AtomsIndexEntry` struct (name, version, path, upstream)
5. Implement `ReadAtomsIndex(indexPath string) ([]AtomsIndexEntry, error)`
6. Implement `UpdateAtomsIndex(indexPath string, entry AtomsIndexEntry) error` — upserts by name+version

Add BurntSushi/toml to `src/internal/go.mod`.

### Phase B (Coder B, depends on A) — fetch + list

**fetch** (`#261`, `#262`):
- Accept positional arg: `<id-or-url>` where id is `catalog/name@version` or direct HTTPS/file URL
- Parse arg: if contains `://` treat as URL; else parse as `name@version`
- Download to `os.CreateTemp("", "ai-atom-*.tar.gz")`
- Compute SHA256 of downloaded bytes
- Extract tar.gz to `~/.ai/atoms/<name>/` (via `paths.AIRoot()`)
- Read `atom.toml` from extracted dir via `ParseAtomTOML`
- Verify SHA256 matches manifest (if manifest has sha256 field)
- Append/update entry in `~/.config/aiConstitution/atoms.json` via `UpdateAtomsIndex`
- Print: `Fetched <name>@<version> → ~/.ai/atoms/<name>/`

**list** (`#265`):
- Read `atoms.json` via `ReadAtomsIndex`
- Print aligned table: `name | version | upstream | local-path`
- Print `(no atoms installed)` when empty or file absent

### Phase C (Coder C, depends on B) — fork + publish

**fork** (`#263`):
- Accept positional arg: `<name>`, flag `--as <local-name>`
- Locate `~/.ai/atoms/<name>/` (must exist)
- Copy entire directory to `~/.ai/atoms/<local-name>/` (default: `<name>-local`)
- Parse atom.toml in copy; add `upstream_ref = "<name>@<version>"`; write back
- Update atoms.json with forked entry
- Print: `Forked <name> → <local-name>. Edit ~/.ai/atoms/<local-name>/ and run ai atoms publish.`

**publish** (`#264`):
- Accept flags `--name`, `--version`, `--dry-run`
- Walk `~/.ai/` excluding `audit/`, `.git/`, `atoms/`
- Hash all files' contents concatenated into one SHA256
- Build/update `~/.ai/atoms/<name>/atom.toml` with: name, version, sha256, files list
- `--dry-run`: print `Would publish: <name>@<version> (<n> files, SHA256: <hash>)`
- Full publish: print `Publishing not yet supported. Use --dry-run to preview.`

---

## Testing Strategy

All tests use temp dirs via env vars `AI_ROOT` and `AICONST_CONFIG_DIR`. No network required — `file://` URLs pointing to fixture tar.gz.

1. `TestAtomsFetchExtractsFiles` — fetch from `file://` URL, verify dir created under AI_ROOT
2. `TestAtomsFetchReadsManifest` — after fetch, parsed atom.toml has correct name/version
3. `TestAtomsFetchRejectsHashMismatch` — atom.toml sha256 field does not match download; expect non-zero exit
4. `TestAtomsForkCopiesDir` — fork creates `<name>-local/` dir with files
5. `TestAtomsForkAddsUpstreamRef` — forked atom.toml has `upstream_ref` field
6. `TestAtomsPublishDryRun` — publish --dry-run writes atom.toml with correct fields
7. `TestAtomsListEmpty` — list with no atoms.json prints `(no atoms installed)`

---

## Risk Assessment

| Risk | Mitigation |
|---|---|
| BurntSushi/toml not yet in go.sum | `go get` in internal module before building |
| tar.gz extraction path traversal | Validate extracted paths start with expected prefix |
| atoms.json concurrent writes | Single process CLI; no concurrency concern for MVP |
| file:// URL on Windows | Tests are Darwin; use `filepath.ToSlash`; no Windows CI |

---

## Dependencies

- BurntSushi/toml (add to `src/internal/go.mod`)
- `src/internal/paths` — already provides AIRoot(), ConfigDir() with env-var override for tests
- `src/cmd/ai/cmd/root.go` — `newAtomsCmd()` already registered; no root.go changes needed

## Backward Compatibility

The existing stub commands had `--kind` and `--name` flags. The new fetch signature changes to a positional `<id-or-url>`. This is acceptable — the commands were stubs with no users.

---

*Plan written per Code.md §11.1. Alternatives table present. Three alternatives considered.*
