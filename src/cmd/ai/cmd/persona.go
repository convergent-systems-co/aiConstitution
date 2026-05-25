package cmd

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/convergent-systems-co/aiConstitution/src/internal/constitution"
	"github.com/convergent-systems-co/aiConstitution/src/internal/paths"
	"github.com/spf13/cobra"
)

// newPersonaCmd implements `ai persona {list,show,share}`.
// See SPEC.md §3 (v0.6 additions) and §7.9.
func newPersonaCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "persona",
		Short: "Inspect persona atoms (agentic + reviewer)",
		Long: `persona surfaces the two kinds of persona atoms:

  agentic   (kind: "agentic")  — loaded by `+"`"+`ai mode <name>`+"`"+`
  reviewer  (kind: "reviewer") — invoked by /spawn review panels

Both kinds resolve via persona-atoms.com, cached locally at
~/.config/aiConstitution/.persona-cache/<kind>/<name>/<version>/.

See SPEC.md §3, §7.9.`,
	}

	// list
	var listKind, listDomain string
	list := &cobra.Command{
		Use:   "list",
		Short: "List persona atoms, grouped by kind",
		RunE: func(cmd *cobra.Command, _ []string) error {
			notice("persona list:", "kind:", listKind, "domain:", listDomain)
			return stub("persona list", "§3 + §7.9.1")
		},
	}
	list.Flags().StringVar(&listKind, "kind", "", "filter by kind (agentic|reviewer)")
	list.Flags().StringVar(&listDomain, "domain", "", "filter reviewer personas by domain (engineering|security|architecture|documentation|finops|data)")

	// show
	c.AddCommand(&cobra.Command{
		Use:   "show <name>",
		Short: "Show resolved persona content + metadata",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			notice("persona show:", args[0])
			return stub("persona show", "§7.9.1")
		},
	})

	// share
	var shareDomain bool
	share := &cobra.Command{
		Use:   "share <name>",
		Short: "File a persona draft upstream (agentic by default; --domain for reviewer)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			notice("persona share:", args[0], "(reviewer:", shareDomain, ")")
			return stub("persona share", "§7.9.3")
		},
	}
	share.Flags().BoolVar(&shareDomain, "domain", false, "share as a reviewer persona (YAML, kind: reviewer)")

	// new
	newCmd := &cobra.Command{
		Use:   "new",
		Short: "Draft a new persona section in Constitution.md and compress",
		Long: `new prompts for a persona name and description, appends a template
## N. <Name> Rules section to Constitution.md, then prints the command
to run ai compress to emit the YAML and compact.md derivatives.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			out := cmd.OutOrStdout()
			root := paths.AIRoot()
			constPath := filepath.Join(root, "Constitution.md")

			data, err := os.ReadFile(constPath) //nolint:gosec
			if err != nil {
				return fmt.Errorf("persona new: read Constitution.md: %w", err)
			}
			sections := constitution.ParseSections(string(data))
			// +2: Governance section (excluded from ParseSections) + next new section.
			nextNum := len(sections) + 2

			name, err := promptLine(cmd.InOrStdin(), out, "Persona name (e.g., Security, DataScience): ")
			if err != nil {
				return err
			}
			name = strings.TrimSpace(name)
			if name == "" {
				return fmt.Errorf("persona new: name cannot be empty")
			}

			desc, err := promptLine(cmd.InOrStdin(), out, "Brief description (one sentence): ")
			if err != nil {
				return err
			}

			template := buildPersonaTemplate(nextNum, name, strings.TrimSpace(desc))

			f, err := os.OpenFile(constPath, os.O_APPEND|os.O_WRONLY, 0o644) //nolint:gosec
			if err != nil {
				return fmt.Errorf("persona new: open Constitution.md: %w", err)
			}
			if _, werr := fmt.Fprint(f, template); werr != nil {
				_ = f.Close()
				return fmt.Errorf("persona new: write template: %w", werr)
			}
			_ = f.Close()

			_, _ = fmt.Fprintf(out, "\nTemplate appended to Constitution.md at ## %d. %s Rules\n", nextNum, name)
			_, _ = fmt.Fprintf(out, "Edit %s to fill in the rules, then run:\n\n  ai compress --persona %s\n\n",
				constPath, strings.ToLower(name))
			return nil
		},
	}

	c.AddCommand(list, share, newCmd)
	return c
}

func promptLine(in io.Reader, out io.Writer, prompt string) (string, error) {
	_, _ = fmt.Fprint(out, prompt)
	scanner := bufio.NewScanner(in)
	if scanner.Scan() {
		return scanner.Text(), nil
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	return "", nil
}

func buildPersonaTemplate(num int, name, desc string) string {
	slug := strings.ToLower(name)
	return fmt.Sprintf(`

## %d. %s Rules

<!-- persona: %s | description: %s -->

**%d.1 Rule label.** MUST description of rule.

**%d.2 Rule label.** SHOULD description of rule.

**%d.3 Rule label.** MAY description of rule.
`, num, name, slug, desc, num, num, num)
}
