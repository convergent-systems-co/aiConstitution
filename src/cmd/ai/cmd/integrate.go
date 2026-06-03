package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

// agentsConstSection is the Constitution block written into AGENTS.md by
// `ai init --codex`. Kept as a named constant so tests can reference it
// without re-typing the prose, and so future amenders change it once.
const agentsConstSection = `
## Constitution

Load the following governance document before acting:

` + "```" + `
@~/.ai/Constitution.compact.md
` + "```" + `

This document governs all AI behavior for this repository. See [aiConstitution](https://github.com/convergent-systems-co/aiConstitution) for details.

## Repository Intent

This repository is governed by the user's constitution from ` + "`" + `~/.ai/` + "`" + `. Do not
commit user memory, audit logs, secrets, or generated local state into a
project repository.

## Codex Operating Rules

- Never edit the primary checkout unless the principal explicitly says to do so.
- Before edits, report ` + "`" + `pwd` + "`" + `, current branch, worktree path, and dirty status.
- Work on a feature branch or canonical worktree: ` + "`" + `<repo>/.worktrees/<name>/` + "`" + `
  for repo-local work, or ` + "`" + `~/.ai/worktrees/<name>/` + "`" + ` for persistent cross-repo
  work.
- Never create, switch, or remove worktrees unless the principal has approved
  that operation in the current conversation.
- Never commit, push, merge, rebase, cherry-pick, revert, or open/merge a PR
  unless ` + "`" + `make-build` + "`" + `, ` + "`" + `make build-pr` + "`" + `, or an equivalent explicit release/PR
  command has been invoked by the principal.
- If the current branch is ` + "`" + `main` + "`" + ` or matches ` + "`" + `release/*` + "`" + `, stop before edits.
- Prefer repository entrypoints such as ` + "`" + `make guard` + "`" + `, ` + "`" + `make worktree BRANCH=...` + "`" + `,
  and ` + "`" + `make build-pr` + "`" + ` over ad hoc Git or PR commands when present.
- Treat Claude Code hooks as optional defense-in-depth. For Codex, the portable
  enforcement plane is repository instructions, guard scripts, Make targets,
  and Git hooks.

## aiConstitution Atoms

Use the local aiConstitution atom cache and tool surfaces when available:

- Skills are installed from skill-atoms.com into ~/.ai/skills/ and linked into ~/.codex/skills/.
- Hooks are installed from ai-atoms.com into ~/.ai/hooks/; command-level enforcement runs through wrappers in ~/.ai/bin/.
- Plugins are installed from plugin-atoms.com into ~/.ai/plugins/; follow each plugin's SKILL.md or manifest guidance when a task matches it.
- Prefer pinned atom versions already present under ~/.ai/ before fetching newer registry content.
`

// agentsFileHeader is written only when AGENTS.md does not yet exist.
const agentsFileHeader = `# AI Agents Configuration
`

// agentsIncludeMarker is the string searched for when deciding whether the
// @-include is already present. Using only the marker (not the full section)
// makes the idempotency check robust against minor formatting drift.
const agentsIncludeMarker = "@~/.ai/Constitution.compact.md"

