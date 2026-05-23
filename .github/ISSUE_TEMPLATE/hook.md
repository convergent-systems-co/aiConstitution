---
name: Hook contribution
about: Submit a new hook (or improvement to an existing hook) from a finding
title: "[Hook] "
labels: ["kind/hook", "status/triage"]
---

<!-- File this when `ai hooks share <name>` is invoked, or manually
     when you want to contribute a hook. See SPEC.md §9. -->

## Hook name

`<name>.py` (or `.go`, `.sh`, `.js`)

## Language

- [ ] Python (stdlib only — preferred for this repo)
- [ ] Bash (stdlib only)
- [ ] Go
- [ ] Node

## Registration surface

Which event / surface this hook listens on:

- [ ] `PreToolUse` (Claude Code)
- [ ] `PostToolUse`
- [ ] `SessionStart` / `SessionEnd`
- [ ] `pre-commit` (git)
- [ ] Command-wrapper `preHook` / `postHook` (specify command + subcommand)
- [ ] Other (specify)

## Motivation

<!-- One paragraph: what pattern did the hook arise from, and what's
     the failure mode it prevents? Cite the originating audit record
     (path inside ~/.ai/audit/) if applicable — redact secrets per
     ~/.ai/Common.md §4.5 before pasting any excerpts. -->

## Hook source

```python
#!/usr/bin/env python3
# Paste the hook source here.
```

## Test cases (optional)

<!-- Concrete inputs that the hook should accept / reject. -->

## Originating audit record

<!-- Path within ~/.ai/audit/violations/ or ~/.ai/audit/overrides/, if
     this hook grew from a logged finding. Redacted excerpt OK. -->

## Spec references

- `SPEC.md §9` (hook authorship loop)
- `SPEC.md §10.5` (command-wrapper facade) — if applicable
