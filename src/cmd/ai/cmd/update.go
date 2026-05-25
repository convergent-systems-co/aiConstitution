package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

// newUpdateCmd implements `ai update`. See SPEC.md §3.9 + §8.
func newUpdateCmd() *cobra.Command {
	var migrate bool
	var skipMigrate bool
	var blocking bool
	var nonInteractive bool

	c := &cobra.Command{
		Use:   "update",
		Short: "Update the binary + reconcile new hooks/skills/personas/questions",
		Long: `update runs the upstream reconciliation. The base action is
` + "`" + `git pull --ff-only` + "`" + ` on ~/.ai/ plus ` + "`" + `go build` + "`" + ` of the binary.

On any subsequent ` + "`" + `ai` + "`" + ` invocation where governance/last-seen-version
differs from the binary version, the migration prompt fires (unless
settings.update.autoMigratePrompt = false). --migrate runs it
immediately; --skip-migrate suppresses it once.

--migrate detects the current layout (v1 vs v2) and either reports that
migration is already complete or runs the v1→v2 migration pipeline.
Use --non-interactive to skip the confirmation prompt.

See SPEC.md §3.9 + §8.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if migrate {
				return runMigrate(cmd, nonInteractive)
			}
			notice("update:", "migrate=", migrate, "skip-migrate=", skipMigrate, "blocking=", blocking)
			return stub("update", "§3.9 + §8")
		},
	}

	c.Flags().BoolVar(&migrate, "migrate", false, "run reconciliation now")
	c.Flags().BoolVar(&skipMigrate, "skip-migrate", false, "one-shot bypass of the migration prompt")
	c.Flags().BoolVar(&blocking, "blocking", false, "opt back into the original blocking behavior of the migration prompt")
	c.Flags().BoolVar(&nonInteractive, "non-interactive", false, "skip confirmation prompt and proceed automatically")

	return c
}

// runMigrate detects the current layout and runs the v1→v2 migration pipeline
// or reports that migration is already complete.
//
// v2 layout is signalled by the presence of Constitution.md at $AI_ROOT.
// v1 layout is everything else (single monolithic file, or no governance files).
//
// Coder C OWNS this function (#199).
func runMigrate(cmd *cobra.Command, nonInteractive bool) error {
	aiRoot := aiRoot()
	constPath := filepath.Join(aiRoot, "Constitution.md")

	if isV2Layout(constPath) {
		fmt.Fprintln(cmd.OutOrStdout(), "Already v2 — no migration needed.")
		return nil
	}

	// v1 layout detected.
	fmt.Fprintln(cmd.OutOrStdout(), "v1 layout detected.")
	fmt.Fprintln(cmd.OutOrStdout(), "Migration would:")
	fmt.Fprintln(cmd.OutOrStdout(), "  1. Flatten monolithic governance file into four-file layout")
	fmt.Fprintln(cmd.OutOrStdout(), "     (Constitution.md, Common.md, Code.md, Writing.md)")
	fmt.Fprintln(cmd.OutOrStdout(), "  2. Add behavioral overlays (per-tool instruction files)")
	fmt.Fprintln(cmd.OutOrStdout(), "  3. Generate runtime injection artifacts")

	if nonInteractive {
		return executeMigrationSteps(cmd, aiRoot)
	}

	// Interactive prompt.
	fmt.Fprint(cmd.OutOrStdout(), "Migrate to unified v2? (yes/no): ")
	scanner := bufio.NewScanner(os.Stdin)
	if !scanner.Scan() {
		return fmt.Errorf("update --migrate: failed to read response")
	}
	answer := strings.TrimSpace(strings.ToLower(scanner.Text()))
	if answer != "yes" && answer != "y" {
		fmt.Fprintln(cmd.OutOrStdout(), "Migration cancelled.")
		return nil
	}

	return executeMigrationSteps(cmd, aiRoot)
}

// isV2Layout returns true when the v2 marker file (Constitution.md) exists at
// the given path. The presence of this file is the canonical signal that the
// four-file layout is already in place.
func isV2Layout(constitutionPath string) bool {
	_, err := os.Stat(constitutionPath)
	return err == nil
}

// executeMigrationSteps runs the v1→v2 migration pipeline. The actual
// migration logic (runMigrateFlatten, runMigrateAddBehavioral,
// runMigrateGenerateRuntime) is deferred to v0.9; this stub records the
// intent and prints what would happen.
func executeMigrationSteps(cmd *cobra.Command, aiRoot string) error {
	// v0.8: pipeline is stubbed. Print the steps that would run.
	fmt.Fprintln(cmd.OutOrStdout(), "Running migration pipeline:")
	fmt.Fprintln(cmd.OutOrStdout(), "  [1/3] runMigrateFlatten — not yet implemented (v0.9)")
	fmt.Fprintln(cmd.OutOrStdout(), "  [2/3] runMigrateAddBehavioral — not yet implemented (v0.9)")
	fmt.Fprintln(cmd.OutOrStdout(), "  [3/3] runMigrateGenerateRuntime — not yet implemented (v0.9)")
	fmt.Fprintf(cmd.OutOrStdout(), "Migration target: %s\n", aiRoot)
	fmt.Fprintln(cmd.OutOrStdout(), "Migration steps stubbed in v0.8 — run again after v0.9 ships.")
	return nil
}
