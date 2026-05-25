package constitution_test

import (
	"strings"
	"testing"

	"github.com/convergent-systems-co/aiConstitution/src/cmd/ai/embed"
	"github.com/convergent-systems-co/aiConstitution/src/internal/constitution"
)

func TestRender_ProducesSections(t *testing.T) {
	as := constitution.AnswerSet{
		Principal: "Alice",
		Tools:     []string{"Claude Code"},
		Domains: []constitution.Domain{
			{Name: "Technical Work", SectionNum: 4, Preamble: "Governs code.", PersonalRules: "- Tests MUST be red first.", Template: "technical"},
		},
		CostCeiling:         "$5",
		BlastRadius:         100,
		ProtectedBranches:   []string{"main"},
		PushbackPersistence: "flag-once",
		ResponseLength:      "match-complexity",
		DisagreementTone:    "direct-framing",
	}
	out, err := constitution.Render(as, minimalTmpl)
	if err != nil {
		t.Fatalf("Render() error: %v", err)
	}
	for _, want := range []string{"Alice", "Claude Code", "Technical Work", "$5", "main"} {
		if !strings.Contains(out, want) {
			t.Errorf("Render() output missing %q", want)
		}
	}
}

func TestRender_WithEmbeddedTemplate(t *testing.T) {
	tmplBytes, err := embed.ConstitutionTemplate()
	if err != nil {
		t.Fatalf("ConstitutionTemplate(): %v", err)
	}
	as := constitution.AnswerSet{
		Principal:           "Test User",
		Tools:               []string{"Claude Code"},
		WorkContext:         "Test Org",
		CostCeiling:         "$5",
		BlastRadius:         100,
		ProtectedBranches:   []string{"main"},
		PushbackPersistence: "flag-once",
		ResponseLength:      "match-complexity",
		DisagreementTone:    "direct-framing",
		ProvenanceInCommits: true,
		Domains: []constitution.Domain{
			{Name: "Technical Work", SectionNum: 4, Preamble: "Governs code.", PersonalRules: "- Tests MUST be red first.", Template: "technical"},
		},
	}
	out, err := constitution.Render(as, string(tmplBytes))
	if err != nil {
		t.Fatalf("Render() error: %v", err)
	}
	for _, want := range []string{"Test User", "Claude Code", "§1", "§2 Behavioral", "§3", "§4 Technical Work"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in rendered output", want)
		}
	}
}

const minimalTmpl = `# AI Constitution
Principal: {{.Principal}}
Tools: {{range .Tools}}{{.}} {{end}}
Cost ceiling: {{.CostCeiling}}
Protected: {{range .ProtectedBranches}}{{.}} {{end}}
Pushback: {{.PushbackPersistence}}
{{range .Domains}}
## §{{.SectionNum}} {{.Name}}
{{.Preamble}}
{{.PersonalRules}}
{{end}}`
