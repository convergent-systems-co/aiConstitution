package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

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
// if the index is absent). When --type is set, only entries whose
// frontmatter type field matches are printed.
func newMemoryListCmd() *cobra.Command {
	var listType string
	c := &cobra.Command{
		Use:   "list",
		Short: "List memories from MEMORY.md",
		RunE: func(cmd *cobra.Command, _ []string) error {
			memDir := paths.MemoryDir()
			path := filepath.Join(memDir, "MEMORY.md")
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
			if listType == "" {
				_, _ = fmt.Fprint(cmd.OutOrStdout(), string(data))
				if !strings.HasSuffix(string(data), "\n") {
					_, _ = fmt.Fprintln(cmd.OutOrStdout())
				}
				return nil
			}
			// Type-filtered: include only entries whose file frontmatter matches.
			sc := bufio.NewScanner(strings.NewReader(string(data)))
			for sc.Scan() {
				line := sc.Text()
				filename := extractMEMORYMDFilename(line)
				if filename == "" {
					// Not an entry line (header, blank, etc.) — print as-is.
					_, _ = fmt.Fprintln(cmd.OutOrStdout(), line)
					continue
				}
				t, _ := readFrontmatterType(filepath.Join(memDir, filename))
				if strings.EqualFold(t, listType) {
					_, _ = fmt.Fprintln(cmd.OutOrStdout(), line)
				}
			}
			return sc.Err()
		},
	}
	c.Flags().StringVar(&listType, "type", "", "filter by type (feedback|reference|project|user)")
	return c
}

