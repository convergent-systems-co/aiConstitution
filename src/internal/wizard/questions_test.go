package wizard_test

import (
	"strings"
	"testing"

	"github.com/convergent-systems-co/aiConstitution/src/internal/wizard"
)

// v2Fixture is the embedded v2 questions.yaml content used across tests.
// It mirrors the schema in src/cmd/ai/embed/questions.yaml.
const v2Fixture = `
version: "1.0"

phases:

  - id: P1
    title: Identity
    mandatory: true
    questions:
      - qid: Q01
        prompt: What name or handle should the AI use for you?
        time: ~5s
        allow_free_text: true
        default: "Principal"
        persists_to: Constitution.md
        persists_to_section: "§1 header / §3 Principal"

      - qid: Q02
        prompt: Which AI tools will you use with this constitution?
        time: ~5s
        options:
          - label: Claude Code
            value: claude-code
          - label: Copilot CLI
            value: copilot-cli
        allow_free_text: true
        default: "claude-code"
        persists_to: Constitution.md
        persists_to_section: "§1.6 Integration"

      - qid: Q03
        prompt: What is your primary work context?
        time: ~5s
        allow_free_text: true
        allow_defer: true
        default: "personal"
        persists_to: Constitution.md
        persists_to_section: "§3 U4 provenance"

  - id: P2
    title: Domains
    mandatory: true
    questions:
      - qid: Q04
        prompt: What kinds of work will you do with AI assistance?
        time: ~30s
        options:
          - label: Software / Engineering
            value: technical
            note: Includes code, infra, testing, change management
          - label: Writing / Prose
            value: prose
        allow_free_text: true
        default: "technical"
        persists_to: Constitution.md
        persists_to_section: "§4-N domain stubs"

      - qid: Q05
        prompt: For each domain, what are your key rules?
        time: ~3m
        allow_chat: true
        allow_defer: true
        default: ""
        persists_to: Constitution.md
        persists_to_section: "§N.Personal"

  - id: P3
    title: Autonomy
    mandatory: true
    questions:
      - qid: Q06
        prompt: What is your per-task cost ceiling for cloud/API spending?
        time: ~5s
        options:
          - label: $5 (default)
            value: "$5"
          - label: No ceiling
            value: "none"
            warning: Removes a safety gate.
        default: "$5"
        persists_to: Constitution.md
        persists_to_section: "§3.2 Autonomy Gates §3.6"

      - qid: Q07
        prompt: How many files can the AI touch per task without asking?
        time: ~5s
        options:
          - label: 100 (default)
            value: "100"
        allow_free_text: true
        default: "100"
        persists_to: Constitution.md
        persists_to_section: "§3.6"

      - qid: Q08
        prompt: Which branch names should the AI never commit to directly?
        time: ~5s
        options:
          - label: main only
            value: "main"
        allow_free_text: true
        default: "main"
        persists_to: Constitution.md
        persists_to_section: "§3.2 + branch-guard.json"

      - qid: Q09
        prompt: What is your default autonomy posture for routine work?
        time: ~5s
        options:
          - label: Autonomous (default)
            value: autonomous
          - label: Cautious
            value: cautious
            warning: Significantly slows down routine work.
        default: "autonomous"
        persists_to: Constitution.md
        persists_to_section: "§3.2 §2.1"

  - id: P4
    title: Behavioral Style
    mandatory: true
    questions:
      - qid: Q10
        prompt: How hard should the AI push back when it disagrees with you?
        time: ~5s
        options:
          - label: Flag once, then follow my lead (default)
            value: flag-once
        default: "flag-once"
        persists_to: Constitution.md
        persists_to_section: "§2.1 personal overlay"

      - qid: Q11
        prompt: What response length do you prefer by default?
        time: ~5s
        options:
          - label: Match the complexity of my request (default)
            value: match-complexity
        default: "match-complexity"
        persists_to: Constitution.md
        persists_to_section: "§2.2 personal overlay"

      - qid: Q12
        prompt: What tone should the AI use when it disagrees with you?
        time: ~5s
        options:
          - label: Direct with framing (default)
            value: direct-framing
        default: "direct-framing"
        persists_to: Constitution.md
        persists_to_section: "§2.4 personal overlay"

      - qid: Q13
        prompt: Should AI involvement be noted in git commit trailers?
        time: ~5s
        options:
          - label: Yes — add Co-Authored-By trailer (default)
            value: "true"
          - label: No — omit AI attribution
            value: "false"
        default: "true"
        persists_to: Constitution.md
        persists_to_section: "§3.6 provenance"

  - id: P5
    title: Review
    mandatory: true
    questions:
      - qid: Q14
        prompt: Review your constitution before writing it to disk
        time: ~3m
        informational: false
        allow_chat: true
        allow_defer: false
        default: "confirmed"
        persists_to: Constitution.md
        persists_to_section: "final review"
`

