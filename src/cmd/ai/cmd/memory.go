package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/convergent-systems-co/aiConstitution/src/internal/memory"
	"github.com/convergent-systems-co/aiConstitution/src/internal/paths"
	"github.com/spf13/cobra"
)

// newMemoryCmd implements `ai memory {list,show,archive,codify,retire}`.
// See SPEC.md §3 and §6.
func newMemoryCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "memory",
		Short: "Inspect and curate ~/.ai/memory/",
		Long: `memory operates on the cross-tool memory layer at ~/.ai/memory/.

Subcommands:
  list      Enumerate memories (reads MEMORY.md).
  show      Print the contents of a memory file.
  archive   Move a memory to archived/ and remove its MEMORY.md pointer.
  codify    Promote a memory to a constitutional amendment.
  retire    Remove a memory (typically after codification).

See SPEC.md §3, §6, and Common.md §5.1.`,
	}

	c.AddCommand(
		newMemoryListCmd(),
		newMemoryShowCmd(),
		newMemoryArchiveCmd(),
		newMemoryCodifyCmd(),
		newMemoryRetireCmd(),
	)
	return c
}

// newMemoryListCmd prints the contents of MEMORY.md (or "(no memories)"
// if the index is absent). This is the canonical surface for "what
// memories are loaded on this machine"; it does NOT walk the directory
// because not every .md is a memory entry.
func newMemoryListCmd() *cobra.Command {
	var listType string
	c := &cobra.Command{
		Use:   "list",
		Short: "List memories from MEMORY.md",
		RunE: func(cmd *cobra.Command, _ []string) error {
			_ = listType // reserved for type-filter once frontmatter parsing lands
			path := filepath.Join(paths.MemoryDir(), "MEMORY.md")
			data, err := os.ReadFile(filepath.Clean(path))
			if err != nil {
				if errors.Is(err, os.ErrNotExist) {
					_, _ = fmt.Fprintln(cmd.OutOrStdout(), "(no memories)")
					return nil
				}
				return fmt.Errorf("memory list: %w", err)
			}
			if len(strings.TrimSpace(string(data))) == 0 {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "(no memories)")
				return nil
			}
			_, _ = fmt.Fprint(cmd.OutOrStdout(), string(data))
			if !strings.HasSuffix(string(data), "\n") {
				_, _ = fmt.Fprintln(cmd.OutOrStdout())
			}
			return nil
		},
	}
	c.Flags().StringVar(&listType, "type", "", "filter by type (feedback|reference|project|user)")
	return c
}

// newMemoryShowCmd prints the contents of <name>.md under MemoryDir.
// The name argument is taken as-is — callers should pass the bare slug
// (no .md extension). Failure to find the file is an error per
// Common.md P2 (Honesty Over Compliance).
func newMemoryShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show <name>",
		Short: "Print the contents of a memory entry",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := filepath.Join(paths.MemoryDir(), args[0]+".md")
			data, err := os.ReadFile(filepath.Clean(path))
			if err != nil {
				return fmt.Errorf("memory show: %w", err)
			}
			_, _ = fmt.Fprint(cmd.OutOrStdout(), string(data))
			if !strings.HasSuffix(string(data), "\n") {
				_, _ = fmt.Fprintln(cmd.OutOrStdout())
			}
			return nil
		},
	}
}

// newMemoryArchiveCmd moves <name>.md into MemoryDir/archived/ and
// strips its pointer line from MEMORY.md. This is the standard exit
// for a memory once it's been codified or otherwise resolved; the
// archive keeps history without polluting the active list.
func newMemoryArchiveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "archive <name>",
		Short: "Move a memory to archived/ and drop its MEMORY.md pointer",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			memDir := paths.MemoryDir()
			src := filepath.Join(memDir, name+".md")
			if _, err := os.Stat(src); err != nil {
				return fmt.Errorf("memory archive: %w", err)
			}
			archiveDir := filepath.Join(memDir, "archived")
			if err := os.MkdirAll(archiveDir, 0o750); err != nil {
				return fmt.Errorf("memory archive: mkdir archived: %w", err)
			}
			dst := filepath.Join(archiveDir, name+".md")
			if err := os.Rename(src, dst); err != nil {
				return fmt.Errorf("memory archive: move: %w", err)
			}
			if err := prunePointer(memDir, name); err != nil {
				return fmt.Errorf("memory archive: prune MEMORY.md: %w", err)
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Archived: %s\n", dst)
			return nil
		},
	}
}

// prunePointer rewrites MEMORY.md, removing any line that references
// the given memory name. The match is conservative: a line that contains
// the name as a substring (after the leading "- [") is dropped. Absence
// of MEMORY.md is silently fine — archive can still succeed even if no
// index existed.
func prunePointer(memDir, name string) error {
	indexPath := filepath.Join(memDir, "MEMORY.md")
	data, err := os.ReadFile(filepath.Clean(indexPath))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	out := &strings.Builder{}
	sc := bufio.NewScanner(strings.NewReader(string(data)))
	for sc.Scan() {
		line := sc.Text()
		if strings.Contains(line, name) {
			continue
		}
		out.WriteString(line)
		out.WriteByte('\n')
	}
	if err := sc.Err(); err != nil {
		return err
	}
	//nolint:gosec // G306: user config file (MEMORY.md); 0o600 is intentional
	return os.WriteFile(indexPath, []byte(out.String()), 0o600)
}

// newMemoryCodifyCmd is TL-1's codify implementation, unchanged here
// other than being extracted into its own constructor so the parent
// command stays compact.
func newMemoryCodifyCmd() *cobra.Command {
	return &cobra.Command{
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
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), created)
			return nil
		},
	}
}

// newMemoryRetireCmd is reserved for the §6 codification workflow.
// Retained as a stub here so the surface matches what the spec lists.
func newMemoryRetireCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "retire <slug>",
		Short: "Retire (remove) a memory entry",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			notice("memory retire:", args[0])
			return stub("memory retire", "§6")
		},
	}
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
	defer func() { _ = f.Close() }()

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
