package constitution

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// CanonicalFile is the single file that constitutes the unified end-state.
// All new code reads from or writes to this file; the derivative files
// (Common.md, Code.md, Writing.md) exist only in migration/detection paths.
const CanonicalFile = "Constitution.md"

// FileNames is the ordered list of legacy filenames used for migration
// detection and the four-file → unified conversion. Do not use FileNames
// when the goal is to read or write the unified end-state — use CanonicalFile.
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

// FileStatusV2 detects whether root has a unified Constitution.md or the legacy
// four-file layout (Constitution.md + Common.md + Code.md + Writing.md).
// The returned map includes a synthetic "v2" key: true when Constitution.md is
// present and Common.md is absent, indicating the user has migrated.
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

// Section represents one extracted persona section from Constitution.md.
// Governance sections (e.g., "## 0. Governance Rules") are excluded.
type Section struct {
	Number   int    // ordinal from the heading (## N.)
	Name     string // word before "Rules" (e.g., "Common", "Code")
	Slug     string // lowercase Name (e.g., "common", "code")
	FileName string // derivative output filename (e.g., "Common.md")
	Body     string // raw markdown content of this section
}

// sectionHeaderRe matches "## N. <Name> Rules" with optional whitespace.
var sectionHeaderRe = regexp.MustCompile(`(?m)^## (\d+)\. (\w+) Rules\s*$`)

// ParseSections extracts persona sections from Constitution.md content.
// Sections whose Name is "Governance" are excluded — they contain
// meta-rules only, not enforceable AI directives.
func ParseSections(content string) []Section {
	matches := sectionHeaderRe.FindAllStringIndex(content, -1)
	if len(matches) == 0 {
		return nil
	}

	var sections []Section
	for i, loc := range matches {
		header := content[loc[0]:loc[1]]
		sub := sectionHeaderRe.FindStringSubmatch(header)
		if sub == nil {
			continue
		}
		num, _ := strconv.Atoi(sub[1])
		name := sub[2]
		if strings.EqualFold(name, "Governance") {
			continue
		}

		bodyStart := loc[1]
		var bodyEnd int
		if i+1 < len(matches) {
			bodyEnd = matches[i+1][0]
		} else {
			bodyEnd = len(content)
		}
		body := strings.TrimSpace(content[bodyStart:bodyEnd])

		sections = append(sections, Section{
			Number:   num,
			Name:     name,
			Slug:     strings.ToLower(name),
			FileName: name + ".md",
			Body:     body,
		})
	}
	return sections
}
