package constitution

import (
	"bytes"
	"text/template"
)

type AnswerSet struct {
	Principal           string
	Tools               []string
	WorkContext         string
	CostCeiling         string
	BlastRadius         int
	ProtectedBranches   []string
	AutonomyPosture     string
	PushbackPersistence string
	ResponseLength      string
	DisagreementTone    string
	ProvenanceInCommits bool
	Domains             []Domain
}

type Domain struct {
	Name          string
	SectionNum    int
	Preamble      string
	PersonalRules string
	Template      string
}

func Render(as AnswerSet, tmplSrc string) (string, error) {
	t, err := template.New("constitution").Parse(tmplSrc)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, as); err != nil {
		return "", err
	}
	return buf.String(), nil
}
