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
@~/.ai/Constitution.md
` + "```" + `

This document governs all AI behavior for this repository. See [aiConstitution](https://github.com/convergent-systems-co/aiConstitution) for details.
`

// agentsFileHeader is written only when AGENTS.md does not yet exist.
const agentsFileHeader = `# AI Agents Configuration
`

// agentsIncludeMarker is the string searched for when deciding whether the
// @-include is already present. Using only the marker (not the full section)
// makes the idempotency check robust against minor formatting drift.
const agentsIncludeMarker = "@~/.ai/Constitution.md"

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

  --codex    Writes (or appends) an @-include block to AGENTS.md in the current
             directory, instructing Codex agents to load the Constitution.
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
	runtimeFile := filepath.Join(aiRoot, "Constitution.runtime.md")
	if _, err := os.Stat(runtimeFile); err != nil {
		return fmt.Errorf(
			"Constitution.runtime.md not found at %s — run 'ai generate runtime' first",
			runtimeFile,
		)
	}

	rulesDir := filepath.Join(cwd, ".cursor", "rules")
	if err := os.MkdirAll(rulesDir, 0o750); err != nil {
		return fmt.Errorf("creating %s: %w", rulesDir, err)
	}

	symlinkPath := filepath.Join(rulesDir, "constitution.md")

	if existing, err := os.Readlink(symlinkPath); err == nil {
		if existing == runtimeFile {
			fmt.Printf("Cursor rule symlink already up to date: %s\n", symlinkPath)
			return nil
		}
		// Stale — remove and recreate.
		if err := os.Remove(symlinkPath); err != nil {
			return fmt.Errorf("removing stale symlink %s: %w", symlinkPath, err)
		}
	}

	if err := os.Symlink(runtimeFile, symlinkPath); err != nil {
		return fmt.Errorf("creating symlink %s: %w", symlinkPath, err)
	}
	fmt.Printf("Cursor rule symlink created: %s\n", symlinkPath)
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
