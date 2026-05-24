package amend

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// LoadDraft reads a draft file written by WriteDraft and parses its
// frontmatter + body back into a Draft.
func LoadDraft(path string) (Draft, error) {
	if path == "" {
		return Draft{}, errors.New("amend: path is empty")
	}
	data, err := os.ReadFile(path) //nolint:gosec // G304: caller-provided draft path
	if err != nil {
		return Draft{}, fmt.Errorf("amend: read %s: %w", path, err)
	}
	return parseDraftBody(string(data))
}

// parseDraftBody parses the YAML-ish frontmatter and "Proposed change"
// section out of a draft file body.
func parseDraftBody(body string) (Draft, error) {
	var d Draft
	rest := body
	if strings.HasPrefix(body, "---\n") {
		end := strings.Index(body[4:], "\n---")
		if end < 0 {
			return Draft{}, errors.New("amend: malformed frontmatter (missing closing ---)")
		}
		fm := body[4 : 4+end]
		rest = body[4+end+4:]
		rest = strings.TrimLeft(rest, "\n")
		for _, line := range strings.Split(fm, "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			colon := strings.IndexByte(line, ':')
			if colon < 0 {
				continue
			}
			key := strings.TrimSpace(line[:colon])
			val := strings.TrimSpace(line[colon+1:])
			switch key {
			case "file":
				d.File = val
			case "section":
				d.Section = val
			case "audit_ref":
				d.AuditRef = val
			case "rationale":
				d.Rationale = val
			case "created":
				if t, err := time.Parse(time.RFC3339, val); err == nil {
					d.Created = t
				}
			}
		}
	}

	d.ProposedChange = extractSection(rest, "Proposed change")
	if d.Rationale == "" {
		d.Rationale = extractSection(rest, "Rationale")
	}

	if d.File == "" || d.Section == "" {
		return Draft{}, errors.New("amend: draft is missing file or section")
	}
	return d, nil
}

// extractSection pulls the prose under a "## <name>" heading until the
// next "## " heading. HTML-style placeholder comments are stripped.
func extractSection(body, name string) string {
	lower := strings.ToLower(body)
	needle := strings.ToLower("## " + name)
	idx := strings.Index(lower, needle)
	if idx < 0 {
		return ""
	}
	nl := strings.IndexByte(body[idx:], '\n')
	if nl < 0 {
		return ""
	}
	start := idx + nl + 1
	end := len(body)
	rest := body[start:]
	if next := strings.Index(rest, "\n## "); next >= 0 {
		end = start + next
	}
	out := strings.TrimSpace(body[start:end])
	out = stripHTMLComments(out)
	return strings.TrimSpace(out)
}

func stripHTMLComments(s string) string {
	for {
		open := strings.Index(s, "<!--")
		if open < 0 {
			return s
		}
		closeIdx := strings.Index(s[open:], "-->")
		if closeIdx < 0 {
			return s
		}
		s = s[:open] + s[open+closeIdx+3:]
	}
}

// versionRe matches a "**Version:** X.Y" line. Captures the prefix
// (anything up to and including "**Version:** "), the major, and the
// minor version digits.
var versionRe = regexp.MustCompile(`(?m)^(\*\*Version:\*\*\s+)(\d+)\.(\d+)(.*)$`)

// BumpVersion bumps the minor version in the first matching
// "**Version:** X.Y" line found in content. Returns the new content
// and the (old, new) version strings. If no version line is found,
// returns content unchanged and "", "".
func BumpVersion(content string) (newContent, oldVersion, newVersion string) {
	loc := versionRe.FindStringSubmatchIndex(content)
	if loc == nil {
		return content, "", ""
	}
	match := versionRe.FindStringSubmatch(content)
	prefix := match[1]
	major := match[2]
	minor, err := strconv.Atoi(match[3])
	if err != nil {
		return content, "", ""
	}
	suffix := match[4]

	oldVersion = major + "." + match[3]
	newVersion = major + "." + strconv.Itoa(minor+1)
	replacement := prefix + newVersion + suffix
	newContent = content[:loc[0]] + replacement + content[loc[1]:]
	return newContent, oldVersion, newVersion
}

