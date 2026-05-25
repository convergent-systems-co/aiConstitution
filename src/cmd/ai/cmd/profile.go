package cmd

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/convergent-systems-co/aiConstitution/src/internal/paths"
	"github.com/spf13/cobra"
)

// newProfileCmd implements `ai profile {list,show,new,edit,remove,share}`.
// Issues #215 (list/show) and #216 (new/edit/remove). See SPEC.md §3.8 + §7.8.
func newProfileCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "profile",
		Short: "Manage profiles (compositions of atomic personas)",
		Long: `profile manages the YAML recipes at
~/.config/aiConstitution/profiles/ that declare persona compositions.

Subcommands:
  list             List all profiles (name + description)
  show <name>      Print profile file content
  new <name>       Create a new profile with frontmatter stub
  edit <name>      Open profile in $EDITOR (prints path when EDITOR unset)
  remove <name>    Delete a profile

See SPEC.md §3.8 + §7.8.`,
	}

	c.AddCommand(
		newProfileListCmd(),
		newProfileShowCmd(),
		newProfileNewCmd(),
		newProfileEditCmd(),
		newProfileRemoveCmd(),
		// share remains stub
		&cobra.Command{
			Use: "share <name>", Short: "File the profile upstream as an atom PR", Args: cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				notice("profile share:", args[0])
				return stub("profile share", "§7.9.3")
			},
		},
	)
	return c
}

// profilesDir returns the canonical profiles directory under ConfigDir().
func profilesDir() string {
	return filepath.Join(paths.ConfigDir(), "profiles")
}

// profilePath builds the full path for a profile by name (no extension needed).
func profilePath(name string) string {
	return filepath.Join(profilesDir(), name+".yaml")
}

// parseYAMLField does a single-pass scan of YAML content and returns
// the value for the first occurrence of `key:`. This avoids a yaml
// dependency for simple key: value frontmatter.
func parseYAMLField(content []byte, key string) string {
	prefix := key + ":"
	scanner := bufio.NewScanner(bytes.NewReader(content))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, prefix) {
			value := strings.TrimSpace(line[len(prefix):])
			// Strip surrounding quotes.
			value = strings.Trim(value, `"'`)
			return value
		}
	}
	return ""
}

// newProfileListCmd implements `ai profile list`.
func newProfileListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List profiles",
		RunE: func(cmd *cobra.Command, _ []string) error {
			dir := profilesDir()
			entries, err := os.ReadDir(dir)
			if err != nil {
				if errors.Is(err, os.ErrNotExist) {
					fmt.Fprintln(cmd.OutOrStdout(), "(no profiles)")
					return nil
				}
				return fmt.Errorf("profile list: read %s: %w", dir, err)
			}

			// Collect yaml files.
			type profileRow struct {
				name string
				desc string
			}
			var rows []profileRow
			for _, e := range entries {
				if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
					continue
				}
				data, readErr := os.ReadFile(filepath.Join(dir, e.Name()))
				if readErr != nil {
					continue
				}
				name := strings.TrimSuffix(e.Name(), ".yaml")
				desc := parseYAMLField(data, "description")
				rows = append(rows, profileRow{name: name, desc: desc})
			}

			if len(rows) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "(no profiles)")
				return nil
			}

			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "NAME\tDESCRIPTION")
			for _, r := range rows {
				fmt.Fprintf(w, "%s\t%s\n", r.name, r.desc)
			}
			return w.Flush()
		},
	}
}

// newProfileShowCmd implements `ai profile show <name>`.
func newProfileShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show <name>",
		Short: "Show profile YAML content",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := profilePath(args[0])
			data, err := os.ReadFile(path)
			if err != nil {
				if errors.Is(err, os.ErrNotExist) {
					return fmt.Errorf("profile %q not found", args[0])
				}
				return fmt.Errorf("profile show: %w", err)
			}
			fmt.Fprint(cmd.OutOrStdout(), string(data))
			return nil
		},
	}
}

// newProfileNewCmd implements `ai profile new <name>`.
func newProfileNewCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "new <name>",
		Short: "Create a new profile",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			path := profilePath(name)

			if _, err := os.Stat(path); err == nil {
				return fmt.Errorf("profile %q already exists at %s", name, path)
			}

			if err := os.MkdirAll(profilesDir(), 0o750); err != nil {
				return fmt.Errorf("profile new: mkdir: %w", err)
			}

			content := fmt.Sprintf("name: %s\ndescription: \"\"\ndomains: []\n", name)
			if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
				return fmt.Errorf("profile new: write: %w", err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Created profile %q at %s\n", name, path)
			return nil
		},
	}
}

// newProfileEditCmd implements `ai profile edit <name>`.
// When $EDITOR is unset, prints the path. Otherwise opens the file.
func newProfileEditCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "edit <name>",
		Short: "Open profile in $EDITOR (prints path when EDITOR unset)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			path := profilePath(name)

			if _, err := os.Stat(path); err != nil {
				if errors.Is(err, os.ErrNotExist) {
					return fmt.Errorf("profile %q not found at %s", name, path)
				}
				return fmt.Errorf("profile edit: stat: %w", err)
			}

			editor := os.Getenv("EDITOR")
			if editor == "" {
				fmt.Fprintln(cmd.OutOrStdout(), path)
				return nil
			}

			// gosec G204: editor comes from the user's own $EDITOR env var;
			// this is the canonical "open in editor" pattern.
			e := exec.Command(editor, path) //nolint:gosec // G204: $EDITOR is user-controlled by design
			e.Stdin = os.Stdin
			e.Stdout = os.Stdout
			e.Stderr = os.Stderr
			if err := e.Run(); err != nil {
				return fmt.Errorf("profile edit: %s: %w", editor, err)
			}
			return nil
		},
	}
}

// newProfileRemoveCmd implements `ai profile remove <name>`.
func newProfileRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove <name>",
		Short: "Remove a profile",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			path := profilePath(name)

			if _, err := os.Stat(path); err != nil {
				if errors.Is(err, os.ErrNotExist) {
					return fmt.Errorf("profile %q not found at %s", name, path)
				}
				return fmt.Errorf("profile remove: stat: %w", err)
			}

			if err := os.Remove(path); err != nil {
				return fmt.Errorf("profile remove: %w", err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Removed profile %q\n", name)
			return nil
		},
	}
}
