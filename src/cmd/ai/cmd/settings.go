package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/convergent-systems-co/aiConstitution/src/internal/config"
	"github.com/convergent-systems-co/aiConstitution/src/internal/paths"
	"github.com/spf13/cobra"
)

// newSettingsCmd implements `ai settings {get,set,edit,reset}`.
// See SPEC.md §3.11 + §13.
func newSettingsCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "settings",
		Short: "Read or write user preferences at ~/.config/aiConstitution/settings.toml",
		Long: `settings manages the canonical TOML at
~/.config/aiConstitution/settings.toml (XDG-compliant on Linux; same
path on macOS for cross-platform predictability).

Precedence (highest first): environment variable → settings.toml →
shipped defaults compiled into the binary.

See SPEC.md §3.11 + §13.`,
	}

	c.AddCommand(
		newSettingsGetCmd(),
		newSettingsSetCmd(),
		newSettingsEditCmd(),
		newSettingsResetCmd(),
	)
	return c
}

// ─── get ──────────────────────────────────────────────────────────────────────

// newSettingsGetCmd implements `ai settings get <key>`.
// Key uses dot notation: "review.cadenceDays" → [review] table, cadenceDays key.
func newSettingsGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <key>",
		Short: "Read a setting",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			key := args[0]

			settingsPath := paths.SettingsTOML()
			data, err := os.ReadFile(settingsPath) //nolint:gosec // G304: user config path
			if err != nil {
				if os.IsNotExist(err) {
					return fmt.Errorf("settings get: settings.toml not found at %s", settingsPath)
				}
				return fmt.Errorf("settings get: read settings.toml: %w", err)
			}

			var raw map[string]any
			if _, err := toml.Decode(string(data), &raw); err != nil {
				return fmt.Errorf("settings get: parse settings.toml: %w", err)
			}

			val, err := navigateDotPath(raw, key)
			if err != nil {
				return fmt.Errorf("settings get: key %q not found", key)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "%v\n", val) //nolint:errcheck
			return nil
		},
	}
}

// navigateDotPath walks a dot-notation key path into a map[string]any tree.
// Returns the leaf value or an error when any segment is absent.
func navigateDotPath(m map[string]any, dotKey string) (any, error) {
	segments := strings.SplitN(dotKey, ".", 2)
	head := segments[0]

	val, ok := m[head]
	if !ok {
		return nil, fmt.Errorf("key segment %q not found", head)
	}

	if len(segments) == 1 {
		// Leaf reached.
		return val, nil
	}

	// Descend into sub-table.
	sub, ok := val.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("key segment %q is not a table", head)
	}
	return navigateDotPath(sub, segments[1])
}

// ─── set ──────────────────────────────────────────────────────────────────────

// newSettingsSetCmd implements `ai settings set <key>=<value>`.
// Parses key on first `=`; value may contain `=` characters.
func newSettingsSetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "set <key>=<value>",
		Short: "Write a setting (validated)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			_ = cmd

			arg := args[0]
			eqIdx := strings.Index(arg, "=")
			if eqIdx < 0 {
				return fmt.Errorf("settings set: argument must be in <key>=<value> form; got %q", arg)
			}
			key := arg[:eqIdx]
			rawVal := arg[eqIdx+1:]

			if key == "" {
				return fmt.Errorf("settings set: key must not be empty")
			}

			// Load existing file or start with empty map.
			settingsPath := paths.SettingsTOML()
			raw := make(map[string]any)
			existingData, err := os.ReadFile(settingsPath) //nolint:gosec // G304
			if err == nil {
				if _, parseErr := toml.Decode(string(existingData), &raw); parseErr != nil {
					return fmt.Errorf("settings set: parse existing settings.toml: %w", parseErr)
				}
			} else if !os.IsNotExist(err) {
				return fmt.Errorf("settings set: read settings.toml: %w", err)
			}

			// Parse the value to the most-specific scalar type that fits.
			var typedVal any
			if b, err := strconv.ParseBool(rawVal); err == nil {
				typedVal = b
			} else if i, err := strconv.ParseInt(rawVal, 10, 64); err == nil {
				typedVal = i
			} else {
				typedVal = rawVal
			}

			// Set the value at the dot-path, creating sub-tables as needed.
			if err := setDotPath(raw, key, typedVal); err != nil {
				return fmt.Errorf("settings set: %w", err)
			}

			// Write back atomically.
			if err := writeSettingsMap(raw, settingsPath); err != nil {
				return fmt.Errorf("settings set: write settings.toml: %w", err)
			}

			return nil
		},
	}
}

// setDotPath navigates/creates intermediate tables in m and sets the leaf
// value at the final segment of the dot-notation key.
func setDotPath(m map[string]any, dotKey string, val any) error {
	segments := strings.SplitN(dotKey, ".", 2)
	head := segments[0]

	if head == "" {
		return fmt.Errorf("key segment must not be empty")
	}

	if len(segments) == 1 {
		m[head] = val
		return nil
	}

	// Descend or create sub-table.
	existing, exists := m[head]
	if !exists {
		sub := make(map[string]any)
		m[head] = sub
		return setDotPath(sub, segments[1], val)
	}
	sub, ok := existing.(map[string]any)
	if !ok {
		return fmt.Errorf("key segment %q exists but is not a table", head)
	}
	return setDotPath(sub, segments[1], val)
}

