// Package memory manages the cross-tool memory layer at ~/.ai/memory/.
//
// Findings written here feed the review loop (ai review) and may be
// promoted to constitutional amendments via ai memory codify.
// Schema: Common.md §5.1.
package memory

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// WriteFinding writes a feedback-type memory file to <root>/feedback_<slug>.md
// and appends a pointer entry to <root>/MEMORY.md.
// slug is derived from rule: lowercased, spaces→hyphens, filesystem-safe,
// max 32 chars. The directory is created with 0o750 if absent.
// Returns the path of the created finding file.
func WriteFinding(root, rule, whatHappened, remediation string) (string, error) {
	if err := os.MkdirAll(root, 0o750); err != nil {
		return "", fmt.Errorf("memory: mkdir %q: %w", root, err)
	}

	slug := findingSlug(rule)
	filename := "feedback_" + slug + ".md"
	findingPath := filepath.Join(root, filename)

	content := buildFindingContent(rule, whatHappened, remediation)
	//nolint:gosec // G306: user config file; 0o600 is intentional
	if err := os.WriteFile(findingPath, []byte(content), 0o600); err != nil {
		return "", fmt.Errorf("memory: write finding: %w", err)
	}

	if err := appendMemoryPointer(root, filename, rule); err != nil {
		return "", fmt.Errorf("memory: update MEMORY.md: %w", err)
	}

	return findingPath, nil
}

// buildFindingContent constructs the YAML-frontmatter + body content for
// a feedback finding file (legacy — uses derived slug).
func buildFindingContent(rule, whatHappened, remediation string) string {
	slug := findingSlug(rule)
	return buildFindingFullContent("feedback", slug, rule, rule, whatHappened, remediation)
}

// buildFindingFullContent constructs the YAML-frontmatter + body using the
// canonical memory file format per Common.md §5.1.
func buildFindingFullContent(memType, slug, description, rule, whatHappened, remediation string) string {
	if description == "" {
		description = rule
	}
	return fmt.Sprintf(
		"---\nname: %s\ndescription: %s\nrule: %s\nmetadata:\n  type: %s\n---\n\n## Rule\n\n%s\n\n**Why:** %s\n\n**How to apply:** %s\n",
		slug, description, rule, memType, rule, whatHappened, remediation,
	)
}

// appendMemoryPointer appends a one-line pointer entry to <root>/MEMORY.md,
// creating the file if absent.
func appendMemoryPointer(root, filename, rule string) error {
	memPath := filepath.Join(root, "MEMORY.md")
	line := fmt.Sprintf("- [%s](%s)\n", rule, filename)

	//nolint:gosec // G304: path derived from root (paths.MemoryDir()); not user-controlled
	f, err := os.OpenFile(memPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("memory: open MEMORY.md: %w", err)
	}
	defer func() { _ = f.Close() }()

	if _, err := f.WriteString(line); err != nil {
		return fmt.Errorf("memory: write MEMORY.md: %w", err)
	}
	return nil
}

// FindingSlug is the exported equivalent of findingSlug for callers that
// need to derive a slug without calling WriteFinding.
func FindingSlug(rule string) string { return findingSlug(rule) }

// WriteFindingFull writes a memory file with explicit type, slug, and
// description rather than deriving them from rule. The file is written as
// <root>/<type>_<slug>.md and a pointer appended to <root>/MEMORY.md.
func WriteFindingFull(root, memType, slug, description, rule, whatHappened, remediation string) (string, error) {
	if err := os.MkdirAll(root, 0o750); err != nil {
		return "", fmt.Errorf("memory: mkdir %q: %w", root, err)
	}
	filename := memType + "_" + slug + ".md"
	findingPath := filepath.Join(root, filename)
	content := buildFindingFullContent(memType, slug, description, rule, whatHappened, remediation)
	//nolint:gosec // G306: user config file; 0o600 is intentional
	if err := os.WriteFile(findingPath, []byte(content), 0o600); err != nil {
		return "", fmt.Errorf("memory: write finding: %w", err)
	}
	pointer := fmt.Sprintf("- [%s](%s) — %s\n", slug, filename, description)
	memPath := filepath.Join(root, "MEMORY.md")
	//nolint:gosec // G304: internal path
	f, err := os.OpenFile(memPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return "", fmt.Errorf("memory: open MEMORY.md: %w", err)
	}
	defer func() { _ = f.Close() }()
	if _, err := f.WriteString(pointer); err != nil {
		return "", fmt.Errorf("memory: write MEMORY.md: %w", err)
	}
	return findingPath, nil
}

// findingSlug returns the first 32 characters of rule, lowercased,
// with whitespace and filesystem-unsafe characters replaced by hyphens.
func findingSlug(rule string) string {
	s := strings.ToLower(rule)
	var b strings.Builder
	for _, r := range s {
		switch r { //nolint:staticcheck // QF1002: explicit case list is clearer here
		case ' ', '\t', '/', '\\', ':', '*', '?', '"', '<', '>', '|':
			b.WriteRune('-')
		default:
			b.WriteRune(r)
		}
	}
	result := b.String()
	if len(result) > 32 {
		return result[:32]
	}
	return result
}
