// Package constitution loads and validates the four AI Constitution files
// from ~/.ai/ (or a supplied root directory in tests).
package constitution

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// FileNames is the canonical list of the four required constitution files,
// in loading order (Constitution first per spec §3.1).
var FileNames = []string{
	"Constitution.md",
	"Common.md",
	"Code.md",
	"Writing.md",
}

// Files holds the in-memory content of all four constitution files
// loaded from a single root directory.
type Files struct {
	Constitution string
	Common       string
	Code         string
	Writing      string
	// Local is the optional Constitution.local.md override content.
	// Empty string means no local override is present.
	Local string
}

// Finding describes a single validation issue found in a loaded file set.
type Finding struct {
	File    string
	Message string
}

func (f Finding) Error() string {
	return fmt.Sprintf("%s: %s", f.File, f.Message)
}

// Load reads the four canonical files from root (typically ~/.ai/).
// Returns an error if any required file is missing.
// Constitution.local.md is loaded if present; its absence is not an error.
func Load(root string) (Files, error) {
	var f Files
	mapping := []struct {
		name string
		dest *string
	}{
		{"Constitution.md", &f.Constitution},
		{"Common.md", &f.Common},
		{"Code.md", &f.Code},
		{"Writing.md", &f.Writing},
	}

	for _, m := range mapping {
		data, err := os.ReadFile(filepath.Join(root, m.name))
		if err != nil {
			return Files{}, fmt.Errorf("constitution: required file %q missing from %q: %w", m.name, root, err)
		}
		*m.dest = string(data)
	}

	// Constitution.local.md is optional.
	localData, err := os.ReadFile(filepath.Join(root, "Constitution.local.md"))
	if err == nil {
		f.Local = string(localData)
	}

	return f, nil
}

// Validate returns structural findings for the loaded file set.
// An empty slice means all files are structurally valid.
func (f Files) Validate() []Finding {
	var findings []Finding
	checks := []struct {
		name    string
		content string
	}{
		{"Constitution.md", f.Constitution},
		{"Common.md", f.Common},
		{"Code.md", f.Code},
		{"Writing.md", f.Writing},
	}
	for _, c := range checks {
		if strings.TrimSpace(c.content) == "" {
			findings = append(findings, Finding{File: c.name, Message: "file is empty"})
		}
	}
	return findings
}

// FileStatus returns a map of file name → present (true/false) for
// all four required files plus the optional local override.
// Used by ai doctor and ai status without performing a full Load.
func FileStatus(root string) map[string]bool {
	status := make(map[string]bool, 5)
	for _, name := range FileNames {
		_, err := os.Stat(filepath.Join(root, name))
		status[name] = err == nil
	}
	_, err := os.Stat(filepath.Join(root, "Constitution.local.md"))
	status["Constitution.local.md"] = err == nil
	return status
}
