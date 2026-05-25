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

// newStatusCmd implements `ai status`. The existing surface from v0.1,
// extended in §3.2 and §3.3 to include the review-cadence nag and the
// last-seen-version check.
func newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Print a short status report (sync state, review cadence, doctor warnings)",
		Long: `status prints a one-screen status report:
  - AI root path
  - Constitution file presence (~/.ai/{Constitution,Common,Code,Writing}.md)
  - Active mode + composing atoms (TODO: §3.2)
  - Sync state — last push, last pull (TODO: §3.2)
  - Review cadence — count of pending review candidates (TODO: §3.2)
  - Doctor warnings — broken symlinks, missing hooks, stale binary (TODO: §3.3)

See SPEC.md §3.2 + §3.3.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			root := paths.AIRoot()
			out := cmd.OutOrStdout()
			_, _ = fmt.Fprintf(out, "AI Root: %s\n\n", root)

			_, _ = fmt.Fprintln(out, "Constitution files:")
			status := constitution.FileStatus(root)
			for _, name := range constitution.FileNames {
				mark := "present"
				if !status[name] {
					mark = "MISSING"
				}
				_, _ = fmt.Fprintf(out, "  %-30s %s\n", name, mark)
			}
			if status["Constitution.local.md"] {
				_, _ = fmt.Fprintf(out, "  %-30s %s\n", "Constitution.local.md", "present (local override)")
			}

			_, _ = fmt.Fprintln(out, "\nActive personas (CLAUDE.md block):")
			claudeMD := paths.ClaudeMD()
			active := readActivePersonas(claudeMD)
			if len(active) == 0 {
				_, _ = fmt.Fprintln(out, "  (no personas block found — run `ai compress`)")
			} else {
				for _, p := range active {
					_, _ = fmt.Fprintf(out, "  %s\n", p)
				}
			}
			return nil
		},
	}
}

// readActivePersonas parses the <!-- ai:personas --> block in CLAUDE.md
// and returns the persona slugs in order.
func readActivePersonas(claudeMDPath string) []string {
	data, err := os.ReadFile(claudeMDPath) //nolint:gosec
	if err != nil {
		return nil
	}
	content := string(data)
	start := strings.Index(content, "<!-- ai:personas")
	end := strings.Index(content, "<!-- /ai:personas -->")
	if start == -1 || end == -1 || end <= start {
		return nil
	}
	block := content[start:end]
	var slugs []string
	for _, line := range strings.Split(block, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "@") {
			continue
		}
		base := filepath.Base(line[1:])
		if !strings.HasSuffix(base, ".md") {
			continue
		}
		slug := strings.ToLower(strings.TrimSuffix(base, ".md"))
		slugs = append(slugs, slug)
	}
	return slugs
}
