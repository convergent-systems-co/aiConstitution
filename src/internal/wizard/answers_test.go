package wizard_test

import (
	"strings"
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

// TestAnswersToAnswerSet_Q07DomainPicker verifies that the Q07 answer drives
// the Domains slice correctly for each valid option value.
func TestAnswersToAnswerSet_Q07DomainPicker(t *testing.T) {
	cases := []struct {
		q07          string
		wantLen      int
		wantTemplate string // template of the first domain
		wantSection  int    // SectionNum of the first domain
	}{
		{q07: "code", wantLen: 1, wantTemplate: "technical", wantSection: 4},
		{q07: "writing", wantLen: 1, wantTemplate: "prose", wantSection: 4},
		{q07: "both", wantLen: 2, wantTemplate: "technical", wantSection: 4},
		{q07: "other", wantLen: 2, wantTemplate: "technical", wantSection: 4},
		{q07: "", wantLen: 2, wantTemplate: "technical", wantSection: 4},
	}

	for _, tc := range cases {
		tc := tc
		t.Run("Q07="+tc.q07, func(t *testing.T) {
			as, err := wizard.AnswersToAnswerSet(map[string]string{
				"Q01": "Tester",
				"Q07": tc.q07,
			})
			if err != nil {
				t.Fatalf("AnswersToAnswerSet() error: %v", err)
			}
			if len(as.Domains) != tc.wantLen {
				t.Errorf("len(Domains) = %d, want %d", len(as.Domains), tc.wantLen)
			}
			if len(as.Domains) > 0 {
				if as.Domains[0].Template != tc.wantTemplate {
					t.Errorf("Domains[0].Template = %q, want %q", as.Domains[0].Template, tc.wantTemplate)
				}
				if as.Domains[0].SectionNum != tc.wantSection {
					t.Errorf("Domains[0].SectionNum = %d, want %d", as.Domains[0].SectionNum, tc.wantSection)
				}
			}
			// "both" default: verify second domain is prose at §5
			if tc.wantLen == 2 && len(as.Domains) == 2 {
				if as.Domains[1].Template != "prose" {
					t.Errorf("Domains[1].Template = %q, want %q", as.Domains[1].Template, "prose")
				}
				if as.Domains[1].SectionNum != 5 {
					t.Errorf("Domains[1].SectionNum = %d, want 5", as.Domains[1].SectionNum)
				}
			}
		})
	}
}

// TestAnswersToAnswerSet_Q07_TechnicalRendering verifies that selecting "code"
// produces a constitution that contains the technical domain section and does
// not contain the prose domain section.
func TestAnswersToAnswerSet_Q07_TechnicalRendering(t *testing.T) {
	// Minimal constitution template exercising domain rendering.
	tmpl := `{{range .Domains}}DOMAIN:{{.Template}}:{{.SectionNum}}
{{end}}`

	as, err := wizard.AnswersToAnswerSet(map[string]string{
		"Q01": "Tester",
		"Q07": "code",
	})
	if err != nil {
		t.Fatalf("AnswersToAnswerSet() error: %v", err)
	}
	rendered, err := constitution.Render(as, tmpl)
	if err != nil {
		t.Fatalf("Render() error: %v", err)
	}

	if !strings.Contains(rendered, "DOMAIN:technical:4") {
		t.Errorf("expected technical domain at §4 in rendered output, got:\n%s", rendered)
	}
	if strings.Contains(rendered, "DOMAIN:prose") {
		t.Errorf("expected no prose domain in 'code'-only output, got:\n%s", rendered)
	}
}

// TestAnswersToAnswerSet_Q07_ProseRendering verifies that selecting "writing"
// produces a constitution that contains the prose domain section and does not
// contain the technical domain section.
func TestAnswersToAnswerSet_Q07_ProseRendering(t *testing.T) {
	tmpl := `{{range .Domains}}DOMAIN:{{.Template}}:{{.SectionNum}}
{{end}}`

	as, err := wizard.AnswersToAnswerSet(map[string]string{
		"Q01": "Tester",
		"Q07": "writing",
	})
	if err != nil {
		t.Fatalf("AnswersToAnswerSet() error: %v", err)
	}
	rendered, err := constitution.Render(as, tmpl)
	if err != nil {
		t.Fatalf("Render() error: %v", err)
	}

	if !strings.Contains(rendered, "DOMAIN:prose:4") {
		t.Errorf("expected prose domain at §4 in rendered output, got:\n%s", rendered)
	}
	if strings.Contains(rendered, "DOMAIN:technical") {
		t.Errorf("expected no technical domain in 'writing'-only output, got:\n%s", rendered)
	}
}

// Ensure constitution.Domain is reachable from wizard package (import check)
var _ constitution.Domain