// AppendChangelog inserts a new bullet entry under the document's
// Changelog heading. The Changelog heading is recognized as any
// markdown heading whose body trimmed equals "Changelog" (case-
// insensitive), possibly prefixed with "## " or "## N. ".
//
// The entry is inserted at the top of the Changelog body (newest
// first) per existing convention in Common.md / Code.md.
func AppendChangelog(content, entry string) string {
	headingRe := regexp.MustCompile(`(?mi)^#+\s+(?:\d+\.\s+)?changelog\s*$`)
	loc := headingRe.FindStringIndex(content)
	if loc == nil {
		out := strings.TrimRight(content, "\n") + "\n\n## Changelog\n\n" + entry + "\n"
		return out
	}
	nl := strings.IndexByte(content[loc[1]:], '\n')
	if nl < 0 {
		return content + "\n\n" + entry + "\n"
	}
	insertAt := loc[1] + nl + 1
	for insertAt < len(content) && content[insertAt] == '\n' {
		insertAt++
	}
	return content[:insertAt] + entry + "\n\n" + content[insertAt:]
}

// Apply executes the amendment described by d against aiRoot. Returns
// the ApplyResult describing what changed.
func Apply(d Draft, aiRoot string) (ApplyResult, error) {
	if aiRoot == "" {
		return ApplyResult{}, errors.New("amend: aiRoot is empty")
	}
	if d.File == "" || d.Section == "" {
		return ApplyResult{}, errors.New("amend: draft requires File and Section")
	}
	targetPath := filepath.Join(aiRoot, d.File)
	data, err := os.ReadFile(targetPath) //nolint:gosec // G304: aiRoot + canonical filename
	if err != nil {
		return ApplyResult{}, fmt.Errorf("amend: read %s: %w", targetPath, err)
	}
	content := string(data)

	if strings.TrimSpace(d.ProposedChange) != "" {
		_, end, found := LocateSection(content, d.Section)
		insertBody := strings.TrimRight(d.ProposedChange, "\n") + "\n"
		if found {
			pre := strings.TrimRight(content[:end], "\n")
			content = pre + "\n\n" + insertBody + "\n" + content[end:]
		} else {
			content = strings.TrimRight(content, "\n") + "\n\n" + insertBody
		}
	}

	bumped, oldV, newV := BumpVersion(content)
	if oldV == "" {
		return ApplyResult{}, fmt.Errorf("amend: %s has no **Version:** line", targetPath)
	}
	content = bumped

	rationale := d.Rationale
	if rationale == "" {
		rationale = firstLine(d.ProposedChange)
	}
	if rationale == "" {
		rationale = "Amendment applied via ai amend apply"
	}
	entry := fmt.Sprintf("- **%s** — %s.", newV, strings.TrimRight(rationale, "."))
	if d.AuditRef != "" {
		entry += " Sourced from " + d.AuditRef + "."
	}
	content = AppendChangelog(content, entry)

	if err := writeAtomic(targetPath, []byte(content)); err != nil {
		return ApplyResult{}, err
	}

	auditPath, err := writeAuditRecord(aiRoot, d, oldV, newV, entry)
	if err != nil {
		return ApplyResult{}, err
	}

	return ApplyResult{
		TargetPath:     targetPath,
		OldVersion:     oldV,
		NewVersion:     newV,
		ChangelogEntry: entry,
		AuditPath:      auditPath,
	}, nil
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if nl := strings.IndexByte(s, '\n'); nl >= 0 {
		return strings.TrimSpace(s[:nl])
	}
	return s
}

func writeAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".amend-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}

func writeAuditRecord(aiRoot string, d Draft, oldV, newV, entry string) (string, error) {
	auditDir := filepath.Join(aiRoot, "audit", "overrides")
	if err := os.MkdirAll(auditDir, 0o750); err != nil {
		return "", err
	}
	stamp := time.Now().UTC().Format("2006-01-02T150405Z")
	slug := d.Slug
	if slug == "" {
		slug = slugify(d.File, d.Section)
	}
	name := stamp + "-" + slug + ".md"
	dst := filepath.Join(auditDir, name)

	var b strings.Builder
	fmt.Fprintf(&b, "# Amendment apply — %s\n\n", stamp)
	fmt.Fprintf(&b, "- **File:** %s\n", d.File)
	fmt.Fprintf(&b, "- **Section:** %s\n", d.Section)
	fmt.Fprintf(&b, "- **Old version:** %s\n", oldV)
	fmt.Fprintf(&b, "- **New version:** %s\n", newV)
	fmt.Fprintf(&b, "- **Audit ref:** %s\n", d.AuditRef)
	fmt.Fprintf(&b, "- **Changelog entry:** %s\n", entry)
	if d.ProposedChange != "" {
		b.WriteString("\n## Proposed change\n\n")
		b.WriteString(d.ProposedChange)
		if !strings.HasSuffix(d.ProposedChange, "\n") {
			b.WriteString("\n")
		}
	}
	if err := os.WriteFile(dst, []byte(b.String()), 0o640); err != nil {
		return "", err
	}
	return dst, nil
}
