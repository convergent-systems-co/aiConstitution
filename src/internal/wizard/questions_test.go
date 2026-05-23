package wizard_test

import (
	"testing"

	"github.com/convergent-systems-co/aiConstitution/src/internal/wizard"
)

const sampleTaxonomy = `
version: "0.2"
questions:
  - id: user_name
    category: identity
    type: text
    prompt: "What is your full name?"
    required: true
  - id: has_org
    category: identity
    type: confirm
    prompt: "Do you work within an organization?"
    required: false
  - id: org_name
    category: identity
    type: text
    prompt: "What is your organization name?"
    required: true
    depends:
      id: has_org
      value: "true"
`

func TestParseTaxonomyParsesQuestions(t *testing.T) {
	tax, err := wizard.ParseTaxonomy([]byte(sampleTaxonomy))
	if err != nil {
		t.Fatalf("ParseTaxonomy() error = %v", err)
	}
	if tax.Version != "0.2" {
		t.Errorf("Version = %q, want %q", tax.Version, "0.2")
	}
	if len(tax.Questions) != 3 {
		t.Errorf("len(Questions) = %d, want 3", len(tax.Questions))
	}
}

const phasedTaxonomy = `
version: "0.3"
phases:
  - id: identity
    questions:
      - qid: user_name
        category: identity
        type: text
        prompt: "What is your full name?"
        required: true
      - qid: has_org
        category: identity
        type: confirm
        prompt: "Do you work within an organization?"
        required: false
`

func TestParseTaxonomyHandlesPhasedFormat(t *testing.T) {
	tax, err := wizard.ParseTaxonomy([]byte(phasedTaxonomy))
	if err != nil {
		t.Fatalf("ParseTaxonomy() error = %v", err)
	}
	if len(tax.Questions) != 2 {
		t.Errorf("len(Questions) = %d, want 2 (flattened from phases)", len(tax.Questions))
	}
	if tax.Questions[0].ID != "user_name" {
		t.Errorf("Questions[0].ID = %q, want %q", tax.Questions[0].ID, "user_name")
	}
}

func TestActiveQuestionsSkipsUnsatisfiedDependency(t *testing.T) {
	tax, _ := wizard.ParseTaxonomy([]byte(sampleTaxonomy))
	answers := map[string]string{
		"user_name": "Alice",
		"has_org":   "false",
	}
	active := tax.ActiveQuestions(answers)
	for _, q := range active {
		if q.ID == "org_name" {
			t.Error("org_name should be inactive when has_org=false")
		}
	}
}

func TestActiveQuestionsIncludesSatisfiedDependency(t *testing.T) {
	tax, _ := wizard.ParseTaxonomy([]byte(sampleTaxonomy))
	answers := map[string]string{
		"user_name": "Alice",
		"has_org":   "true",
	}
	active := tax.ActiveQuestions(answers)
	found := false
	for _, q := range active {
		if q.ID == "org_name" {
			found = true
		}
	}
	if !found {
		t.Error("org_name should be active when has_org=true")
	}
}
