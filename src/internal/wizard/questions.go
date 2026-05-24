// Package wizard implements the question taxonomy parser and non-interactive
// runner for ai setup.
package wizard

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// QuestionType enumerates the supported input types.
type QuestionType string

// Supported question types per spec §13.2.
const (
	TypeText        QuestionType = "text"
	TypeConfirm     QuestionType = "confirm"
	TypeSelect      QuestionType = "select"
	TypeMultiSelect QuestionType = "multi-select"
)

// Dependency describes a conditional dependency on a prior answer.
type Dependency struct {
	ID    string `yaml:"id"`
	Value string `yaml:"value"`
}

// Option is a single selectable answer in a wizard question.
type Option struct {
	Label   string `yaml:"label"`
	Value   string `yaml:"value"`
	Note    string `yaml:"note,omitempty"`
	Warning string `yaml:"warning,omitempty"`
}

// Question is a single wizard question parsed from questions.yaml.
type Question struct {
	ID       string       `yaml:"id"`
	QID      string       `yaml:"qid"`      // alternative field name used in phases[].questions[]
	Category string       `yaml:"category"`
	Type     QuestionType `yaml:"type"`
	Prompt   string       `yaml:"prompt"`
	Options  []Option     `yaml:"options,omitempty"`
	Default  string       `yaml:"default,omitempty"`
	Required bool         `yaml:"required"`
	Depends  *Dependency  `yaml:"depends,omitempty"`
}

// Phase groups questions under a category heading in questions.yaml.
// The canonical questions.yaml uses phases[].questions[] with qid: keys
// rather than a flat questions[] list with id: keys.
type Phase struct {
	ID        string     `yaml:"id"`
	Questions []Question `yaml:"questions"`
}

// Taxonomy is the parsed questions.yaml file.
type Taxonomy struct {
	Version   string     `yaml:"version"`
	Questions []Question `yaml:"questions"` // flat list OR flattened from Phases
	Phases    []Phase    `yaml:"phases"`    // phase-grouped questions (canonical schema)
}

// ParseTaxonomy decodes a questions.yaml byte slice into a Taxonomy.
// It handles both the flat questions[] format and the phases[].questions[]
// format used by the canonical governance/wizard/questions.yaml. When the
// phases format is detected, questions are flattened into Taxonomy.Questions
// and qid: field values are normalized to id:.
func ParseTaxonomy(data []byte) (Taxonomy, error) {
	var t Taxonomy
	if err := yaml.Unmarshal(data, &t); err != nil {
		return Taxonomy{}, fmt.Errorf("wizard: parse taxonomy: %w", err)
	}

	// Normalize qid → id for questions already in the flat list.
	for i := range t.Questions {
		if t.Questions[i].ID == "" && t.Questions[i].QID != "" {
			t.Questions[i].ID = t.Questions[i].QID
		}
	}

	// If questions.yaml uses phases[].questions[] format, flatten into t.Questions.
	if len(t.Questions) == 0 && len(t.Phases) > 0 {
		for pi := range t.Phases {
			for qi := range t.Phases[pi].Questions {
				q := t.Phases[pi].Questions[qi]
				// Normalize qid → id within each phase.
				if q.ID == "" && q.QID != "" {
					q.ID = q.QID
				}
				t.Questions = append(t.Questions, q)
			}
		}
	}

	return t, nil
}

// ActiveQuestions returns questions whose dependency (if any) is
// satisfied by the current answers map.
func (t Taxonomy) ActiveQuestions(answers map[string]string) []Question {
	active := make([]Question, 0, len(t.Questions))
	for _, q := range t.Questions {
		if q.Depends == nil {
			active = append(active, q)
			continue
		}
		if answers[q.Depends.ID] == q.Depends.Value {
			active = append(active, q)
		}
	}
	return active
}
