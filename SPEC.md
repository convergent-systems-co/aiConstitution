# aiConstitution Implementation Specification

**Status:** v1.0.0-draft  
**Full spec:** [`specs/aiConstitution-spec-v1.0.0-draft.md`](specs/aiConstitution-spec-v1.0.0-draft.md)

The AI Constitution is a version-controlled governance layer for AI-assisted
software development. The user maintains a single unified `Constitution.md`
in `~/.ai/` that defines how every AI tool they use must behave. Hooks enforce rules deterministically.
An audit system captures violations. Violations become amendments that update the
files. The files become atoms, distributed and cached locally for immutable
version-pinned use. The `ai` binary is the CLI that orchestrates everything.

This spec defines what to build. The substrate (signing, caching, lifecycle,
distribution) is defined by the Atom Spec v1.1.0 and the per-catalog specs.
This spec does not repeat those definitions; it cites them.

The AI Constitution Spec is a `design-spec` atom in schema-atoms. It conforms
to Atom Spec v1.1.0. Its atom reference:

```
schema-atoms/design-spec/ai-constitution-spec@1.0.0-draft
```

Olympus (the Convergent Systems AI OS) and the AI Constitution are peers. Both
consume atoms from the same 25-catalog ecosystem under `convergent-systems.co`,
cache atoms locally using the same cache substrate, and run as Phase 1 bootstrap
applications under single stewardship. Neither depends on the other at the
substrate level.

---

*This file exists at the repo root so `ai-constitution.convergent-systems.co/spec` resolves via the Astro content collection. For the complete specification, see the link above.*
