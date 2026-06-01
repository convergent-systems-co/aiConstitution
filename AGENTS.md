# AI Agents Configuration

## Constitution

Load the following governance document before acting:

```
@~/.ai/Constitution.compact.md
```

This document governs all AI behavior for this repository. See [aiConstitution](https://github.com/convergent-systems-co/aiConstitution) for details.

## aiConstitution Atoms

Use the local aiConstitution atom cache and tool surfaces when available:

- Skills are installed from skill-atoms.com into ~/.ai/skills/ and linked into ~/.codex/skills/.
- Hooks are installed from ai-atoms.com into ~/.ai/hooks/; command-level enforcement runs through wrappers in ~/.ai/bin/.
- Plugins are installed from plugin-atoms.com into ~/.ai/plugins/; follow each plugin's SKILL.md or manifest guidance when a task matches it.
- Prefer pinned atom versions already present under ~/.ai/ before fetching newer registry content.
