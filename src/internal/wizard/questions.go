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

// Question is a single wizard question parsed from questions.yaml.
type Question struct {
	ID       string       `yaml:"id"`
	Category string       `yaml:"category"`
	Type     QuestionType `yaml:"type"`
	Prompt   string       `yaml:"prompt"`
	Options  []string     `yaml:"options,omitempty"`
	Default  string       `yaml:"default,omitempty"`
	Required bool         `yaml:"required"`
	Depends  *Dependency  `yaml:"depends,omitempty"`
}

// Taxonomy is the parsed questions.yaml file.
type Taxonomy struct {
	Version   string     `yaml:"version"`
	Questions []Question `yaml:"questions"`
}

// ParseTaxonomy decodes a questions.yaml byte slice into a Taxonomy.
func ParseTaxonomy(data []byte) (Taxonomy, error) {
	var t Taxonomy
	if err := yaml.Unmarshal(data, &t); err != nil {
		return Taxonomy{}, fmt.Errorf("wizard: parse taxonomy: %w", err)
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
