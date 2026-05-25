// Package cmd is the cobra command tree for `ai`.
//
// The surface is defined by SPEC.md §3. Each top-level verb has its
// own file under this package. For v0.8 every verb is registered and
// prints a meaningful "not yet implemented" message that cites the
// authoritative spec section.
package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/convergent-systems-co/aiConstitution/src/cmd/ai/internal/buildinfo"

	"github.com/spf13/cobra"
)

// errNotImplementedHint is the v0.8 stub error returned by un-implemented
// commands. It cites the spec section that defines the verb's behavior.
type errNotImplementedHint struct {
	verb    string
	section string
}

func (e errNotImplementedHint) Error() string {
	return fmt.Sprintf("ai %s: not yet implemented (v0.8 scaffold). Authoritative spec: SPEC.md %s.", e.verb, e.section)
}

// stub is a convenience constructor for the v0.8 stub error.
func stub(verb, specSection string) error {
	return errNotImplementedHint{verb: verb, section: specSection}
}

// notice prints a "what would happen" trace for a stub command before
// returning the stub error. Keeps the v0.8 scaffold informative without
// pretending to do work that hasn't been implemented yet.
func notice(args ...any) {
	fmt.Fprintln(os.Stderr, append([]any{"[ai]"}, args...)...)
}

// NewRootCmd builds the root cobra command. Exposed for tests.
func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "ai",
		Short: "Personal AI Constitution — CLI, TUI, sync, restore, governance",
		Long: `ai is the Go CLI that operationalizes the four-file AI Constitution
governance system (Constitution / Common / Code / Writing).

It ships:
  - a Bubble Tea TUI wizard (ai setup, ai --tui)
  - a memory-to-amendment review loop (ai review)
  - a self-repairing doctor (ai doctor)
  - sync/restore for cross-machine portability (ai sync, ai restore)
  - atom-based persona/profile/skill distribution (ai mode, ai profile,
    ai persona, ai skills, ai atoms)
  - a cross-tool command-wrapper facade that enforces governance
    regardless of which AI tool issued the command

See SPEC.md at the repo root for the authoritative implementation
specification (currently draft v0.8).`,
		SilenceUsage:  true,
		SilenceErrors: false,
		Version:       buildinfo.Version(),
	}

	// Global flags. `ai --tui` is the documented alias for
	// `ai setup --tui` per SPEC.md §3; we honor it by rewriting argv
	// in Execute before the cobra tree sees it.
	root.PersistentFlags().Bool("tui", false, "launch the setup TUI (alias of `ai setup --tui`)")

	root.SetVersionTemplate("{{.Version}}\n")

	// Register every spec-defined verb. Each command file in this
	// package contributes a constructor; we collect them here so the
	// surface is grepable in one place.
	root.AddCommand(
		newSetupCmd(),
		newReviewCmd(),
		newDoctorCmd(),
		newSyncCmd(),
		newRestoreCmd(),
		newAmendCmd(),
		newMemoryCmd(),
		newBrandCmd(),
		newAtomsCmd(),
		newModeCmd(),
		newFocusCmd(),
		newProfileCmd(),
		newPersonaCmd(),
		newSkillsCmd(),
		newPluginsCmd(),
		newUpdateCmd(),
		newHooksCmd(),
		newSettingsCmd(),
		newIssueCmd(),
		newStatusCmd(),
		newAuditCmd(),
		newPlanCmd(),
		newBackupCmd(),
		newWorktreeCmd(),
		newCloneCmd(),
		newVersionCmd(),
		newGenerateCmd(),
		newMigrateCmd(),
		newInitIntegrateCmd(),
	)

	return root
}

// Execute is the package's run entry point, called from main().
//
// Honors the `ai --tui` alias by rewriting argv to `ai setup --tui`
// before the cobra tree resolves the command. This keeps the alias
// implementation deterministic and inspectable from the Execute
// boundary rather than tucked away in a PreRun hook.
func Execute(ctx context.Context) error {
	root := NewRootCmd()
	if rewrite, ok := rewriteTUIAlias(os.Args[1:]); ok {
		root.SetArgs(rewrite)
	}
	return root.ExecuteContext(ctx)
}

// rewriteTUIAlias detects `ai --tui` (with no subcommand) and rewrites
// the arg slice to `setup --tui`. Returns (newArgs, true) if a
// rewrite was applied, (nil, false) otherwise.
//
// Detection rule: the first non-flag arg is the subcommand. If
// --tui or -tui appears before any non-flag arg, treat it as the
// root alias and rewrite. If a subcommand comes first, leave alone.
func rewriteTUIAlias(args []string) ([]string, bool) {
	for _, a := range args {
		if a == "--" {
			return nil, false
		}
		if len(a) > 0 && a[0] != '-' {
			// First positional is a subcommand; not the root alias.
			return nil, false
		}
		if a == "--tui" || a == "-tui" {
			// Filter out --tui from the args and prepend "setup --tui".
			filtered := make([]string, 0, len(args)+1)
			filtered = append(filtered, "setup", "--tui")
			for _, x := range args {
				if x == "--tui" || x == "-tui" {
					continue
				}
				filtered = append(filtered, x)
			}
			return filtered, true
		}
	}
	return nil, false
}
