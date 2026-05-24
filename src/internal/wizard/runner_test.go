package wizard_test

import (
	"testing"

	"github.com/convergent-systems-co/aiConstitution/src/internal/wizard"
)

func TestNonInteractiveRunnerUsesSeededAnswers(t *testing.T) {
	tax, _ := wizard.ParseTaxonomy([]byte(sampleTaxonomy))
	seeds := map[string]string{
		"user_name": "Bob",
		"has_org":   "true",
		"org_name":  "Acme",
	}
	answers, err := wizard.RunNonInteractive(tax, seeds)
	if err != nil {
		t.Fatalf("RunNonInteractive() error = %v", err)
	}
	if answers["user_name"] != "Bob" {
		t.Errorf("user_name = %q, want %q", answers["user_name"], "Bob")
	}
	if answers["org_name"] != "Acme" {
		t.Errorf("org_name = %q, want %q", answers["org_name"], "Acme")
	}
}

func TestNonInteractiveRunnerErrorsOnMissingRequired(t *testing.T) {
	tax, _ := wizard.ParseTaxonomy([]byte(sampleTaxonomy))
	seeds := map[string]string{
		// user_name is required but missing
		"has_org": "false",
	}
	_, err := wizard.RunNonInteractive(tax, seeds)
	if err == nil {
		t.Fatal("expected error for missing required question, got nil")
	}
}

func TestNonInteractiveRunnerUsesDefault(t *testing.T) {
	const taxWithDefault = `
version: "0.2"
questions:
  - id: color
    category: prefs
    type: text
    prompt: "Favourite color?"
    default: "blue"
    required: false
`
	tax, _ := wizard.ParseTaxonomy([]byte(taxWithDefault))
	answers, err := wizard.RunNonInteractive(tax, nil)
	if err != nil {
		t.Fatalf("RunNonInteractive() error = %v", err)
	}
	if answers["color"] != "blue" {
		t.Errorf("color = %q, want %q (default)", answers["color"], "blue")
	}
}
