# AI Agents Configuration

## Constitution

Load the following governance document before acting:

```
@~/.ai/Constitution.compact.md
```

This document governs all AI behavior for this repository. See [aiConstitution](https://github.com/convergent-systems-co/aiConstitution) for details.

## Repository Intent

This repository is the tool half of the aiConstitution system: the Go CLI,
embedded hooks, wrapper templates, setup flows, and documentation that install
and enforce a user's constitution from `~/.ai/`. It is not the user's personal
constitution repository and must not commit user memory, audit logs, secrets,
or generated local state.

## Codex and Copilot Operating Rules

These rules are explicit because Codex and GitHub Copilot cannot assume the
Claude Code hook runtime is active.

- Never edit the primary checkout unless the principal explicitly says to do so.
- Before edits, report `pwd`, current branch, worktree path, and dirty status.
- Work on a feature branch or canonical worktree: `<repo>/.worktrees/<name>/`
  for repo-local work, or `~/.ai/worktrees/<name>/` for persistent cross-repo
  work.
- Never create, switch, or remove worktrees unless the principal has approved
  that operation in the current conversation.
- Never commit, push, merge, rebase, cherry-pick, revert, or open/merge a PR
  unless `make-build`, `make build-pr`, or an equivalent explicit release/PR
  command has been invoked by the principal.
- If the current branch is `main` or matches `release/*`, stop before edits.
- Use `make guard`, `make worktree BRANCH=...`, and `make build-pr` instead of
  ad hoc `git worktree add`, `git commit`, `git push`, or `gh pr merge`.
- Treat `~/.ai/hooks/` and Claude settings as optional defense-in-depth. For
  Codex and Copilot, the portable enforcement plane is this repository's
  instructions, guard script, Make targets, and Git hooks.

## aiConstitution Atoms

Use the local aiConstitution atom cache and tool surfaces when available:

- Skills are installed from skill-atoms.com into ~/.ai/skills/ and linked into ~/.codex/skills/.
- Hooks are installed from ai-atoms.com into ~/.ai/hooks/; command-level enforcement runs through wrappers in ~/.ai/bin/.
- Plugins are installed from plugin-atoms.com into ~/.ai/plugins/; follow each plugin's SKILL.md or manifest guidance when a task matches it.
- Prefer pinned atom versions already present under ~/.ai/ before fetching newer registry content.
