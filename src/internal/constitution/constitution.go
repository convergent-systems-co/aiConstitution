package constitution

import (
	"fmt"
	"os"
	"path/filepath"
)

// FileNames is the list of legacy constitution filenames.
var FileNames = []string{"Constitution.md", "Common.md", "Code.md", "Writing.md"}

// UnifiedFiles holds the unified constitution content.
type UnifiedFiles struct {
	Content string // full Constitution.md
	Local   string // Constitution.local.md if present
}

// LoadUnified reads Constitution.md from root.
// Returns error if missing.
func LoadUnified(root string) (UnifiedFiles, error) {
	data, err := os.ReadFile(filepath.Join(root, "Constitution.md")) //nolint:gosec
	if err != nil {
		return UnifiedFiles{}, fmt.Errorf("constitution: Constitution.md missing from %q: %w", root, err)
	}
	uf := UnifiedFiles{Content: string(data)}
	localData, err := os.ReadFile(filepath.Join(root, "Constitution.local.md")) //nolint:gosec
	if err == nil {
		uf.Local = string(localData)
	}
	return uf, nil
}

// FileStatusV2 detects whether root has a v2 unified constitution or legacy four-file layout.
// The returned map includes a synthetic "v2" key (true when Constitution.md present and Common.md absent).
func FileStatusV2(root string) map[string]bool {
	status := make(map[string]bool)
	for _, name := range FileNames {
		_, err := os.Stat(filepath.Join(root, name))
		status[name] = err == nil
	}
	_, constitutionErr := os.Stat(filepath.Join(root, "Constitution.md"))
	_, commonErr := os.Stat(filepath.Join(root, "Common.md"))
	status["v2"] = constitutionErr == nil && commonErr != nil
	return status
}
