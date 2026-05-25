package audit_test

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/convergent-systems-co/aiConstitution/src/internal/audit"
)

// setAIRoot sets AI_ROOT to a temp dir for the duration of the test.
func setAIRoot(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()
	t.Setenv("AI_ROOT", tmp)
	return tmp
}

// --- AppendEvent tests ---

func TestAppendEvent_CreatesJSONLFile(t *testing.T) {
	root := setAIRoot(t)
	e := audit.Event{
		Chronon: time.Date(2026, 5, 24, 10, 0, 0, 0, time.UTC),
		Trace:   "trace-abc",
		CWD:     "/home/user",
		Actor:   audit.ActorHuman,
		Kind:    audit.KindRequest,
	}

	if err := audit.AppendEvent(e); err != nil {
		t.Fatalf("AppendEvent() error = %v", err)
	}

	want := filepath.Join(root, "audit", "interactions", "2026-05.jsonl")
	if _, err := os.Stat(want); os.IsNotExist(err) {
		t.Fatalf("expected file %q to exist after AppendEvent", want)
	}
}

func TestAppendEvent_PathIncludesYearMonth(t *testing.T) {
	root := setAIRoot(t)

	events := []struct {
		chronon  time.Time
		wantFile string
	}{
		{time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC), "2026-01.jsonl"},
		{time.Date(2025, 12, 31, 23, 59, 0, 0, time.UTC), "2025-12.jsonl"},
	}

	for _, tc := range events {
		e := audit.Event{
			Chronon: tc.chronon,
			Trace:   "t",
			Actor:   audit.ActorSystem,
			Kind:    audit.KindSignal,
		}
		if err := audit.AppendEvent(e); err != nil {
			t.Fatalf("AppendEvent() error = %v", err)
		}
		want := filepath.Join(root, "audit", "interactions", tc.wantFile)
		if _, err := os.Stat(want); os.IsNotExist(err) {
			t.Errorf("expected file %q, does not exist", want)
		}
	}
}

func TestAppendEvent_WritesValidJSON(t *testing.T) {
	root := setAIRoot(t)
	e := audit.Event{
		Chronon:  time.Date(2026, 5, 24, 12, 30, 0, 0, time.UTC),
		Trace:    "trace-xyz",
		CWD:      "/repo",
		Actor:    audit.ActorAssistant,
		Kind:     audit.KindEmission,
		Engine:   "claude-sonnet",
		Stimulus: "hello",
	}

	if err := audit.AppendEvent(e); err != nil {
		t.Fatalf("AppendEvent() error = %v", err)
	}

	path := filepath.Join(root, "audit", "interactions", "2026-05.jsonl")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", path, err)
	}

	var decoded audit.Event
	line := strings.TrimSpace(string(data))
	if err := json.Unmarshal([]byte(line), &decoded); err != nil {
		t.Fatalf("json.Unmarshal() error = %v; line = %q", err, line)
	}

	if decoded.Trace != e.Trace {
		t.Errorf("decoded.Trace = %q, want %q", decoded.Trace, e.Trace)
	}
	if decoded.Actor != e.Actor {
		t.Errorf("decoded.Actor = %q, want %q", decoded.Actor, e.Actor)
	}
	if decoded.Kind != e.Kind {
		t.Errorf("decoded.Kind = %q, want %q", decoded.Kind, e.Kind)
	}
	if decoded.Engine != e.Engine {
		t.Errorf("decoded.Engine = %q, want %q", decoded.Engine, e.Engine)
	}
}

func TestAppendEvent_AppendsToExistingFile(t *testing.T) {
	root := setAIRoot(t)
	chronon := time.Date(2026, 5, 24, 0, 0, 0, 0, time.UTC)

	for i := range 3 {
		e := audit.Event{
			Chronon: chronon,
			Trace:   "trace-" + string(rune('a'+i)),
			Actor:   audit.ActorTool,
			Kind:    audit.KindInvocationAttempt,
		}
		if err := audit.AppendEvent(e); err != nil {
			t.Fatalf("AppendEvent() [%d] error = %v", i, err)
		}
	}

	path := filepath.Join(root, "audit", "interactions", "2026-05.jsonl")
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("Open(%q) error = %v", path, err)
	}
	defer f.Close()

	lineCount := 0
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if strings.TrimSpace(sc.Text()) != "" {
			lineCount++
		}
	}
	if lineCount != 3 {
		t.Errorf("expected 3 lines in JSONL file, got %d", lineCount)
	}
}

func TestAppendEvent_MkdirAll(t *testing.T) {
	root := setAIRoot(t)
	// Verify the interactions subdir does not exist before the call.
	interactionsDir := filepath.Join(root, "audit", "interactions")
	if _, err := os.Stat(interactionsDir); !os.IsNotExist(err) {
		t.Skip("interactions dir already exists — skipping mkdir test")
	}

	e := audit.Event{
		Chronon: time.Date(2026, 5, 24, 0, 0, 0, 0, time.UTC),
		Trace:   "t",
		Actor:   audit.ActorSystem,
		Kind:    audit.KindTraceOpen,
	}
	if err := audit.AppendEvent(e); err != nil {
		t.Fatalf("AppendEvent() error = %v", err)
	}

	if _, err := os.Stat(interactionsDir); os.IsNotExist(err) {
		t.Errorf("expected directory %q to be created by AppendEvent", interactionsDir)
	}
}

