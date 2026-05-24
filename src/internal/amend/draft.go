package amend

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// ParseRef splits a reference of the form "Common.md/U17" into
// ("Common.md", "U17", nil). Either component empty → error.
func ParseRef(ref string) (file, section string, err error) {
	idx := strings.Index(ref, "/")
	if idx <= 0 || idx >= len(ref)-1 {
		return "", "", fmt.Errorf("amend: ref %q must have form <file>/<section>", ref)
	}
	file = strings.TrimSpace(ref[:idx])
	section = strings.TrimSpace(ref[idx+1:])
	if file == "" || section == "" {
		return "", "", fmt.Errorf("amend: ref %q has empty file or section", ref)
	}
	return file, section, nil
}

// LocateSection finds the byte range [start, end) in content that
// covers the section named by section. Returns (0, 0, false) when the
// section header cannot be located.
//
// Matching is permissive: any heading line (one or more '#') whose
// trimmed body contains the section token after stripping common
// prefixes ("§", "U", "P", "##") matches. Trailing colons, dots, and
// numeric prefixes ("11.2.") are tolerated. The matched section
// extends from the heading line through (but not including) the next
// heading at the same depth or higher.
func LocateSection(content, section string) (start, end int, found bool) {
	if content == "" || section == "" {
		return 0, 0, false
	}
	target := normalizeSectionToken(section)
	if target == "" {
		return 0, 0, false
	}

	lines := strings.Split(content, "\n")
	// Compute line-start byte offsets.
	offsets := make([]int, len(lines)+1)
	acc := 0
	for i, l := range lines {
		offsets[i] = acc
		acc += len(l) + 1
	}
	offsets[len(lines)] = acc

	headingRe := regexp.MustCompile(`^(#+)\s+(.*)$`)
	matchedLine := -1
	matchedDepth := 0
	for i, l := range lines {
		m := headingRe.FindStringSubmatch(l)
		if m == nil {
			continue
		}
		depth := len(m[1])
		body := m[2]
		if sectionLineMatches(body, target) {
			matchedLine = i
			matchedDepth = depth
			break
		}
	}
	if matchedLine < 0 {
		return 0, 0, false
	}
	// Find end: next heading at depth <= matchedDepth.
	endLine := len(lines)
	for j := matchedLine + 1; j < len(lines); j++ {
		m := headingRe.FindStringSubmatch(lines[j])
		if m == nil {
			continue
		}
		depth := len(m[1])
		if depth <= matchedDepth {
			endLine = j
			break
		}
	}
	return offsets[matchedLine], offsets[endLine], true
}

// normalizeSectionToken strips a section reference to its core
// identifier. Examples:
//
//	"U17"     -> "U17"
//	"§U17"    -> "U17"
//	"§11.2"   -> "11.2"
//	"11.2"    -> "11.2"
func normalizeSectionToken(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "§")
	s = strings.TrimPrefix(s, "#")
	s = strings.TrimSpace(s)
	return s
}

// sectionLineMatches reports whether a heading body refers to the
// target section token.
func sectionLineMatches(body, target string) bool {
	body = strings.TrimSpace(body)
	tokens := strings.FieldsFunc(body, func(r rune) bool {
		switch r {
		case ' ', '\t', '.', ':', ',', '(', ')', '—', '-':
			return true
		}
		return false
	})
	for _, t := range tokens {
		t = strings.TrimPrefix(t, "§")
		if t == target {
			return true
		}
	}
	// Accept numeric-dotted patterns ("11.2") by string contains with
	// word-boundary check.
	if strings.Contains(body, target) {
		idx := strings.Index(body, target)
		ok := true
		if idx > 0 {
			c := body[idx-1]
			if isAlphaNum(c) {
				ok = false
			}
		}
		if ok && idx+len(target) < len(body) {
			c := body[idx+len(target)]
			if isAlphaNum(c) {
				ok = false
			}
		}
		if ok {
			return true
		}
	}
	return false
}

func isAlphaNum(c byte) bool {
	return (c >= '0' && c <= '9') || (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z')
}

// WriteDraft writes d to plansDir/<UTC>-<slug>.md and returns the
// absolute path. plansDir is created if absent. The body is YAML
// frontmatter followed by a Markdown "Proposed change" section.
func WriteDraft(d Draft, plansDir string) (string, error) {
	if plansDir == "" {
		return "", errors.New("amend: plansDir is empty")
	}
	if d.File == "" || d.Section == "" {
		return "", errors.New("amend: draft requires File and Section")
	}
	if d.Created.IsZero() {
		d.Created = time.Now().UTC()
	}
	if d.Slug == "" {
		d.Slug = slugify(d.File, d.Section)
	}
	if err := os.MkdirAll(plansDir, 0o750); err != nil {
		return "", fmt.Errorf("amend: mkdir %s: %w", plansDir, err)
	}

	stamp := d.Created.UTC().Format("2006-01-02T150405Z")
	name := stamp + "-" + d.Slug + ".md"
	dst := filepath.Join(plansDir, name)

	body := renderDraftBody(d)
	if err := os.WriteFile(dst, []byte(body), 0o640); err != nil {
		return "", fmt.Errorf("amend: write %s: %w", dst, err)
	}
	return dst, nil
}

// renderDraftBody produces the on-disk Markdown for a Draft.
func renderDraftBody(d Draft) string {
	var b strings.Builder
	b.WriteString("---\n")
	fmt.Fprintf(&b, "file: %s\n", d.File)
	fmt.Fprintf(&b, "section: %s\n", d.Section)
	fmt.Fprintf(&b, "audit_ref: %s\n", d.AuditRef)
	fmt.Fprintf(&b, "rationale: %s\n", d.Rationale)
	fmt.Fprintf(&b, "created: %s\n", d.Created.UTC().Format(time.RFC3339))
	b.WriteString("---\n\n")
	fmt.Fprintf(&b, "# Amendment draft: %s / %s\n\n", d.File, d.Section)
	b.WriteString("## Proposed change\n\n")
	if d.ProposedChange != "" {
		b.WriteString(d.ProposedChange)
		if !strings.HasSuffix(d.ProposedChange, "\n") {
			b.WriteString("\n")
		}
	} else {
		b.WriteString("<!-- Replace this comment with the prose to append to the section. -->\n")
	}
	b.WriteString("\n## Rationale\n\n")
	if d.Rationale != "" {
		b.WriteString(d.Rationale)
		b.WriteString("\n")
	} else {
		b.WriteString("<!-- One-line 'why'. Will appear in the Changelog entry on apply. -->\n")
	}
	return b.String()
}
