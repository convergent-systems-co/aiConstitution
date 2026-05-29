package cmd

import (
	"fmt"

	"github.com/convergent-systems-co/aiConstitution/src/cmd/ai/embed"
	"github.com/convergent-systems-co/aiConstitution/src/cmd/ai/internal/buildinfo"
	"github.com/convergent-systems-co/aiConstitution/src/internal/wizard"

	"github.com/spf13/cobra"
)

// newVersionCmd implements `ai version`. The binary and the embedded
// questions.yaml each carry their own version — they evolve independently.
// Code.md was removed in the unified-constitution model (v1.4.x); it is
// now a section of Constitution.md, not a separate file.
func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the binary version and questions.yaml version",
		RunE: func(cmd *cobra.Command, _ []string) error {
			out := cmd.OutOrStdout()
			_, _ = fmt.Fprintf(out, "ai %s\n", buildinfo.Version())
			_, _ = fmt.Fprintf(out, "questions.yaml %s\n", questionsYAMLVersion())
			return nil
		},
	}
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
