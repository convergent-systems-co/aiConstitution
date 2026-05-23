// Package patterns provides Go bindings for hooks/patterns.json — the
// canonical secret-pattern source consumed by:
//
//   - hooks/secret-block.py        (PreToolUse; Python)
//   - hooks/secret-precommit.py    (git pre-commit; Python)
//   - bin/ai pre-sync scan         (Go; this package)
//   - bin/ai issue file redaction  (Go; this package)
//
// The schema is defined inline here and matches the JSON file's shape
// exactly. The same patterns are guaranteed to fire identically across
// the Python and Go consumers because they're loaded from the same
// source file.
package patterns

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Set is a versioned collection of secret patterns.
type Set struct {
	Version  string    `json:"version"`
	Patterns []Pattern `json:"patterns"`
}

// Pattern is one entry in the set.
type Pattern struct {
	ID        string `json:"id"`
	Regex     string `json:"regex"`
	Severity  string `json:"severity"`  // "high" | "medium" | "low"
	Redaction string `json:"redaction"` // e.g. "[REDACTED:github-token]"

	// Optional: a short human-readable description. Not used by the
	// matcher; surfaced in violation reports.
	Description string `json:"description,omitempty"`

	// Compiled lazily on first use.
	compiled *regexp.Regexp
}

// Compile parses every pattern's regex. Returns the first compilation
// error encountered.
func (s *Set) Compile() error {
	for i := range s.Patterns {
		re, err := regexp.Compile(s.Patterns[i].Regex)
		if err != nil {
			return fmt.Errorf("pattern %q: %w", s.Patterns[i].ID, err)
		}
		s.Patterns[i].compiled = re
	}
	return nil
}

// Match represents a single hit.
type Match struct {
	PatternID  string
	Severity   string
	Redaction  string
	LineNumber int
	Column     int
	Snippet    string // already redacted
}

// Scan walks the input through every compiled pattern and returns
// every match. Multi-byte safe; line/column are 1-based.
func (s *Set) Scan(input string) []Match {
	var hits []Match
	lines := strings.Split(input, "\n")
	for lineIdx, line := range lines {
		for _, p := range s.Patterns {
			if p.compiled == nil {
				continue
			}
			for _, loc := range p.compiled.FindAllStringIndex(line, -1) {
				hits = append(hits, Match{
					PatternID:  p.ID,
					Severity:   p.Severity,
					Redaction:  p.Redaction,
					LineNumber: lineIdx + 1,
					Column:     loc[0] + 1,
					Snippet:    redactSnippet(line, loc[0], loc[1], p.Redaction),
				})
			}
		}
	}
	return hits
}

// Redact applies every pattern's redaction to the input, returning a
// safe-to-publish copy.
func (s *Set) Redact(input string) string {
	out := input
	for _, p := range s.Patterns {
		if p.compiled == nil {
			continue
		}
		out = p.compiled.ReplaceAllString(out, p.Redaction)
	}
	return out
}

// Load parses a patterns.json file from disk and compiles every regex.
func Load(path string) (*Set, error) {
	// gosec G304: caller supplies the path; filepath.Clean is the
	// conventional acknowledgement that this is intentional.
	f, err := os.Open(filepath.Clean(path))
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	return ReadFrom(f)
}

// ReadFrom parses a Set from any io.Reader.
func ReadFrom(r io.Reader) (*Set, error) {
	var s Set
	if err := json.NewDecoder(r).Decode(&s); err != nil {
		return nil, fmt.Errorf("patterns: %w", err)
	}
	if err := s.Compile(); err != nil {
		return nil, err
	}
	return &s, nil
}

// redactSnippet returns a short context around the match with the
// matched substring replaced by the redaction marker.
func redactSnippet(line string, start, end int, redaction string) string {
	const ctx = 20
	a := start - ctx
	if a < 0 {
		a = 0
	}
	b := end + ctx
	if b > len(line) {
		b = len(line)
	}
	return line[a:start] + redaction + line[end:b]
}
