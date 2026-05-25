// Package plugins manages the on-disk layout and state for `ai plugins`.
//
// Each plugin lives at ~/.ai/plugins/<name>/ with a manifest.yaml.
// Enabled state is persisted at ~/.config/aiConstitution/plugins.json.
//
// Per SPEC.md §11.
package plugins

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// PluginManifest is the parsed content of ~/.ai/plugins/<name>/manifest.yaml.
//
// Required fields: Name, Version.
// Optional fields: Description, Source, Skills.
type PluginManifest struct {
	// Name is the unique identifier for the plugin. Matches the
	// directory name under ~/.ai/plugins/.
	Name string `yaml:"name"`

	// Version is the semantic version string (e.g. "1.0.0").
	Version string `yaml:"version"`

	// Description is a human-readable summary of what the plugin does.
	Description string `yaml:"description,omitempty"`

	// Source is the URL (https://*.tar.gz) or local path used to
	// fetch this plugin. Used by `ai plugins update`.
	Source string `yaml:"source,omitempty"`

	// Skills is the list of skill names that this plugin contributes.
	Skills []string `yaml:"skills,omitempty"`
}

// ParseManifest reads and validates a manifest.yaml file.
// Returns a non-nil error when:
//   - the file cannot be read,
//   - the YAML is malformed, or
//   - the required fields (name, version) are absent.
func ParseManifest(path string) (*PluginManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("plugins: read manifest %q: %w", path, err)
	}

	var m PluginManifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("plugins: parse manifest %q: %w", path, err)
	}

	if m.Name == "" {
		return nil, fmt.Errorf("plugins: manifest %q missing required field: name", path)
	}
	if m.Version == "" {
		return nil, fmt.Errorf("plugins: manifest %q missing required field: version", path)
	}

	return &m, nil
}
