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

// newDoctorCmd implements `ai doctor`. See SPEC.md §3.3.
func newDoctorCmd() *cobra.Command {
	var fix bool
	var resetHead string

	c := &cobra.Command{
		Use:   "doctor",
		Short: "Detect and repair structural damage to the ~/.ai/ tree",
		Long: `doctor checks the predictable failure modes of the constitution
tree and either reports them or fixes them.

Checks currently implemented:
  1. Constitution files present (~/.ai/{Constitution,Common,Code,Writing}.md)

See SPEC.md §3.3 for the full 10-point check list (in progress).`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			_ = fix
			_ = resetHead
			root := paths.AIRoot()
			status := constitution.FileStatus(root)
			out := cmd.OutOrStdout()

			allOK := true
			for _, name := range constitution.FileNames {
				present := status[name]
				mark := "✓"
				if !present {
					mark = "✗"
					allOK = false
				}
				_, _ = fmt.Fprintf(out, "  [%s] %s\n", mark, name)
			}
			if status["Constitution.local.md"] {
				_, _ = fmt.Fprintf(out, "  [✓] Constitution.local.md (local override)\n")
			}
			// Check 2: CLAUDE.md personas block.
			claudeMD := paths.ClaudeMD()
			if doctorCheckPersonasBlock(claudeMD) {
				_, _ = fmt.Fprintf(out, "  [✓] CLAUDE.md personas block\n")
			} else {
				_, _ = fmt.Fprintf(out, "  [✗] CLAUDE.md personas block missing — run `ai mode` or `ai compress` to create it\n")
				allOK = false
			}

			// Check 3: derivative file presence.
			constPath := filepath.Join(root, "Constitution.md")
			if data, err := os.ReadFile(constPath); err == nil { //nolint:gosec
				for _, s := range constitution.ParseSections(string(data)) {
					yamlPath := filepath.Join(root, s.FileName)
					if _, statErr := os.Stat(yamlPath); statErr != nil {
						_, _ = fmt.Fprintf(out, "  [✗] %s missing — run `ai compress`\n", s.FileName)
						allOK = false
					} else {
						_, _ = fmt.Fprintf(out, "  [✓] %s present\n", s.FileName)
					}
				}
			}

			if !allOK {
				return fmt.Errorf("doctor: issues detected in %s", root)
			}
			_, _ = fmt.Fprintln(out, "Constitution files: OK")
			return nil
		},
	}

	c.Flags().BoolVar(&fix, "fix", false, "attempt to repair each detected issue")
	c.Flags().StringVar(&resetHead, "reset-head", "", "reset HEAD to <ref> (use with caution)")
	return c
}

// doctorCheckPersonasBlock returns true if claudeMDPath contains the
// <!-- ai:personas --> block.
func doctorCheckPersonasBlock(claudeMDPath string) bool {
	data, err := os.ReadFile(claudeMDPath) //nolint:gosec
	if err != nil {
		return false
	}
	return strings.Contains(string(data), "<!-- ai:personas")
}
