# Plan — #382 ai audit override/violation + #384 ai memory retire

## Objective
Replace stub implementations in `audit override`, `audit violation`, and `memory retire` with working code that writes canonical markdown files to `~/.ai/audit/` and `~/.ai/memory/retired/`.

## Scope

### Files to modify
- `src/cmd/ai/cmd/audit.go` — replace stub RunE for override and violation subcommands with real flag-driven implementations
- `src/cmd/ai/cmd/audit_test.go` — add tests for override and violation happy paths, dir auto-creation, schema validation
- `src/cmd/ai/cmd/memory.go` — replace stub RunE for `retire` subcommand
- `src/cmd/ai/cmd/memory_test.go` — add tests for retire happy path, missing file error, MEMORY.md line removal

## Approach

### #382 — audit override

1. Add flags to the `override` subcommand (--tool, --section, --scope, --strict, --relaxed, --risk, --confirmation, --artifacts).
2. On RunE: validate required flags, form a UTC ISO8601 filename (`20060102T150405Z`), create `~/.ai/audit/overrides/` dir if missing, write the markdown file, print the path.
3. Template:
```
# Override — <UTC timestamp>

- **Tool / Agent:** <tool>
- **Section / Rule relaxed:** <section>
- **Scope:** <scope>
- **Strict behavior:** <strict>
- **Relaxed behavior:** <relaxed>
- **Risk acknowledged:** <risk>
- **Reasoning (AI):** <confirmation>
- **Principal confirmation:** <confirmation>
- **Artifacts affected:** <artifacts>
```

### #382 — audit violation

1. Add flags to the `violation` subcommand (--section, --what, --noticed, --remediation, --amendment).
2. On RunE: validate required flags, write to `~/.ai/audit/violations/<UTC>.md`, print path.
3. Template:
```
# Violation — <UTC timestamp>

- **Section / Rule violated:** <section>
- **What happened:** <what>
- **How noticed:** <noticed>
- **Remediation:** <remediation>
- **Proposed amendment (if any):** <amendment>
```

### #384 — memory retire

1. Accept positional `<name>` arg.
2. Resolve `~/.ai/memory/<name>.md` (append `.md` if not present).
3. Create `~/.ai/memory/retired/` if needed.
4. Move file to `~/.ai/memory/retired/<UTC>-<name>.md`.
5. Remove matching line from `~/.ai/memory/MEMORY.md`.
6. Print: `Retired: ~/.ai/memory/<name>.md → retired/<timestamp>-<name>.md`
7. Error if source file does not exist.

## Testing strategy

- TDD: write failing tests first, then implement.
- For each subcommand: happy path (all flags, file written with correct content), directory auto-creation, schema field validation by reading back the file.
- For `memory retire`: happy path, missing file error, MEMORY.md line removal.

## Risks
- Concurrent writes to the same UTC second could collide; acceptable for CLI audit commands.
- MEMORY.md mutation needs to preserve unrelated lines.

## Alternatives considered
- Prompt-based TTY input: deferred (flags-only per spec).
