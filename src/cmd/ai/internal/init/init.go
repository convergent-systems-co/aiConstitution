// Package init writes the AI-tool integration files at the root of
// ~/.ai/ so that Claude Code, GitHub Copilot CLI, and Codex/AGENTS.md
// surfaces all load the four canonical constitution files on session
// start. See SPEC.md §10.2.
//
// Existing files are never overwritten — the user's edits to
// CLAUDE.md, copilot-instructions.md, or AGENTS.md survive a re-run of
// `ai setup`. Use `ai update --migrate` for guided overwrites.
package init

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// claudeTemplate is the body written to ~/.ai/CLAUDE.md if absent.
const claudeTemplate = `# Claude Instructions

Load and follow ~/.ai/{Constitution,Common,Code,Writing}.md strictly.

These four files are the authoritative governance for every task in
this workspace. The Constitution file is the meta-rule layer, Common
holds the universal operating rules, Code governs technical work, and
Writing governs prose.
`

// copilotTemplate is the body written to ~/.ai/.github/copilot-instructions.md
// if absent.
const copilotTemplate = `# Copilot Instructions

Load and follow ~/.ai/Constitution.compact.md strictly.

This file is the authoritative governance for every task in this workspace.

## Operating Rules

- Never edit the primary checkout unless the principal explicitly says to do so.
- Before edits, report pwd, current branch, worktree path, and dirty status.
- Work on a feature branch or canonical worktree: <repo>/.worktrees/<name>/
  for repo-local work, or ~/.ai/worktrees/<name>/ for persistent cross-repo
  work.
- Never create, switch, or remove worktrees unless the principal has approved
  that operation in the current conversation.
- Never commit, push, merge, rebase, cherry-pick, revert, or open/merge a PR
  unless make-build, make build-pr, or an equivalent explicit release/PR
  command has been invoked by the principal.
- If the current branch is main or matches release/*, stop before edits.
- Prefer repository entrypoints such as make guard, make worktree BRANCH=...,
  and make build-pr over ad hoc Git or PR commands when present.
- Treat Claude Code hooks as optional defense-in-depth. For Copilot, the
  portable enforcement plane is repository instructions, guard scripts, Make
  targets, and Git hooks.
`

// agentsTemplate is the body written to ~/.ai/AGENTS.md (for Codex
// and other AGENTS.md-aware tools) if absent.
const agentsTemplate = `# AGENTS Instructions

Load and follow ~/.ai/Constitution.compact.md strictly.

This file is the authoritative governance for every task in this workspace.

## Operating Rules

- Never edit the primary checkout unless the principal explicitly says to do so.
- Before edits, report pwd, current branch, worktree path, and dirty status.
- Work on a feature branch or canonical worktree: <repo>/.worktrees/<name>/
  for repo-local work, or ~/.ai/worktrees/<name>/ for persistent cross-repo
  work.
- Never create, switch, or remove worktrees unless the principal has approved
  that operation in the current conversation.
- Never commit, push, merge, rebase, cherry-pick, revert, or open/merge a PR
  unless make-build, make build-pr, or an equivalent explicit release/PR
  command has been invoked by the principal.
- If the current branch is main or matches release/*, stop before edits.
- Prefer repository entrypoints such as make guard, make worktree BRANCH=...,
  and make build-pr over ad hoc Git or PR commands when present.
- Treat Claude Code hooks as optional defense-in-depth. For Codex, the portable
  enforcement plane is repository instructions, guard scripts, Make targets,
  and Git hooks.
`

// EnsureToolFiles ensures the ~/.ai/ directory tree and the three AI
// tool integration files exist. Returns the absolute paths of files
// actually written (not pre-existing ones). Existing files are left
// untouched.
//
// aiRoot is the canonical ~/.ai/ root, typically paths.AIRoot().
func EnsureToolFiles(aiRoot string) ([]string, error) {
	if aiRoot == "" {
		return nil, errors.New("init: aiRoot is empty")
	}
	if err := os.MkdirAll(aiRoot, 0o750); err != nil {
		return nil, fmt.Errorf("init: mkdir %s: %w", aiRoot, err)
	}

	type entry struct {
		path string
		body string
	}
	entries := []entry{
		{filepath.Join(aiRoot, "CLAUDE.md"), claudeTemplate},
		{filepath.Join(aiRoot, ".github", "copilot-instructions.md"), copilotTemplate},
		{filepath.Join(aiRoot, "AGENTS.md"), agentsTemplate},
	}

	written := make([]string, 0, len(entries))
	for _, e := range entries {
		ok, err := writeIfAbsent(e.path, []byte(e.body))
		if err != nil {
			return written, err
		}
		if ok {
			written = append(written, e.path)
		}
	}
	return written, nil
}

// writeIfAbsent writes data to dst with 0o640 permissions if dst does
// not already exist. Returns (true, nil) when a file was created;
// (false, nil) when dst pre-existed; (false, err) on filesystem error.
func writeIfAbsent(dst string, data []byte) (bool, error) {
	if _, err := os.Stat(dst); err == nil {
		return false, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return false, err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o750); err != nil {
		return false, err
	}
	if err := os.WriteFile(dst, data, 0o600); err != nil { //nolint:gosec // G306: integration files are user-readable config, 0600 is correct
		return false, err
	}
	return true, nil
}
