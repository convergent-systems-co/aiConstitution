package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/convergent-systems-co/aiConstitution/src/internal/compress"
	"github.com/convergent-systems-co/aiConstitution/src/internal/constitution"
	"github.com/convergent-systems-co/aiConstitution/src/internal/paths"
	"github.com/spf13/cobra"
)

// newCompressCmd implements `ai compress`. See design spec §5.
func newCompressCmd() *cobra.Command {
	var personaFlag string
	var checkFlag bool

	c := &cobra.Command{
		Use:   "compress",
		Short: "Regenerate YAML + compact.md derivatives from Constitution.md",
		Long: `compress reads Constitution.md, extracts each ## N. <Persona> Rules section,
and emits two files per section into the AI root (~/.ai/):

  <Persona>.md           YAML derivative (for Claude Code + YAML tools)
  <Persona>.compact.md   Compressed prose (for GitHub Copilot + Markdown tools)

The Governance section is excluded — it contains meta-rules, not AI directives.

With --check, compress exits non-zero if any derivative is stale without
writing files. Suitable for pre-commit hooks and CI.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			root := paths.AIRoot()
			constPath := filepath.Join(root, "Constitution.md")

			data, err := os.ReadFile(constPath) //nolint:gosec
			if err != nil {
				return fmt.Errorf("compress: read Constitution.md from %s: %w", root, err)
			}

			sections := constitution.ParseSections(string(data))
			if len(sections) == 0 {
				return fmt.Errorf("compress: no ## N. <Persona> Rules sections found in Constitution.md")
			}

			if personaFlag != "" {
				var filtered []constitution.Section
				for _, s := range sections {
					if s.Slug == personaFlag {
						filtered = append(filtered, s)
					}
				}
				if len(filtered) == 0 {
					return fmt.Errorf("compress: persona %q not found in Constitution.md", personaFlag)
				}
				sections = filtered
			}

			version := extractVersion(string(data))
			out := cmd.OutOrStdout()
			stale := 0

			for _, s := range sections {
				ds, err := compress.Extract(s, version)
				if err != nil {
					return fmt.Errorf("compress: extract %s: %w", s.Name, err)
				}

				yamlPath := filepath.Join(root, s.FileName)
				compactPath := filepath.Join(root, s.Slug+".compact.md")

				if checkFlag {
					if isStale(yamlPath, ds.Hash) {
						_, _ = fmt.Fprintf(out, "  [stale] %s\n", s.FileName)
						stale++
					} else {
						_, _ = fmt.Fprintf(out, "  [ok]    %s\n", s.FileName)
					}
					continue
				}

				if err := os.WriteFile(yamlPath, ds.YAML, 0o644); err != nil { //nolint:gosec
					return fmt.Errorf("compress: write %s: %w", yamlPath, err)
				}
				if err := os.WriteFile(compactPath, ds.Compact, 0o644); err != nil { //nolint:gosec
					return fmt.Errorf("compress: write %s: %w", compactPath, err)
				}
				_, _ = fmt.Fprintf(out, "  wrote %s + %s\n", s.FileName, filepath.Base(compactPath))
			}

			if stale > 0 {
				return fmt.Errorf("compress: %d derivative(s) are stale — run `ai compress` to regenerate", stale)
			}
			return nil
		},
	}

	c.Flags().StringVar(&personaFlag, "persona", "", "regenerate only this persona slug (e.g., code)")
	c.Flags().BoolVar(&checkFlag, "check", false, "exit non-zero if any derivative is stale (no writes)")
	return c
}

// extractVersion pulls the version string from a Constitution.md header line.
// Format: **Version:** 0.17  Returns "unknown" if not found.
func extractVersion(content string) string {
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "**Version:**") {
			v := strings.TrimPrefix(trimmed, "**Version:**")
			v = strings.Trim(strings.TrimSpace(v), "*")
			return strings.TrimSpace(v)
		}
	}
	return "unknown"
}

// isStale returns true if the derivative file is missing or does not
// contain the expected source hash in its header comment.
func isStale(path, wantHash string) bool {
	data, err := os.ReadFile(path) //nolint:gosec
	if err != nil {
		return true
	}
	return !strings.Contains(string(data), wantHash)
}
