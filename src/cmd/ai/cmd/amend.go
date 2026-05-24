package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/convergent-systems-co/aiConstitution/src/internal/amend"
	"github.com/convergent-systems-co/aiConstitution/src/internal/paths"
	"github.com/spf13/cobra"
)

// newAmendCmd implements `ai amend draft <ref>` (and, in a follow-up
// commit, `ai amend apply <path>`). See SPEC.md §3.5 and §6
// (Memory → Amendment Lifecycle).
func newAmendCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "amend",
		Short: "Open or apply an amendment against a canonical file",
		Long: `amend writes a versioned change against one of the four canonical
files, bumping the file's version, appending the Changelog entry, and
recording the amendment in the audit log.

Subcommands:
  draft <file>/<section>   Create a new amendment draft and open it
                           in $EDITOR.

See SPEC.md §3.5 and §6.`,
	}

	c.AddCommand(newAmendDraftCmd())
	return c
}

func newAmendDraftCmd() *cobra.Command {
	var fromViolation string
	var rationale string

	c := &cobra.Command{
		Use:   "draft <file>/<section>",
		Short: "Create a new amendment draft and open it in $EDITOR",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			file, section, err := amend.ParseRef(args[0])
			if err != nil {
				return err
			}
			// Best-effort section probe — emit a warning to stderr when
			// the section is not found, but still write the draft so
			// the user can edit the proposed_change in $EDITOR.
			aiRoot := paths.AIRoot()
			targetPath := filepath.Join(aiRoot, file)
			if data, err := os.ReadFile(targetPath); err == nil { //nolint:gosec // G304: aiRoot + canonical filename
				if _, _, found := amend.LocateSection(string(data), section); !found {
					_, _ = fmt.Fprintf(cmd.ErrOrStderr(),
						"[ai] warning: section %q not found in %s — draft will still be created\n",
						section, targetPath)
				}
			}

			d := amend.Draft{
				File:      file,
				Section:   section,
				AuditRef:  fromViolation,
				Rationale: rationale,
				Created:   time.Now().UTC(),
			}
			plansDir := filepath.Join(paths.GovernanceDir(), "plans")
			path, err := amend.WriteDraft(d, plansDir)
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Wrote draft: %s\n", path)

			editor := os.Getenv("EDITOR")
			if editor == "" {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(),
					"$EDITOR not set; edit the file directly to fill in the proposed change.")
				return nil
			}
			ec := exec.Command(editor, path) //nolint:gosec // G204: $EDITOR is user-controlled by design
			ec.Stdin = os.Stdin
			ec.Stdout = cmd.OutOrStdout()
			ec.Stderr = cmd.ErrOrStderr()
			if err := ec.Run(); err != nil {
				return fmt.Errorf("amend: $EDITOR %s exited with error: %w", editor, err)
			}
			return nil
		},
	}
	c.Flags().StringVar(&fromViolation, "from-violation", "",
		"path to an audit/violations/*.md file that motivated this amendment")
	c.Flags().StringVar(&rationale, "rationale", "",
		"one-line rationale (used in the Changelog entry on apply)")
	return c
}