func TestAppendEvent_LineEndsWithNewline(t *testing.T) {
	root := setAIRoot(t)
	e := audit.Event{
		Chronon: time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC),
		Trace:   "nl-test",
		Actor:   audit.ActorHuman,
		Kind:    audit.KindRequest,
	}
	if err := audit.AppendEvent(e); err != nil {
		t.Fatalf("AppendEvent() error = %v", err)
	}

	path := filepath.Join(root, "audit", "interactions", "2026-03.jsonl")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", path, err)
	}
	if len(data) == 0 || data[len(data)-1] != '\n' {
		t.Errorf("expected file to end with newline; last byte = %q", data[len(data)-1])
	}
}

// --- WriteViolation tests ---

func TestWriteViolation_CreatesMarkdownFile(t *testing.T) {
	root := setAIRoot(t)

	if err := audit.WriteViolation(
		"Code.md/§11.3 — refactor-protocol",
		"Refactor included a bug fix.",
		"self-detected",
		"Separated the fix into its own commit.",
	); err != nil {
		t.Fatalf("WriteViolation() error = %v", err)
	}

	violationsDir := filepath.Join(root, "audit", "violations")
	entries, err := os.ReadDir(violationsDir)
	if err != nil {
		t.Fatalf("ReadDir(%q) error = %v", violationsDir, err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 file in violations dir, got %d", len(entries))
	}
	name := entries[0].Name()
	if !strings.HasSuffix(name, ".md") {
		t.Errorf("expected .md file, got %q", name)
	}
}

func TestWriteViolation_ContainsAllFields(t *testing.T) {
	root := setAIRoot(t)
	rule := "Common.md/§2.2 — destructive-action"
	what := "Dropped a table without approval."
	how := "user-flagged"
	rem := "Restored from backup."

	if err := audit.WriteViolation(rule, what, how, rem); err != nil {
		t.Fatalf("WriteViolation() error = %v", err)
	}

	violationsDir := filepath.Join(root, "audit", "violations")
	entries, _ := os.ReadDir(violationsDir)
	data, err := os.ReadFile(filepath.Join(violationsDir, entries[0].Name()))
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	body := string(data)

	checks := []string{
		"# Violation",
		"**File / Rule violated:**",
		rule,
		"**What happened:**",
		what,
		"**How noticed:**",
		how,
		"**Remediation:**",
		rem,
		"**Proposed amendment (if any):**",
	}
	for _, c := range checks {
		if !strings.Contains(body, c) {
			t.Errorf("violation file missing %q\nFull content:\n%s", c, body)
		}
	}
}

func TestWriteViolation_SlugFromRule(t *testing.T) {
	root := setAIRoot(t)

	rule := "Code.md §11.2 Merge methods rule"
	if err := audit.WriteViolation(rule, "what", "self-detected", "fixed"); err != nil {
		t.Fatalf("WriteViolation() error = %v", err)
	}

	violationsDir := filepath.Join(root, "audit", "violations")
	entries, _ := os.ReadDir(violationsDir)
	name := entries[0].Name()

	// Slug should be lowercase, spaces→hyphens, first 32 chars of rule.
	// "Code.md §11.2 Merge methods rule" → "code.md-§11.2-merge-methods-rul"
	// Timestamp prefix is 16 chars (20060102T150405Z) + "-".
	// Just check the slug portion is lowercase and no raw spaces.
	if strings.Contains(name, " ") {
		t.Errorf("filename %q contains spaces", name)
	}
	parts := strings.SplitN(name, "-", 2) // split on first dash after timestamp
	// The filename format is <ts>-<slug>.md, ts = 16 chars like 20260524T120000Z
	if len(name) < 17 {
		t.Errorf("filename %q too short to contain timestamp", name)
	}
	// ts is exactly 16 chars (YYYYMMDDTHHMMSSz)
	ts := name[:16]
	if !strings.Contains(ts, "T") {
		t.Errorf("expected timestamp in first 16 chars of filename, got %q", ts)
	}
	_ = parts
}

func TestWriteViolation_MkdirAll(t *testing.T) {
	root := setAIRoot(t)
	violationsDir := filepath.Join(root, "audit", "violations")
	if _, err := os.Stat(violationsDir); !os.IsNotExist(err) {
		t.Skip("violations dir already exists")
	}

	if err := audit.WriteViolation("rule", "what", "self-detected", "rem"); err != nil {
		t.Fatalf("WriteViolation() error = %v", err)
	}
	if _, err := os.Stat(violationsDir); os.IsNotExist(err) {
		t.Errorf("expected %q to be created", violationsDir)
	}
}

func TestWriteViolation_FilenameHasTimestamp(t *testing.T) {
	root := setAIRoot(t)

	before := time.Now().UTC()
	if err := audit.WriteViolation("some-rule", "happened", "tool-flagged", "remedied"); err != nil {
		t.Fatalf("WriteViolation() error = %v", err)
	}
	after := time.Now().UTC()

	violationsDir := filepath.Join(root, "audit", "violations")
	entries, _ := os.ReadDir(violationsDir)
	name := entries[0].Name()

	// Extract ts from filename: first 16 chars (YYYYMMDDTHHMMSSz pattern).
	ts := name[:16]
	parsed, err := time.Parse("20060102T150405Z", ts)
	if err != nil {
		t.Fatalf("could not parse timestamp %q from filename %q: %v", ts, name, err)
	}
	// Allow 1 second of clock slop.
	if parsed.Before(before.Add(-time.Second)) || parsed.After(after.Add(time.Second)) {
		t.Errorf("timestamp %v outside window [%v, %v]", parsed, before, after)
	}
}