// newMemoryShowCmd prints the contents of the memory file for <slug>.
// Resolution order: (1) direct slug.md, (2) MEMORY.md link lookup.
func newMemoryShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show <slug>",
		Short: "Print the contents of a memory entry",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			memDir := paths.MemoryDir()
			slug := args[0]
			path, err := resolveMemoryFile(memDir, slug)
			if err != nil {
				return fmt.Errorf("memory show: %w", err)
			}
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
			src, err := resolveMemoryFile(memDir, name)
			if err != nil {
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

// resolveMemoryFile returns the absolute path of the memory file for slug.
// Resolution order: (1) direct <memDir>/<slug>.md, (2) MEMORY.md link lookup.
func resolveMemoryFile(memDir, slug string) (string, error) {
	direct := filepath.Join(memDir, slug+".md")
	if _, err := os.Stat(direct); err == nil {
		return direct, nil
	}
	// Walk MEMORY.md looking for - [slug](filename.md)
	indexPath := filepath.Join(memDir, "MEMORY.md")
	data, err := os.ReadFile(filepath.Clean(indexPath))
	if err != nil {
		return "", fmt.Errorf("%q not found in memory dir or MEMORY.md", slug)
	}
	sc := bufio.NewScanner(strings.NewReader(string(data)))
	for sc.Scan() {
		line := sc.Text()
		if !strings.Contains(line, "["+slug+"]") {
			continue
		}
		filename := extractMEMORYMDFilename(line)
		if filename == "" {
			continue
		}
		return filepath.Join(memDir, filename), nil
	}
	return "", fmt.Errorf("audit file %q not found in violations/ or overrides/", slug)
}

// extractMEMORYMDFilename parses a MEMORY.md entry line and returns the
// link target filename (e.g. "feedback_foo.md"). Returns "" for non-entry lines.
func extractMEMORYMDFilename(line string) string {
	// Format: - [slug](filename.md) — description
	start := strings.Index(line, "](")
	if start < 0 {
		return ""
	}
	rest := line[start+2:]
	end := strings.Index(rest, ")")
	if end < 0 {
		return ""
	}
	return rest[:end]
}

// readFrontmatterType reads the YAML frontmatter of a memory file and
// returns the value of metadata.type. Returns "" on any error.
func readFrontmatterType(path string) (string, error) {
	//nolint:gosec // G304: internal path derived from MEMORY.md content
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sc := bufio.NewScanner(strings.NewReader(string(data)))
	inFrontmatter := false
	for sc.Scan() {
		line := sc.Text()
		if line == "---" {
			if !inFrontmatter {
				inFrontmatter = true
				continue
			}
			break // end of frontmatter
		}
		if inFrontmatter && strings.HasPrefix(line, "  type:") {
			return strings.TrimSpace(strings.TrimPrefix(line, "  type:")), nil
		}
	}
	return "", nil
}

// newMemoryCodifyCmd promotes a violation file to a memory entry.
// Flags --type, --slug, --description allow callers to override the
// values extracted from the violation file.
func newMemoryCodifyCmd() *cobra.Command {
	var memType, slug, description string
	c := &cobra.Command{
		Use:   "codify <path>",
		Short: "Promote a violation file to a memory finding",
		Long: `codify reads a violation markdown file produced by audit.WriteViolation,
extracts the rule, what-happened, and remediation fields, then writes a
<type>_<slug>.md into ~/.ai/memory/ and appends a pointer to MEMORY.md.

Flags override extracted values when provided.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			created, err := runMemoryCodifyWithOverrides(args[0], memType, slug, description)
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), created)
			return nil
		},
	}
	c.Flags().StringVar(&memType, "type", "feedback", "memory type (feedback|reference|project|user)")
	c.Flags().StringVar(&slug, "slug", "", "memory slug (derived from rule if not set)")
	c.Flags().StringVar(&description, "description", "", "one-line description for MEMORY.md index")
	return c
}

// newMemoryRetireCmd implements `ai memory retire <name>`.
// Resolves ~/.ai/memory/<name>.md, moves it to ~/.ai/memory/retired/<UTC>-<name>.md,
// removes the matching MEMORY.md pointer line, and prints the move.
func newMemoryRetireCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "retire <name>",
		Short: "Retire (remove) a memory entry permanently",
		Long: `retire moves a memory file into ~/.ai/memory/retired/ with a UTC timestamp
prefix and removes its pointer from MEMORY.md. Unlike archive, retire does not
preserve the entry in a recoverable location — use it after the memory has been
codified into the constitution or confirmed as no longer needed.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			// Strip .md suffix if the caller provided it — normalise to bare name.
			name = strings.TrimSuffix(name, ".md")

			memDir := paths.MemoryDir()
			srcPath := filepath.Join(memDir, name+".md")
			if _, err := os.Stat(srcPath); err != nil {
				if errors.Is(err, os.ErrNotExist) {
					return fmt.Errorf("memory retire: %q not found in %s", name+".md", memDir)
				}
				return fmt.Errorf("memory retire: stat %q: %w", srcPath, err)
			}

			retiredDir := filepath.Join(memDir, "retired")
			if err := os.MkdirAll(retiredDir, 0o750); err != nil {
				return fmt.Errorf("memory retire: mkdir retired: %w", err)
			}

			ts := time.Now().UTC().Format("20060102T150405Z")
			dstName := ts + "-" + name + ".md"
			dstPath := filepath.Join(retiredDir, dstName)

			if err := os.Rename(srcPath, dstPath); err != nil {
				return fmt.Errorf("memory retire: move: %w", err)
			}

			// Best-effort: prune the MEMORY.md pointer (absence is not an error).
			_ = prunePointer(memDir, name)

			_, _ = fmt.Fprintf(cmd.OutOrStdout(),
				"Retired: %s → retired/%s\n",
				filepath.Join(memDir, name+".md"),
				dstName,
			)
			return nil
		},
	}
}

// runMemoryCodify reads a violation file and calls WriteFinding with defaults.
func runMemoryCodify(violationPath string) (string, error) {
	return runMemoryCodifyWithOverrides(violationPath, "feedback", "", "")
}

// runMemoryCodifyWithOverrides reads a violation markdown file, extracts
// structured fields, and calls memory.WriteFinding. When slug or description
// are non-empty they override the extracted/derived values.
func runMemoryCodifyWithOverrides(violationPath, memType, slug, description string) (string, error) {
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

	// Apply flag overrides.
	if memType == "" {
		memType = "feedback"
	}
	effectiveSlug := slug
	if effectiveSlug == "" {
		effectiveSlug = memory.FindingSlug(rule)
	}
	effectiveDesc := description
	if effectiveDesc == "" {
		effectiveDesc = rule
	}

	memDir := paths.MemoryDir()
	return memory.WriteFindingFull(memDir, memType, effectiveSlug, effectiveDesc, rule, whatHappened, remediation)
}

// extractViolationField strips the field label prefix and returns the trimmed value.
func extractViolationField(line, prefix string) string {
	return strings.TrimSpace(strings.TrimPrefix(line, prefix))
}