func TestParseTaxonomy_V2(t *testing.T) {
	tax, err := wizard.ParseTaxonomy([]byte(v2Fixture))
	if err != nil {
		t.Fatalf("ParseTaxonomy() unexpected error: %v", err)
	}

	if tax.Version != "1.0" {
		t.Errorf("Version = %q, want %q", tax.Version, "1.0")
	}
	if len(tax.Phases) != 5 {
		t.Errorf("len(Phases) = %d, want 5", len(tax.Phases))
	}
	if tax.QuestionCount() != 14 {
		t.Errorf("QuestionCount() = %d, want 14", tax.QuestionCount())
	}
}

func TestParseTaxonomy_PhaseOrder(t *testing.T) {
	tax, err := wizard.ParseTaxonomy([]byte(v2Fixture))
	if err != nil {
		t.Fatalf("ParseTaxonomy() unexpected error: %v", err)
	}

	wantIDs := []string{"P1", "P2", "P3", "P4", "P5"}
	for i, want := range wantIDs {
		if tax.Phases[i].ID != want {
			t.Errorf("Phases[%d].ID = %q, want %q", i, tax.Phases[i].ID, want)
		}
	}
}

func TestParseTaxonomy_PhaseByID(t *testing.T) {
	tax, err := wizard.ParseTaxonomy([]byte(v2Fixture))
	if err != nil {
		t.Fatalf("ParseTaxonomy() unexpected error: %v", err)
	}

	p, ok := tax.PhaseByID("P3")
	if !ok {
		t.Fatal("PhaseByID(P3) returned false")
	}
	if p.Title != "Autonomy" {
		t.Errorf("Phase P3 title = %q, want %q", p.Title, "Autonomy")
	}

	_, ok = tax.PhaseByID("P99")
	if ok {
		t.Error("PhaseByID(P99) returned true for non-existent phase")
	}
}

func TestParseTaxonomy_AllMandatory(t *testing.T) {
	tax, err := wizard.ParseTaxonomy([]byte(v2Fixture))
	if err != nil {
		t.Fatalf("ParseTaxonomy() unexpected error: %v", err)
	}

	for _, p := range tax.Phases {
		if !p.Mandatory {
			t.Errorf("Phase %s: Mandatory = false, want true", p.ID)
		}
	}
}

