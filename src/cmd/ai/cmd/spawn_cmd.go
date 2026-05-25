package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/convergent-systems-co/aiConstitution/src/internal/paths"
	"github.com/spf13/cobra"
)

// spawnJSON is the on-disk shape of state/spawn.json.
type spawnJSON struct {
	Persona    string `json:"persona"`
	SpawnedAt  string `json:"spawnedAt"`
	ParentMode string `json:"parentMode"`
}

// readCurrentMode reads the "mode" field from mode.json.
// Returns empty string if mode.json does not exist or has no mode field.
func readCurrentMode() string {
	data, err := os.ReadFile(paths.ModeJSON())
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return ""
		}
		return ""
	}
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		return ""
	}
	if v, ok := m["mode"].(string); ok {
		return v
	}
	return ""
}

// newSpawnCmd implements `ai spawn <name>` (#220).
// Resolves the named persona from ~/.ai/personas/<name>.yaml,
// writes state/spawn.json, and prints a markdown activation block.
func newSpawnCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "spawn <name>",
		Short: "Spawn a persona agent",
		Long: `spawn resolves a persona by name from ~/.ai/personas/ and
activates it by writing state/spawn.json. It prints a markdown
activation block containing the persona file content so the
spawned agent can load its context.

Error when persona not found.

See SPEC.md §3 (v0.8 spawn surface).`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			// Resolve persona file.
			path := personaPath(name)
			data, err := os.ReadFile(path)
			if err != nil {
				if errors.Is(err, os.ErrNotExist) {
					return fmt.Errorf("persona %q not found at %s", name, path)
				}
				return fmt.Errorf("spawn: read persona: %w", err)
			}

			// Read current mode (empty string if none active).
			parentMode := readCurrentMode()

			// Write state/spawn.json.
			stateDir := filepath.Join(paths.ConfigDir(), "state")
			if err := os.MkdirAll(stateDir, 0o750); err != nil {
				return fmt.Errorf("spawn: mkdir state: %w", err)
			}
			spawnFile := filepath.Join(stateDir, "spawn.json")
			payload := spawnJSON{
				Persona:    name,
				SpawnedAt:  time.Now().UTC().Format(time.RFC3339),
				ParentMode: parentMode,
			}
			spawnData, err := json.Marshal(payload)
			if err != nil {
				return fmt.Errorf("spawn: marshal: %w", err)
			}
			if err := os.WriteFile(spawnFile, spawnData, 0o644); err != nil {
				return fmt.Errorf("spawn: write %s: %w", spawnFile, err)
			}

			// Print markdown activation block.
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "# Spawned: %s\n\n", name)
			fmt.Fprintf(out, "**Persona:** `%s`  \n", name)
			if parentMode != "" {
				fmt.Fprintf(out, "**Parent mode:** `%s`  \n", parentMode)
			}
			fmt.Fprintf(out, "**Spawned at:** `%s`\n\n", payload.SpawnedAt)
			fmt.Fprintln(out, "## Persona context")
			fmt.Fprintln(out, "")
			fmt.Fprintln(out, "```yaml")
			fmt.Fprint(out, string(data))
			fmt.Fprintln(out, "```")

			return nil
		},
	}
}
