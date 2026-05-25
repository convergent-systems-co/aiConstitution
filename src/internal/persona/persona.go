// Package persona resolves the active persona list and manages the
// <!-- ai:personas --> block in ~/.claude/CLAUDE.md.
package persona

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/convergent-systems-co/aiConstitution/src/internal/config"
)

const (
	blockOpen  = "<!-- ai:personas — managed by ai cli, do not edit manually -->"
	blockClose = "<!-- /ai:personas -->"
)

// projectYAML is the minimal structure of project.yaml we care about.
type projectYAML struct {
	Personas struct {
		Load []string `yaml:"load"`
	} `yaml:"personas"`
}

// Resolve returns the active persona slug list. project.yaml (at
// projectYAMLPath) overrides settings.toml defaults if the file exists
// and has a non-empty personas.load list. Missing project.yaml is not
// an error — it falls back silently to settings defaults.
func Resolve(s config.Settings, projectYAMLPath string) ([]string, error) {
	if projectYAMLPath != "" {
		data, err := os.ReadFile(projectYAMLPath) //nolint:gosec
		if err == nil {
			var p projectYAML
			if err2 := yaml.Unmarshal(data, &p); err2 == nil && len(p.Personas.Load) > 0 {
				return p.Personas.Load, nil
			}
		}
	}
	return s.Personas.Default, nil
}

// RewriteBlock rewrites the <!-- ai:personas --> block in claudeMDPath.
// personas is the ordered list of slugs (e.g., ["common", "code"]).
// aiRoot is the path used to build the @include lines (e.g., ~/.ai).
// If the block doesn't exist, it is inserted after the first @include
// line (or appended if none exists).
func RewriteBlock(claudeMDPath string, personas []string, aiRoot string) error {
	data, err := os.ReadFile(claudeMDPath) //nolint:gosec
	if err != nil {
		return fmt.Errorf("persona: read %s: %w", claudeMDPath, err)
	}

	newBlock := buildBlock(personas, aiRoot)
	content := string(data)

	blockRe := regexp.MustCompile(`(?s)` + regexp.QuoteMeta(blockOpen) + `.*?` + regexp.QuoteMeta(blockClose) + `\n?`)
	if blockRe.MatchString(content) {
		content = blockRe.ReplaceAllString(content, newBlock)
	} else {
		insertRe := regexp.MustCompile(`(?m)(^@[^\n]+\n)`)
		if loc := insertRe.FindStringIndex(content); loc != nil {
			content = content[:loc[1]] + "\n" + newBlock + content[loc[1]:]
		} else {
			content = content + "\n" + newBlock
		}
	}

	return os.WriteFile(claudeMDPath, []byte(content), 0o600) //nolint:gosec
}

// PersonaFileName maps a persona slug to its derivative filename.
// "common" → "Common.md", "code" → "Code.md", etc.
func PersonaFileName(slug string) string {
	if slug == "" {
		return ""
	}
	return strings.ToUpper(slug[:1]) + slug[1:] + ".md"
}

func buildBlock(personas []string, aiRoot string) string {
	var sb strings.Builder
	sb.WriteString(blockOpen + "\n")
	for _, slug := range personas {
		name := PersonaFileName(slug)
		sb.WriteString(fmt.Sprintf("@%s\n", filepath.Join(aiRoot, name)))
	}
	sb.WriteString(blockClose + "\n")
	return sb.String()
}