func TestParseTaxonomy_QuestionByQID(t *testing.T) {
	tax, err := wizard.ParseTaxonomy([]byte(v2Fixture))
	if err != nil {
		t.Fatalf("ParseTaxonomy() unexpected error: %v", err)
	}

	// Q01 — free text, no options, persists to Constitution.md
	q01, ok := tax.QuestionByQID("Q01")
	if !ok {
		t.Fatal("QuestionByQID(Q01) returned false")
	}
	if !q01.AllowFreeText {
		t.Error("Q01: AllowFreeText = false, want true")
	}
	if q01.Default != "Principal" {
		t.Errorf("Q01: Default = %q, want %q", q01.Default, "Principal")
	}
	if q01.PeristsTo != "Constitution.md" {
		t.Errorf("Q01: PeristsTo = %q, want %q", q01.PeristsTo, "Constitution.md")
	}

	// Q06 — has a warning option
	q06, ok := tax.QuestionByQID("Q06")
	if !ok {
		t.Fatal("QuestionByQID(Q06) returned false")
	}
	var hasWarning bool
	for _, opt := range q06.Options {
		if opt.Warning != "" {
			hasWarning = true
		}
	}
	if !hasWarning {
		t.Error("Q06: expected at least one option with a warning, got none")
	}

	// Q14 — last question, Review phase
	q14, ok := tax.QuestionByQID("Q14")
	if !ok {
		t.Fatal("QuestionByQID(Q14) returned false")
	}
	if q14.Default != "confirmed" {
		t.Errorf("Q14: Default = %q, want %q", q14.Default, "confirmed")
	}

	// Non-existent QID
	_, ok = tax.QuestionByQID("Q99")
	if ok {
		t.Error("QuestionByQID(Q99) returned true for non-existent qid")
	}
}

func TestParseTaxonomy_Options(t *testing.T) {
	tax, err := wizard.ParseTaxonomy([]byte(v2Fixture))
	if err != nil {
		t.Fatalf("ParseTaxonomy() unexpected error: %v", err)
	}

	q02, ok := tax.QuestionByQID("Q02")
	if !ok {
		t.Fatal("QuestionByQID(Q02) returned false")
	}
	if len(q02.Options) != 2 {
		t.Errorf("Q02: len(Options) = %d, want 2", len(q02.Options))
	}
	if q02.Options[0].Value != "claude-code" {
		t.Errorf("Q02: Options[0].Value = %q, want %q", q02.Options[0].Value, "claude-code")
	}

	// Q04 has an option with a note
	q04, ok := tax.QuestionByQID("Q04")
	if !ok {
		t.Fatal("QuestionByQID(Q04) returned false")
	}
	if q04.Options[0].Note == "" {
		t.Error("Q04: Options[0].Note is empty, expected a note")
	}
}

func TestParseTaxonomy_ErrorOnMissingVersion(t *testing.T) {
	bad := strings.ReplaceAll(v2Fixture, `version: "1.0"`, "")
	_, err := wizard.ParseTaxonomy([]byte(bad))
	if err == nil {
		t.Error("ParseTaxonomy() expected error for missing version, got nil")
	}
}

func TestParseTaxonomy_ErrorOnNoPhases(t *testing.T) {
	bad := `version: "1.0"
phases: []`
	_, err := wizard.ParseTaxonomy([]byte(bad))
	if err == nil {
		t.Error("ParseTaxonomy() expected error for empty phases, got nil")
	}
}

func TestParseTaxonomy_ErrorOnMissingQID(t *testing.T) {
	bad := `version: "1.0"
phases:
  - id: P1
    title: Identity
    mandatory: true
    questions:
      - prompt: A question without a qid
        time: ~5s
`
	_, err := wizard.ParseTaxonomy([]byte(bad))
	if err == nil {
		t.Error("ParseTaxonomy() expected error for missing qid, got nil")
	}
}

func TestParseTaxonomy_ErrorOnDuplicateQID(t *testing.T) {
	bad := `version: "1.0"
phases:
  - id: P1
    title: Identity
    mandatory: true
    questions:
      - qid: Q01
        prompt: First question
        time: ~5s
  - id: P2
    title: Domains
    mandatory: true
    questions:
      - qid: Q01
        prompt: Duplicate qid
        time: ~5s
`
	_, err := wizard.ParseTaxonomy([]byte(bad))
	if err == nil {
		t.Error("ParseTaxonomy() expected error for duplicate qid, got nil")
	}
}

func TestParseTaxonomy_ErrorOnMissingPhaseID(t *testing.T) {
	bad := `version: "1.0"
phases:
  - title: No ID Phase
    mandatory: true
    questions:
      - qid: Q01
        prompt: Some question
        time: ~5s
`
	_, err := wizard.ParseTaxonomy([]byte(bad))
	if err == nil {
		t.Error("ParseTaxonomy() expected error for missing phase id, got nil")
	}
}