// writeSettingsMap encodes m as TOML and writes it atomically to path.
func writeSettingsMap(m map[string]any, path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	// Atomic write: temp file in same directory, then rename.
	tmp, err := os.CreateTemp(filepath.Dir(path), ".settings-*.toml.tmp")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpName := tmp.Name()
	defer func() {
		if _, statErr := os.Stat(tmpName); statErr == nil {
			os.Remove(tmpName) //nolint:errcheck
		}
	}()

	enc := toml.NewEncoder(tmp)
	if err := enc.Encode(m); err != nil {
		tmp.Close() //nolint:errcheck
		return fmt.Errorf("encode TOML: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}
	return os.Rename(tmpName, path)
}

// ─── edit ─────────────────────────────────────────────────────────────────────

// newSettingsEditCmd implements `ai settings edit`.
// Opens settings.toml in $EDITOR (fallback: vi). Creates the file from
// defaults if it doesn't exist. Validates TOML after editor exits.
func newSettingsEditCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "edit",
		Short: "Open settings.toml in $EDITOR (validates on save)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			settingsPath := paths.SettingsTOML()

			// Create the file from defaults if it doesn't exist.
			if _, err := os.Stat(settingsPath); os.IsNotExist(err) {
				if err := writeDefaultSettings(settingsPath); err != nil {
					return fmt.Errorf("settings edit: create default settings.toml: %w", err)
				}
			}

			editor := os.Getenv("EDITOR")
			if editor == "" {
				editor = "vi"
			}

			// Open editor; blocks until it exits.
			if err := execEditor(editor, settingsPath); err != nil {
				return fmt.Errorf("settings edit: editor exited with error: %w", err)
			}

			// Validate the result is parseable TOML.
			data, err := os.ReadFile(settingsPath) //nolint:gosec // G304
			if err != nil {
				return fmt.Errorf("settings edit: read settings.toml after edit: %w", err)
			}
			var check map[string]any
			if _, err := toml.Decode(string(data), &check); err != nil {
				return fmt.Errorf("settings edit: settings.toml is not valid TOML after edit: %w\nRe-edit with: ai settings edit", err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "settings.toml saved and validated.\n") //nolint:errcheck
			return nil
		},
	}
}

// ─── reset ────────────────────────────────────────────────────────────────────

// newSettingsResetCmd implements `ai settings reset [--accept-defaults]`.
// Without the flag: prints what would change and errors asking for confirmation.
// With the flag: writes defaults immediately.
func newSettingsResetCmd() *cobra.Command {
	var acceptDefaults bool

	r := &cobra.Command{
		Use:   "reset",
		Short: "Restore defaults",
		RunE: func(cmd *cobra.Command, _ []string) error {
			settingsPath := paths.SettingsTOML()

			if !acceptDefaults {
				// Non-interactive: show what would change and exit with guidance.
				defaults := config.Defaults()
				fmt.Fprintln(cmd.OutOrStdout(), "Would write default settings.toml:") //nolint:errcheck
				printDefaultsSummary(cmd, defaults)
				return fmt.Errorf("settings reset: non-interactive context — pass --accept-defaults to write defaults")
			}

			if err := writeDefaultSettings(settingsPath); err != nil {
				return fmt.Errorf("settings reset: %w", err)
			}

			fmt.Fprintln(cmd.OutOrStdout(), "settings.toml reset to defaults.") //nolint:errcheck
			return nil
		},
	}

	r.Flags().BoolVar(&acceptDefaults, "accept-defaults", false, "non-interactive accept of the canonical defaults")
	return r
}

// ─── helpers ──────────────────────────────────────────────────────────────────

// writeDefaultSettings encodes config.Defaults() to TOML and writes it
// atomically to settingsPath. Creates parent directories as needed.
func writeDefaultSettings(settingsPath string) error {
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0o750); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	defaults := config.Defaults()

	tmp, err := os.CreateTemp(filepath.Dir(settingsPath), ".settings-*.toml.tmp")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpName := tmp.Name()
	defer func() {
		if _, statErr := os.Stat(tmpName); statErr == nil {
			os.Remove(tmpName) //nolint:errcheck
		}
	}()

	enc := toml.NewEncoder(tmp)
	if err := enc.Encode(defaults); err != nil {
		tmp.Close() //nolint:errcheck
		return fmt.Errorf("encode defaults: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}
	return os.Rename(tmpName, settingsPath)
}

// printDefaultsSummary prints a human-readable summary of the default
// settings to cmd's stdout.
func printDefaultsSummary(cmd *cobra.Command, defaults config.Settings) {
	w := bufio.NewWriter(cmd.OutOrStdout())
	fmt.Fprintf(w, "  schemaVersion            = %q\n", defaults.SchemaVersion)
	fmt.Fprintf(w, "  review.cadenceDays       = %d\n", defaults.Review.CadenceDays)
	fmt.Fprintf(w, "  upstream.shareNewHooks   = %v\n", defaults.Upstream.ShareNewHooks)
	fmt.Fprintf(w, "  focus.defaultMode        = %q\n", defaults.Focus.DefaultMode)
	fmt.Fprintf(w, "  sync.includeSettingsFile = %v\n", defaults.Sync.IncludeSettingsFile)
	w.Flush() //nolint:errcheck
}
