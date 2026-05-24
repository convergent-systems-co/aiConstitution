package wizard_test

import (
	"testing"

	"github.com/convergent-systems-co/aiConstitution/src/internal/wizard"
)

const v2TestTaxonomy = `
version: "1.0"
phases:
  - id: P1
    title: Identity
    mandatory: true
    questions:
      - qid: Q01
        prompt: "Your name?"
        default: "DefaultUser"
      - qid: Q02
        prompt: "Your tools?"
        default: "claude-code"
`

func TestNonInteractiveRunnerUsesSeededAnswers(t *testing.T) {
	tax, err := wizard.ParseTaxonomy([]byte(v2TestTaxonomy))
	if err != nil {
		t.Fatalf("ParseTaxonomy() error = %v", err)
	}
	seeds := map[string]string{"Q01": "Bob"}
	answers, err := wizard.RunNonInteractive(*tax, seeds)
	if err != nil {
		t.Fatalf("RunNonInteractive() error = %v", err)
	}
	if answers["Q01"] != "Bob" {
		t.Errorf("Q01 = %q, want %q", answers["Q01"], "Bob")
	}
	// Q02 not seeded — should use default
	if answers["Q02"] != "claude-code" {
		t.Errorf("Q02 default = %q, want %q", answers["Q02"], "claude-code")
	}
}

func TestNonInteractiveRunnerUsesDefault(t *testing.T) {
	tax, err := wizard.ParseTaxonomy([]byte(v2TestTaxonomy))
	if err != nil {
		t.Fatalf("ParseTaxonomy() error = %v", err)
	}
	answers, err := wizard.RunNonInteractive(*tax, nil)
	if err != nil {
		t.Fatalf("RunNonInteractive() error = %v", err)
	}
	if answers["Q01"] != "DefaultUser" {
		t.Errorf("Q01 default = %q, want %q", answers["Q01"], "DefaultUser")
	}
}
