package wizard_test

import (
	"testing"

	"github.com/convergent-systems-co/aiConstitution/src/internal/constitution"
	"github.com/convergent-systems-co/aiConstitution/src/internal/wizard"
)

func TestAnswersToAnswerSet_BasicMapping(t *testing.T) {
	answers := map[string]string{
		"Q01": "Alice",
		"Q02": "claude-code,copilot-cli",
		"Q03": "Test Org",
		"Q04": "technical,prose",
		"Q05": "- Tests MUST be red first.",
		"Q06": "$5",
		"Q07": "100",
		"Q08": "main",
		"Q09": "autonomous",
		"Q10": "flag-once",
		"Q11": "match-complexity",
		"Q12": "direct-framing",
		"Q13": "true",
	}

	as, err := wizard.AnswersToAnswerSet(answers)
	if err != nil {
		t.Fatalf("AnswersToAnswerSet() error: %v", err)
	}
	if as.Principal != "Alice" {
		t.Errorf("Principal = %q, want %q", as.Principal, "Alice")
	}
	if len(as.Tools) != 2 {
		t.Errorf("len(Tools) = %d, want 2", len(as.Tools))
	}
	if len(as.Domains) != 2 {
		t.Errorf("len(Domains) = %d, want 2", len(as.Domains))
	}
	if as.Domains[0].Template != "technical" {
		t.Errorf("Domains[0].Template = %q, want %q", as.Domains[0].Template, "technical")
	}
	if as.Domains[0].SectionNum != 4 {
		t.Errorf("Domains[0].SectionNum = %d, want 4", as.Domains[0].SectionNum)
	}
	if as.Domains[1].SectionNum != 5 {
		t.Errorf("Domains[1].SectionNum = %d, want 5", as.Domains[1].SectionNum)
	}
	if !as.ProvenanceInCommits {
		t.Error("ProvenanceInCommits should be true")
	}
	if as.CostCeiling != "$5" {
		t.Errorf("CostCeiling = %q, want %q", as.CostCeiling, "$5")
	}
	if as.BlastRadius != 100 {
		t.Errorf("BlastRadius = %d, want 100", as.BlastRadius)
	}
}

func TestAnswersToAnswerSet_MissingPrincipal_DefaultsToReference(t *testing.T) {
	// v2.1 reference-first: missing Q01 defaults to 'Principal', never errors.
	as, err := wizard.AnswersToAnswerSet(map[string]string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if as.Principal != "Principal" {
		t.Errorf("Principal = %q, want %q", as.Principal, "Principal")
	}
}

func TestAnswersToAnswerSet_Defaults(t *testing.T) {
	// Only Q01 provided — all others should use defaults
	as, err := wizard.AnswersToAnswerSet(map[string]string{"Q01": "Bob"})
	if err != nil {
		t.Fatalf("AnswersToAnswerSet() error: %v", err)
	}
	if as.Principal != "Bob" {
		t.Errorf("Principal = %q", as.Principal)
	}
	if as.CostCeiling != "$5" {
		t.Errorf("CostCeiling default = %q, want $5", as.CostCeiling)
	}
	if as.BlastRadius != 100 {
		t.Errorf("BlastRadius default = %d, want 100", as.BlastRadius)
	}
	if len(as.ProtectedBranches) == 0 || as.ProtectedBranches[0] != "main" {
		t.Errorf("ProtectedBranches default = %v", as.ProtectedBranches)
	}
	if as.PushbackPersistence != "flag-once" {
		t.Errorf("PushbackPersistence default = %q", as.PushbackPersistence)
	}
}

// Ensure constitution.Domain is reachable from wizard package (import check)
var _ constitution.Domain
