# Migration: document-writer persona deprecation

**Date:** 2026-05-24
**Issue:** #258
**Status:** Complete (no file rename required — see findings)

---

## Summary

This document records the investigation and outcome of the document-writer
persona deprecation task for the `aiConstitution` repository.

---

## Findings

### Persona file search

The task called for renaming `~/.ai/personas/document-writer.yaml` (or
equivalent) to `document-writer.deprecated.yaml` and updating references.

Search results:

- `~/.ai/personas/` — directory does not exist on this machine
- `governance/personas/` — directory does not exist in the repo worktree
- No `document-writer.yaml`, `document-writer.md`, or any variant of the
  filename was found anywhere in the repository or in `~/.ai/`

### Spec references

`SPEC.md` and `AI-CONSTITUTION-SPEC.md` both reference
`governance/personas/agentic/document-writer.md` as a **future artifact** to
be split in the v0.4+ product migration design — not as a currently existing
file. The SPEC describes the planned removal:

> `document-writer.md` — **Removed outright.** Not deprecated, not shimmed —
> gone. The conflation was a defect, not a feature worth preserving for
> backward compatibility.

This confirms the file was never committed to the repository; it appears only
as a reference in the migration design documentation.

### CLAUDE.md and skill file search

Searched `CLAUDE.md`, `~/.claude/` skill files, and all markdown files in the
repo for references to `document-writer`. Findings:

- `SPEC.md` and `AI-CONSTITUTION-SPEC.md` reference it as a spec artifact
  (not as a currently wired persona).
- No active persona configuration, settings.toml, or skill file references
  `document-writer` as a live wired persona.

---

## Outcome

**No file rename was performed** — there is no file to rename.

**No reference updates were required** — no active configuration points to a
`document-writer` persona.

The planned behavior described in the SPEC (user prompt via `ai update --migrate`
to pick a successor persona) will be implemented when the CLI reaches the
v0.4 milestone that introduces `prose-writer` and `tech-writer` as the
replacement personas.

---

## Successor guidance

Per `SPEC.md §<document-writer-removal>` and the constitution at
`~/.ai/Writing.md §5.1` (drafting modes), the role that `document-writer`
conflated should be split as follows:

| Old persona | Replacement | Governing section |
|---|---|---|
| `document-writer` (prose work) | `prose-writer` (v0.4+) | `~/.ai/Writing.md` — all sections |
| `document-writer` (technical docs) | `tech-writer` (v0.4+) | `~/.ai/Code.md §9` |

Until the successor personas ship, use the Writing.md and Code.md §9 rules
directly as your governing document for prose and documentation work
respectively.

---

## References

- `SPEC.md` — §0.4 changelog entry on document-writer split
- `AI-CONSTITUTION-SPEC.md` — same section
- `~/.ai/Writing.md` — governs prose work (current active replacement)
- `~/.ai/Code.md §9` — governs documentation work (current active replacement)
- Issue #258 — original task
