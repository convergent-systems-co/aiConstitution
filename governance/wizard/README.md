# governance/wizard/

The wizard question taxonomy lives at the repo root (`questions.yaml`)
because it's the single human-edited contract between the wizard
flow and the prose templates. Per `SPEC.md §4 + §5`.

This directory is the **install-time mount point**: when `ai setup`
installs into `~/.ai/`, the wizard expects to find
`~/.ai/governance/wizard/questions.yaml`. The Makefile's `install`
target (TBD — morning work) copies the repo's `questions.yaml`
into place.

For now (v0.8 scaffold) the canonical questions.yaml is at the repo
root and is symlinked into `~/.ai/` by `ai setup`. The pointer below
documents the contract so future contributors don't accidentally
duplicate the file.
