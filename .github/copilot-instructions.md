# GitHub Copilot Instructions

Load and follow `@~/.ai/Constitution.compact.md` before acting in this
repository.

This repository is the tool half of the aiConstitution system: the Go CLI,
embedded hooks, wrapper templates, setup flows, and documentation that install
and enforce a user's constitution from `~/.ai/`. It is not the user's personal
constitution repository and must not commit user memory, audit logs, secrets,
or generated local state.

## Non-Negotiables

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
- Treat Claude Code hooks as optional defense-in-depth. For Copilot, the
  portable enforcement plane is this repository's instructions, guard script,
  Make targets, and Git hooks.