// newInitIntegrateCmd returns the `ai init-integrate` cobra command, which
// wires AI tool integrations into the current working directory or the user's
// home tool configs.
//
// Sub-flags:
//
//	--cursor   create .cursor/rules/constitution.md symlink
//	--codex    write/update AGENTS.md with the @-include block
func newInitIntegrateCmd() *cobra.Command {
	var cursor bool
	var codex bool

	c := &cobra.Command{
		Use:   "init-integrate",
		Short: "Wire AI tool integrations (Cursor, Codex/AGENTS.md)",
		Long: `init-integrate integrates the AI Constitution into local tool configurations.

  --cursor   Creates .cursor/rules/constitution.md → ~/.ai/Constitution.runtime.md
             in the current directory. Idempotent: no-op if already correct.

  --codex    Writes (or appends) an aiConstitution block to AGENTS.md in the
             current directory, instructing Codex agents to load the
             Constitution and local atom-backed skills/hooks/plugins.
             Idempotent: skipped if @~/.ai/Constitution.md already present.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if !cursor && !codex {
				return fmt.Errorf("specify --cursor, --codex, or both. See `ai init-integrate --help`")
			}

			cwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("getting cwd: %w", err)
			}

			home, err := os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("getting home dir: %w", err)
			}
			aiRoot := os.Getenv("AI_ROOT")
			if aiRoot == "" {
				aiRoot = filepath.Join(home, ".ai")
			}

			if cursor {
				if err := runIntegrateCursor(cwd, aiRoot); err != nil {
					return err
				}
			}
			if codex {
				if err := runIntegrateCodex(cwd); err != nil {
					return err
				}
			}
			return nil
		},
	}

	c.Flags().BoolVar(&cursor, "cursor", false, "create .cursor/rules/constitution.md symlink in cwd")
	c.Flags().BoolVar(&codex, "codex", false, "write/update AGENTS.md with @-include block in cwd")
	return c
}

// runIntegrateCursor creates .cursor/rules/constitution.md in cwd as a symlink
// to <aiRoot>/Constitution.runtime.md.
//
// Returns an error if Constitution.runtime.md does not exist.
// Idempotent: if the symlink already points at the right target, it is left
// untouched. A stale symlink is replaced.
func runIntegrateCursor(cwd, aiRoot string) error {
	// Cursor receives the compact form — same as Claude Code and Copilot.
	compactFile := filepath.Join(aiRoot, "Constitution.compact.md")
	if _, err := os.Stat(compactFile); err != nil {
		return fmt.Errorf(
			"Constitution.compact.md not found at %s — run 'ai compress' first",
			compactFile,
		)
	}

	rulesDir := filepath.Join(cwd, ".cursor", "rules")
	if err := os.MkdirAll(rulesDir, 0o750); err != nil {
		return fmt.Errorf("creating %s: %w", rulesDir, err)
	}

	symlinkPath := filepath.Join(rulesDir, "constitution.md")

	if existing, err := os.Readlink(symlinkPath); err == nil {
		if existing == compactFile {
			fmt.Printf("Cursor rule symlink already up to date: %s\n", symlinkPath)
			return nil
		}
		// Stale — remove and recreate.
		if err := os.Remove(symlinkPath); err != nil {
			return fmt.Errorf("removing stale symlink %s: %w", symlinkPath, err)
		}
	}

	if err := symlinkOrCopy(compactFile, symlinkPath); err != nil {
		return fmt.Errorf("creating symlink %s: %w", symlinkPath, err)
	}
	fmt.Printf("Cursor rule symlink created: %s → Constitution.compact.md\n", symlinkPath)
	return nil
}

// runIntegrateCodex writes or updates AGENTS.md in cwd with the Constitution
// @-include block.
//
// Rules:
//   - If AGENTS.md does not exist, create it with the header + section.
//   - If AGENTS.md exists and already contains agentsIncludeMarker, skip (idempotent).
//   - If AGENTS.md exists but lacks the marker, append the section.
func runIntegrateCodex(cwd string) error {
	agentsPath := filepath.Join(cwd, "AGENTS.md")

	existing, err := os.ReadFile(agentsPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("reading %s: %w", agentsPath, err)
	}

	if strings.Contains(string(existing), agentsIncludeMarker) {
		// Already present — idempotent no-op.
		fmt.Printf("AGENTS.md already contains @-include: %s\n", agentsPath)
		return nil
	}

	var content string
	if os.IsNotExist(err) {
		// Create fresh.
		content = agentsFileHeader + agentsConstSection
	} else {
		// Append to existing.
		content = strings.TrimRight(string(existing), "\n") + "\n" + agentsConstSection
	}

	if err := os.WriteFile(agentsPath, []byte(content), 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", agentsPath, err)
	}
	fmt.Printf("AGENTS.md updated: %s\n", agentsPath)
	return nil
}
