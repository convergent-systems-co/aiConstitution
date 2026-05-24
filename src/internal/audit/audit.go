// Package audit writes the canonical audit records:
//
//   - audit/interactions/<YYYY-MM>.jsonl  — JSONL, one line per event.
//     Local-only; never synced. Schema per ~/.ai/Common.md §5.2.
//
//   - audit/overrides/<UTC>.md    — one file per override event.
//     Schema per ~/.ai/Constitution.md §5.1.
//
//   - audit/violations/<UTC>.md   — one file per self-noticed
//     violation event. Schema per ~/.ai/Constitution.md §5.2.
//
// All three subdirectories live under paths.AuditDir(). The
// interactions/ subdir is .gitignored by spec; overrides/ and
// violations/ are tracked.
package audit

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/convergent-systems-co/aiConstitution/src/internal/paths"
)

// Kind enumerates the interaction-log event types per Common.md §5.2
// plus the SPEC.md §6.6 focus-change addition.
type Kind string

// Interaction-log event kinds. The vocabulary is deliberately distinct
// from Claude/Copilot internal log terms so audit lines stay greppable
// independent of any one tool's idioms.
const (
	KindRequest            Kind = "request"
	KindEmission           Kind = "emission"
	KindInvocationAttempt  Kind = "invocation-attempt"
	KindInvocationResult   Kind = "invocation-result"
	KindTraceOpen          Kind = "trace-open"
	KindTraceClose         Kind = "trace-close"
	KindSignal             Kind = "signal"
	KindSubagentEmission   Kind = "subagent-emission"
	KindCompactionAttempt  Kind = "compaction-attempt"
	KindFocusChange        Kind = "focus-change"
	KindFocusMergeOverride Kind = "focus-merge-override"
)

// Actor enumerates the canonical actors.
type Actor string

// Canonical actor values for audit events.
const (
	ActorHuman     Actor = "human"
	ActorAssistant Actor = "assistant"
	ActorTool      Actor = "tool"
	ActorSystem    Actor = "system"
)

// Event is the on-disk interaction-log shape. Fields are
// vocabulary-deliberately-distinct from Claude / Copilot internal
// log terms so the audit lines stay grep-able independent of any
// one tool's idioms.
type Event struct {
	Chronon        time.Time `json:"chronon"`
	Trace          string    `json:"trace"`
	CWD            string    `json:"cwd"`
	Actor          Actor     `json:"actor"`
	Kind           Kind      `json:"kind"`
	Engine         string    `json:"engine,omitempty"`
	Stimulus       string    `json:"stimulus,omitempty"`
	Probe          string    `json:"probe,omitempty"`
	ProbePayload   string    `json:"probe_payload,omitempty"`
	EmissionMarker string    `json:"emission_marker,omitempty"`

	// focus-change specifics (SPEC.md §6.6).
	FocusFrom   string `json:"focus_from,omitempty"`
	FocusTo     string `json:"focus_to,omitempty"`
	FocusSource string `json:"focus_source,omitempty"`
}

// AppendEvent serializes e as a single JSONL line and appends it to
// audit/interactions/<YYYY-MM>.jsonl under paths.AuditDir().
// The directory is created with mode 0o750 if absent.
// The file is opened with mode 0o600 (user-only write; common.md §4 —
// secrets may appear in stimulus/probe_payload fields).
func AppendEvent(e Event) error {
	dir := filepath.Join(paths.AuditDir(), "interactions")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("audit: mkdir interactions: %w", err)
	}

	line, err := json.Marshal(e)
	if err != nil {
		return fmt.Errorf("audit: marshal event: %w", err)
	}
	line = append(line, '\n')

	path := filepath.Join(dir, e.Chronon.UTC().Format("2006-01")+".jsonl")
	//nolint:gosec // G304: path is derived from paths.AuditDir() — not user-controlled
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("audit: open interactions file: %w", err)
	}
	defer f.Close()

	if _, err := f.Write(line); err != nil {
		return fmt.Errorf("audit: write event: %w", err)
	}
	return nil
}

// WriteViolation writes a violation markdown record to
// audit/violations/<ts>-<slug>.md under paths.AuditDir().
// ts is the current UTC time formatted as 20060102T150405Z.
// slug is the first 32 characters of rule, lowercased, spaces replaced
// with hyphens. The directory is created with 0o750 if absent.
func WriteViolation(rule, whatHappened, howNoticed, remediation string) error {
	dir := filepath.Join(paths.AuditDir(), "violations")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("audit: mkdir violations: %w", err)
	}

	ts := time.Now().UTC().Format("20060102T150405Z")
	slug := violationSlug(rule)
	path := filepath.Join(dir, ts+"-"+slug+".md")

	body := fmt.Sprintf(
		"# Violation — %s\n\n- **File / Rule violated:** %s\n- **What happened:** %s\n- **How noticed:** %s\n- **Remediation:** %s\n- **Proposed amendment (if any):** \n",
		ts, rule, whatHappened, howNoticed, remediation,
	)

	//nolint:gosec // G306: user config file (violation log); 0o600 is intentional
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		return fmt.Errorf("audit: write violation file: %w", err)
	}
	return nil
}

// violationSlug returns the first 32 characters of rule, lowercased,
// with spaces and filesystem-unsafe characters replaced by hyphens.
// Safe for use as a POSIX filename segment.
func violationSlug(rule string) string {
	s := strings.ToLower(rule)
	// Replace whitespace, slashes, and other chars unsafe in filenames.
	var b strings.Builder
	for _, r := range s {
		switch {
		case r == ' ' || r == '/' || r == '\\' || r == ':' || r == '*' ||
			r == '?' || r == '"' || r == '<' || r == '>' || r == '|':
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

// Append serializes the event as a single JSONL line and appends it
// to audit/interactions/<YYYY-MM>.jsonl. Delegates to AppendEvent.
// Kept for backward compatibility.
func Append(e Event) (string, error) {
	return "", AppendEvent(e)
}

// RecordOverride writes an override record to audit/overrides/<UTC>.md
// per Constitution.md §5.1. The body is the principal-confirmed
// override block.
//
// TBD: full structured implementation pending override-workflow task.
func RecordOverride(_ string) (string, error) {
	return "", nil
}

// RecordViolation writes a violation record to audit/violations/<UTC>.md
// per Constitution.md §5.2. Delegates to WriteViolation.
// Kept for backward compatibility.
func RecordViolation(body string) (string, error) {
	return "", WriteViolation(body, "", "tool-flagged", "")
}
