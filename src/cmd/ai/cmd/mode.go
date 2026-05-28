package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/convergent-systems-co/aiConstitution/src/internal/config"
	"github.com/convergent-systems-co/aiConstitution/src/internal/paths"
	"github.com/convergent-systems-co/aiConstitution/src/internal/persona"
	"github.com/spf13/cobra"
)

// newModeCmd implements `ai mode {current,list,clear,show,share,pm}` and
// `ai mode <name>`. See SPEC.md §3.7 + §7.
func newModeCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "mode [name]",
		Short: "Activate a persona or profile (additive; not exclusive)",
		Long: `mode loads a persona or profile on top of the always-loaded
four-file constitution. Personas are additive emphasis, not
replacements — see SPEC.md §7 for the rationale.

Resolution order (SPEC.md §7.8.5):
  1. ~/.config/aiConstitution/profile-drafts/<name>.toml
  2. ~/.ai/governance/profiles/<name>.toml
  3. profile-atoms.com/<name>/latest
  4. ~/.config/aiConstitution/persona-drafts/<name>.md
  5. persona-atoms.com/<name>/latest

Use --persona or --profile for unambiguous selection.

Subcommands:
  current   Print the active mode.
  list      Enumerate available profiles and personas.
  clear     Deactivate the current mode (return to four-file only).
  show      Show resolved content for a name.
  share     File a draft as an upstream atom PR.
  pm        Shortcut: activate PM mode (plan-first discipline).

See SPEC.md §3.7 + §7.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			slug := args[0]

			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("mode: load settings: %w", err)
			}

			// Add the requested persona to the active set (additive).
			active := cfg.Personas.Default
			for _, p := range active {
				if p == slug {
					_, _ = fmt.Fprintf(cmd.OutOrStdout(), "persona %q already active\n", slug)
					return nil
				}
			}
			active = append(active, slug)

			claudeMD := paths.ClaudeMD()
			if err := persona.RewriteBlock(claudeMD, active, paths.AIRoot()); err != nil {
				return fmt.Errorf("mode: rewrite CLAUDE.md: %w", err)
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "activated persona %q — CLAUDE.md updated\nActive: %v\n", slug, active)
			return nil
		},
	}

	// current
	c.AddCommand(&cobra.Command{
		Use:   "current",
		Short: "Print the active mode",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runModeCurrent(cmd)
		},
	})

	// list
	c.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "Enumerate profiles + personas, grouped by source",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runModeList(cmd)
		},
	})

	// clear
	c.AddCommand(&cobra.Command{
		Use:   "clear",
		Short: "Deactivate the current mode (return to defaults)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("mode clear: load settings: %w", err)
			}
			claudeMD := paths.ClaudeMD()
			if err := persona.RewriteBlock(claudeMD, cfg.Personas.Default, paths.AIRoot()); err != nil {
				return fmt.Errorf("mode clear: rewrite CLAUDE.md: %w", err)
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "cleared — reverted to defaults: %v\n", cfg.Personas.Default)
			return nil
		},
	})

	// show
	c.AddCommand(&cobra.Command{
		Use:   "show <name>",
		Short: "Show resolved persona/profile content + metadata",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runModeShow(cmd, args[0])
		},
	})

	// share
	c.AddCommand(&cobra.Command{
		Use:   "share <name>",
		Short: "File a draft as an upstream atom PR",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			filePath := filepath.Join(paths.AIRoot(), "mode", args[0])
			return runShareUpstream(args[0], filePath, "convergent-systems-co/mode-atoms", "", cmd.OutOrStdout())
		},
	})

	// pm — named activation shortcut for PM mode (#219)
	c.AddCommand(newPmSubCmd())

	return c
}

// runModeCurrent reads ~/.config/aiConstitution/mode.json and prints the active
// mode slug, or "(none)" if the file is absent or the mode field is empty.
func runModeCurrent(cmd *cobra.Command) error {
	modeFile := paths.ModeJSON()
	data, err := os.ReadFile(modeFile) //nolint:gosec
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			fmt.Fprintln(cmd.OutOrStdout(), "(none)")
			return nil
		}
		return fmt.Errorf("mode current: read %s: %w", modeFile, err)
	}

	var m pmModeJSON
	if jsonErr := json.Unmarshal(data, &m); jsonErr != nil {
		return fmt.Errorf("mode current: parse %s: %w", modeFile, jsonErr)
	}
	if m.Mode == "" {
		fmt.Fprintln(cmd.OutOrStdout(), "(none)")
		return nil
	}
	fmt.Fprintln(cmd.OutOrStdout(), m.Mode)
	return nil
}

// modeRow is a single row in the `mode list` table.
type modeRow struct {
	name   string
	kind   string // "persona" or "profile"
	source string // "local"
}

// runModeList walks ~/.ai/personas/ (personas) and ~/.config/aiConstitution/profiles/
// (profiles) and prints a NAME | TYPE | SOURCE table.
func runModeList(cmd *cobra.Command) error {
	var rows []modeRow

	// Walk personas.
	personasRoot := filepath.Join(paths.AIRoot(), "personas")
	if entries, err := os.ReadDir(personasRoot); err == nil {
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
				continue
			}
			name := strings.TrimSuffix(e.Name(), ".yaml")
			rows = append(rows, modeRow{name: name, kind: "persona", source: "local"})
		}
	}

	// Walk profiles.
	profilesRoot := filepath.Join(paths.ConfigDir(), "profiles")
	if entries, err := os.ReadDir(profilesRoot); err == nil {
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
				continue
			}
			name := strings.TrimSuffix(e.Name(), ".yaml")
			rows = append(rows, modeRow{name: name, kind: "profile", source: "local"})
		}
	}

	if len(rows) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "(no modes available)")
		return nil
	}

	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tTYPE\tSOURCE")
	for _, r := range rows {
		fmt.Fprintf(w, "%s\t%s\t%s\n", r.name, r.kind, r.source)
	}
	return w.Flush()
}

// runModeShow searches for a persona or profile named `name` and prints its
// file content. Persona lookup (~/.ai/personas/<name>.yaml) is tried first,
// then profile lookup (~/.config/aiConstitution/profiles/<name>.yaml).
func runModeShow(cmd *cobra.Command, name string) error {
	candidates := []string{
		filepath.Join(paths.AIRoot(), "personas", name+".yaml"),
		filepath.Join(paths.ConfigDir(), "profiles", name+".yaml"),
	}
	for _, path := range candidates {
		data, err := os.ReadFile(path) //nolint:gosec
		if err == nil {
			fmt.Fprint(cmd.OutOrStdout(), string(data))
			return nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("mode show: read %s: %w", path, err)
		}
	}
	return fmt.Errorf("mode %q not found", name)
}

// newFocusCmd is the documented alias of `ai mode` (SPEC.md §3.7).
func newFocusCmd() *cobra.Command {
	c := newModeCmd()
	c.Use = "focus [name]"
	c.Short = "Alias of `ai mode`"
	c.Aliases = nil
	return c
}

// pmModeJSON is the on-disk shape written by pm-mode.
type pmModeJSON struct {
	Mode        string `json:"mode"`
	ActivatedAt string `json:"activatedAt"`
	Discipline  string `json:"discipline"`
}

// writePmModeJSON encodes and writes the PM mode state to mode.json.
func writePmModeJSON(cmd *cobra.Command) error {
	modeFile := paths.ModeJSON()
	if err := os.MkdirAll(filepath.Dir(modeFile), 0o750); err != nil {
		return fmt.Errorf("pm-mode: mkdir: %w", err)
	}

	payload := pmModeJSON{
		Mode:        "pm",
		ActivatedAt: time.Now().UTC().Format(time.RFC3339),
		Discipline:  "plan-first",
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("pm-mode: marshal: %w", err)
	}
	if err := os.WriteFile(modeFile, data, 0o644); err != nil {
		return fmt.Errorf("pm-mode: write %s: %w", modeFile, err)
	}

	fmt.Fprintln(cmd.OutOrStdout(), "PM mode activated. plan-first discipline is active.")
	return nil
}

// newPmSubCmd implements `ai mode pm` — a named shortcut for PM mode.
func newPmSubCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "pm",
		Short: "Activate PM mode (plan-first discipline)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return writePmModeJSON(cmd)
		},
	}
}

// newPmModeCmd implements the top-level `ai pm-mode` shortcut (#219).
// This is registered at root level for ergonomics; internally it delegates
// to the same writer as `ai mode pm`.
func newPmModeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "pm-mode",
		Short: "Activate PM mode (plan-first discipline) — shortcut for `ai mode pm`",
		Long: `pm-mode writes ~/.config/aiConstitution/mode.json with:
  {"mode":"pm","activatedAt":"<UTC>","discipline":"plan-first"}

Use `+"`"+`ai mode current`+"`"+` to confirm the active mode.

See SPEC.md §3.7 + Common.md §pm-mode.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return writePmModeJSON(cmd)
		},
	}
}
