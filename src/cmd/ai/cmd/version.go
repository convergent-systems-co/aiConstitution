package cmd

import (
	"fmt"

	"github.com/convergent-systems-co/aiConstitution/src/cmd/ai/internal/buildinfo"

	"github.com/spf13/cobra"
)

// newVersionCmd implements `ai version`. The version itself is
// embedded at build time via -ldflags "-X .../buildinfo.Version=…".
func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the binary version",
		RunE: func(cmd *cobra.Command, _ []string) error {
			fmt.Println(buildinfo.Version())
			return nil
		},
	}
}
