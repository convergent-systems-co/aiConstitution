package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/convergent-systems-co/aiConstitution/src/cmd/ai/embed"
	"github.com/convergent-systems-co/aiConstitution/src/cmd/ai/internal/buildinfo"
	"github.com/convergent-systems-co/aiConstitution/src/internal/paths"
	"github.com/convergent-systems-co/aiConstitution/src/internal/wizard"

	"github.com/spf13/cobra"
)

// versionLinePattern matches a Markdown bold metadata line of the form
// `**Version:** 0.7` (with arbitrary trailing whitespace). Constituent
// version strings use semver-like numeric+dot tokens — we accept the
// looser shape so future tags like `0.7.1-rc1` still parse.
var versionLinePattern = regexp.MustCompile(`(?m)^\s*\*\*Version:\*\*\s+(\S+)`)

// newVersionCmd implements `ai version`. The binary, the Code.md prose,
// and the embedded questions.yaml each carry their own version — and
// they evolve independently — so the user-facing output names all three.
func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the binary version, Code.md version, and questions.yaml version",
		RunE: func(cmd *cobra.Command, _ []string) error {
			out := cmd.OutOrStdout()
			_, _ = fmt.Fprintf(out, "ai %s\n", buildinfo.Version())
			_, _ = fmt.Fprintf(out, "Code.md %s\n", codeMdVersion())
			_, _ = fmt.Fprintf(out, "questions.yaml %s\n", questionsYAMLVersion())
			return nil
		},
	}
}

// codeMdVersion reads paths.AIRoot()/Code.md and extracts the
// `**Version:** X.Y` line. Returns "not found" when Code.md is absent
// or the marker line is missing — never fabricates a version.
func codeMdVersion() string {
	path := filepath.Join(paths.AIRoot(), "Code.md")
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "not found"
		}
		return "not found"
	}
	m := versionLinePattern.FindSubmatch(data)
	if m == nil {
		return "not found"
	}
	return string(m[1])
}

// questionsYAMLVersion returns the version field of the embedded
// questions.yaml taxonomy, or "unknown" if it fails to parse.
func questionsYAMLVersion() string {
	tax, err := wizard.ParseTaxonomy(embed.QuestionsYAML())
	if err != nil {
		return "unknown"
	}
	if tax.Version == "" {
		return "unknown"
	}
	return tax.Version
}
