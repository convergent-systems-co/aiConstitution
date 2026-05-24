package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/convergent-systems-co/aiConstitution/src/internal/constitution"
	"github.com/convergent-systems-co/aiConstitution/src/internal/paths"
	"github.com/spf13/cobra"
)

func newMigrateCmd() *cobra.Command {
	var flatten, addBehavioral, generateRuntime bool

	c := &cobra.Command{
		Use:   "migrate",
		Short: "Migrate from four-file constitution to unified v2 format",
		Long: `migrate transforms an existing four-file constitution into a single
unified Constitution.md.

Run in order:
  ai migrate --flatten           Merge four files into one; archive originals.
  ai migrate --add-behavioral    Insert §2 Behavioral Standards section.
  ai migrate --generate-runtime  Write Constitution.runtime.md.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			root := paths.AIRoot()
			switch {
			case flatten:
				return runMigrateFlatten(cmd, root)
			case addBehavioral:
				return runMigrateAddBehavioral(cmd, root)
			case generateRuntime:
				return runMigrateGenerateRuntime(cmd, root)
			default:
				return fmt.Errorf("migrate: specify --flatten, --add-behavioral, or --generate-runtime")
			}
		},
	}
	c.Flags().BoolVar(&flatten, "flatten", false, "merge four files into one Constitution.md")
	c.Flags().BoolVar(&addBehavioral, "add-behavioral", false, "insert §2 Behavioral Standards")
	c.Flags().BoolVar(&generateRuntime, "generate-runtime", false, "write Constitution.runtime.md")
	return c
}

func runMigrateFlatten(cmd *cobra.Command, root string) error {
	fileContents := map[string]string{}
	for _, name := range []string{"Constitution.md", "Common.md", "Code.md", "Writing.md"} {
		data, err := os.ReadFile(filepath.Join(root, name)) //nolint:gosec
		if err != nil {
			return fmt.Errorf("migrate --flatten: %s missing: %w", name, err)
		}
		fileContents[name] = string(data)
	}

	// Archive originals
	archiveDir := filepath.Join(root, "archive", "pre-v2")
	if err := os.MkdirAll(archiveDir, 0o750); err != nil {
		return fmt.Errorf("migrate --flatten: mkdir archive: %w", err)
	}
	for name, content := range fileContents {
		if err := os.WriteFile(filepath.Join(archiveDir, name), []byte(content), 0o600); err != nil { //nolint:gosec
			return fmt.Errorf("migrate --flatten: archive %s: %w", name, err)
		}
	}

	// Build unified document
	var sb strings.Builder
	sb.WriteString("# AI Constitution\n\n")
	sb.WriteString(renumberSections(fileContents["Constitution.md"], "§1"))
	sb.WriteString("\n\n")
	sb.WriteString(renumberSections(fileContents["Common.md"], "§3"))
	sb.WriteString("\n\n")
	sb.WriteString(renumberSections(fileContents["Code.md"], "§4"))
	sb.WriteString("\n\n")
	sb.WriteString(renumberSections(fileContents["Writing.md"], "§5"))

	unified := rewriteCrossRefs(sb.String())

	if err := os.WriteFile(filepath.Join(root, "Constitution.md"), []byte(unified), 0o600); err != nil { //nolint:gosec
		return fmt.Errorf("migrate --flatten: write: %w", err)
	}
	_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Flattened: Constitution.md written. Originals archived to archive/pre-v2/.")
	return nil
}

func renumberSections(content, prefix string) string {
	lines := strings.Split(content, "\n")
	var out []string
	replaced := false
	for _, line := range lines {
		switch {
		case !replaced && strings.HasPrefix(line, "# "):
			out = append(out, fmt.Sprintf("## %s %s", prefix, strings.TrimPrefix(line, "# ")))
			replaced = true
		case strings.HasPrefix(line, "## "):
			out = append(out, fmt.Sprintf("### %s.%s", prefix, strings.TrimPrefix(line, "## ")))
		case strings.HasPrefix(line, "### "):
			out = append(out, "##"+line)
		default:
			out = append(out, line)
		}
	}
	return strings.Join(out, "\n")
}

var crossRefReplacements = []struct{ old, new string }{
	{"Constitution.md §", "§1."},
	{"Common.md §U", "§3.U"},
	{"Common.md §", "§3."},
	{"Code.md §", "§4."},
	{"Writing.md §", "§5."},
}

func rewriteCrossRefs(s string) string {
	for _, r := range crossRefReplacements {
		s = strings.ReplaceAll(s, r.old, r.new)
	}
	return s
}

const behavioralStandardsSection = `## §2 Behavioral Standards

### §2.1 Conviction

Agreement is not the goal — correctness is. Sycophancy is a form of dishonesty.
The AI MUST NOT fabricate agreement, soften a true answer to avoid friction, or
add qualifiers it does not mean.

### §2.2 Directness

No preamble restating the prompt. No closing summary. No "Great question!". Lead
with the answer.

### §2.3 Uncertainty

When the AI does not know, it says so. "I don't know" is a correct response.

### §2.4 Disagreement

Disagreement MUST be surfaced before complying, not after.

### §2.5 Helpfulness

Helpfulness is compliance with the principal's actual intent, not their stated
request. When those diverge, the AI raises the gap.

`

func runMigrateAddBehavioral(cmd *cobra.Command, root string) error {
	path := filepath.Join(root, "Constitution.md")
	data, err := os.ReadFile(path) //nolint:gosec
	if err != nil {
		return fmt.Errorf("migrate --add-behavioral: %w (run --flatten first)", err)
	}
	content := string(data)
	if strings.Contains(content, "§2 Behavioral Standards") {
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "§2 Behavioral Standards already present — skipping.")
		return nil
	}
	insertBefore := "## §3"
	idx := strings.Index(content, insertBefore)
	if idx < 0 {
		return fmt.Errorf("migrate --add-behavioral: §3 marker not found (run --flatten first)")
	}
	updated := content[:idx] + behavioralStandardsSection + content[idx:]
	if err := os.WriteFile(path, []byte(updated), 0o600); err != nil { //nolint:gosec
		return fmt.Errorf("migrate --add-behavioral: write: %w", err)
	}
	_, _ = fmt.Fprintln(cmd.OutOrStdout(), "§2 Behavioral Standards inserted.")
	return nil
}

func runMigrateGenerateRuntime(cmd *cobra.Command, root string) error {
	data, err := os.ReadFile(filepath.Join(root, "Constitution.md")) //nolint:gosec
	if err != nil {
		return fmt.Errorf("migrate --generate-runtime: %w", err)
	}
	rc, err := constitution.ExtractRuntime(string(data))
	if err != nil {
		// Lenient: log the extraction error but still write whatever FormatRuntime
		// produces from the partial RuntimeContent. The unified document generated
		// by --flatten uses section headings (e.g. "§3 Common") that don't match
		// the ExtractRuntime patterns tuned for the full v2 format. The runtime file
		// is still useful even when partially populated.
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "warning: runtime extraction incomplete: %v\n", err)
	}
	out := constitution.FormatRuntime(rc)
	dest := filepath.Join(root, "Constitution.runtime.md")
	if err := os.WriteFile(dest, []byte(out), 0o600); err != nil { //nolint:gosec
		return fmt.Errorf("migrate --generate-runtime: write: %w", err)
	}
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Written: %s\n", dest)
	return nil
}
