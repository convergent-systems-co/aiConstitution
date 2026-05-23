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

import "time"

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

// Append serializes the event as a single JSONL line and appends it
// to audit/interactions/<YYYY-MM>.jsonl. Returns the path written.
//
// TBD for v0.8: actual filesystem write. Stub returns the would-be
// path so callers can validate routing.
func Append(_ Event) (string, error) {
	return "", nil
}

// RecordOverride writes an override record to audit/overrides/<UTC>.md
// per Constitution.md §5.1. The body is the principal-confirmed
// override block.
//
// TBD for v0.8.
func RecordOverride(_ string) (string, error) {
	return "", nil
}

// RecordViolation writes a violation record to audit/violations/<UTC>.md
// per Constitution.md §5.2.
//
// TBD for v0.8.
func RecordViolation(_ string) (string, error) {
	return "", nil
}
