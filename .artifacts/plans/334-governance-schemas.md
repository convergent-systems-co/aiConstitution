# Plan: governance/schemas/ — settings and persona JSON Schemas

**Issue:** #334
**Branch:** feat/334-governance-schemas
**Date:** 2026-05-26

## Objective

Create `governance/schemas/` with two JSON Schema Draft-07 files:
1. `settings.schema.json` — validates `~/.config/aiConstitution/settings.toml`
2. `persona.schema.json` — persona atom config with `domains[]` (§17.2 target)

## Scope

Files to create:
- `.artifacts/plans/334-governance-schemas.md` (this file)
- `governance/schemas/settings.schema.json`
- `governance/schemas/persona.schema.json`

## Approach

1. Derive `settings.schema.json` from `settings.toml.example`:
   - Parse every TOML section into a JSON Schema object property
   - Carry comments forward as `description` strings
   - `additionalProperties: true` on all nested objects (forward-compatible)
   - `schemaVersion` is required at the top level

2. Write `persona.schema.json` using the §17.2 target schema:
   - `domains` is an array (not the legacy `domain` string)
   - Required: `name`, `domains`, `role`, `version`
   - `additionalProperties: true`

## Alternatives

| Alternative | Pros | Cons | Verdict |
|---|---|---|---|
| Single combined schema | One file to maintain | Conflates two very different shapes | Rejected |
| Two separate files (chosen) | Clear single responsibility per file | Two commits | Chosen |
| JSON Schema 2020-12 | Modern | Spec calls for draft-07 explicitly | Rejected |

## Testing strategy

TDD skip applies — JSON Schema files, no executable code.
Validate JSON syntax with: `python3 -c "import json, sys; json.load(open(f))"`

## Risk assessment

- Forward compatibility: mitigated by `additionalProperties: true`
- $id URL pattern: follow `https://schema-atoms.com/json-schema/<slug>/<version>/schema.json`

## Out of scope

- Atomization workflow (§17.4 — Phase F)
- All other schemas in `governance/schemas/`
- Migration of existing reviewer YAML files to `domains[]`
