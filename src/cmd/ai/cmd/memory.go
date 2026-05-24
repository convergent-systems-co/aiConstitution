package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/convergent-systems-co/aiConstitution/src/internal/memory"
	"github.com/convergent-systems-co/aiConstitution/src/internal/paths"
	"github.com/spf13/cobra"
)

// newMemoryCmd implements `ai memory {list,codify,retire}`.
// See SPEC.md §3 and §6.
func newMemoryCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "memory",
		Short: "Inspect and curate ~/.ai/memory/",
		Long: `memory operates on the cross-tool memory layer at ~/.ai/memory/.

Subcommands:
  list      Enumerate memories (optionally filtered by type).
  codify    Promote a memory to a constitutional amendment.
  retire    Remove a memory (typically after codification).

See SPEC.md §3, §6, and Common.md §5.1.`,
	}

	// list
	var listType string
	list := &cobra.Command{
		Use:   "list",
		Short: "List memories",
		RunE: func(cmd *cobra.Command, _ []string) error {
			notice("memory list:", "type filter:", listType)
			return stub("memory list", "§3 + Common.md §5.1")
		},
	}
	list.Flags().StringVar(&listType, "type", "", "filter by type (feedback|reference|project|user)")

	// codify
	codify := &cobra.Command{
		Use:   "codify <path>",
		Short: "Promote a violation file to a memory finding",
		Long: `codify reads a violation markdown file produced by audit.WriteViolation,
extracts the rule, what-happened, and remediation fields, then writes a
feedback_<slug>.md into ~/.ai/memory/ and appends a pointer to MEMORY.md.

The path argument must point to an existing violation file.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			created, err := runMemoryCodify(args[0])
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), created)
			return nil
		},
	}

	// retire
	retire := &cobra.Command{
		Use:   "retire <slug>",
		Short: "Retire (remove) a memory entry",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			notice("memory retire:", args[0])
			return stub("memory retire", "§6")
		},
	}

	c.AddCommand(list, codify, retire)
	return c
}

// runMemoryCodify reads a violation markdown file, extracts the structured
// fields, calls memory.WriteFinding, and returns the path of the created
// feedback file.
func runMemoryCodify(violationPath string) (string, error) {
	//nolint:gosec // G304: path is a user-supplied argument — intentional open
	f, err := os.Open(violationPath)
	if err != nil {
		return "", fmt.Errorf("memory codify: open %q: %w", violationPath, err)
	}
	defer f.Close()

	var rule, whatHappened, remediation string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		switch {
		case strings.HasPrefix(line, "- **File / Rule violated:**"):
			rule = extractViolationField(line, "- **File / Rule violated:**")
		case strings.HasPrefix(line, "- **What happened:**"):
			whatHappened = extractViolationField(line, "- **What happened:**")
		case strings.HasPrefix(line, "- **Remediation:**"):
			remediation = extractViolationField(line, "- **Remediation:**")
		}
	}
	if err := sc.Err(); err != nil {
		return "", fmt.Errorf("memory codify: read %q: %w", violationPath, err)
	}
	if rule == "" {
		return "", fmt.Errorf("memory codify: no '**File / Rule violated:**' line found in %q", violationPath)
	}

	return memory.WriteFinding(paths.MemoryDir(), rule, whatHappened, remediation)
}

// extractViolationField strips the field label prefix and returns the trimmed value.
func extractViolationField(line, prefix string) string {
	return strings.TrimSpace(strings.TrimPrefix(line, prefix))
}
