package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/convergent-systems-co/aiConstitution/src/internal/constitution"
	"github.com/convergent-systems-co/aiConstitution/src/internal/paths"
	"github.com/spf13/cobra"
)

func newGenerateCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "generate",
		Short: "Generate derived artifacts from Constitution.md",
	}
	c.AddCommand(newGenerateRuntimeCmd())
	return c
}

func newGenerateRuntimeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "runtime",
		Short: "Generate Constitution.runtime.md from the unified Constitution.md",
		RunE: func(cmd *cobra.Command, _ []string) error {
			root := paths.AIRoot()
			n, err := runGenerateRuntimeForRoot(root)
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Written: %s (%d bytes)\n", filepath.Join(root, "Constitution.runtime.md"), n)
			return nil
		},
	}
}

func runGenerateRuntimeForRoot(root string) (int, error) {
	content, err := os.ReadFile(filepath.Join(root, "Constitution.md")) //nolint:gosec
	if err != nil {
		return 0, fmt.Errorf("generate runtime: read Constitution.md: %w", err)
	}
	rc, err := constitution.ExtractRuntime(string(content))
	if err != nil {
		return 0, fmt.Errorf("generate runtime: extract: %w", err)
	}
	out := constitution.FormatRuntime(rc)
	dest := filepath.Join(root, "Constitution.runtime.md")
	if err := os.WriteFile(dest, []byte(out), 0o600); err != nil { //nolint:gosec
		return 0, fmt.Errorf("generate runtime: write: %w", err)
	}
	return len(out), nil
}
