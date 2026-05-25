// Package wizard parses the questions.yaml taxonomy that drives the
// `ai setup` / `ai --tui` wizard flow.
//
// The taxonomy defines a linear sequence of phases; each phase contains
// one or more questions. Parsing is intentionally strict: unknown top-level
// keys are ignored (forward-compatible), but a missing required field causes
// a descriptive error so schema drift is caught at load time rather than at
// display time.
//
// The canonical questions.yaml is embedded in the binary via the embed
// package (cmd/ai/embed); callers that need the embedded copy should call
// embed.QuestionsYAML() and pass the result to ParseTaxonomy.
package wizard

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// Taxonomy is the top-level parsed representation of questions.yaml.
type Taxonomy struct {
	// Version is the semver string from the "version:" key (e.g. "1.0").
	// Bump on any breaking change to qid semantics or option values.
	Version string `yaml:"version"`

	// Phases is the ordered list of wizard phases. Each phase is presented
	// in sequence; all phases marked mandatory:true fire regardless of
	// prior answers.
	Phases []Phase `yaml:"phases"`
}

// Phase groups a set of related questions under a single screen banner.
type Phase struct {
	// ID is the stable phase identifier (P1..P5). Never reused after
	// retirement.
	ID string `yaml:"id"`

	// Title is the banner text shown at the top of the phase screen.
	Title string `yaml:"title"`

	// Mandatory, when true, means this phase fires for every user
	// regardless of prior answers.
	Mandatory bool `yaml:"mandatory"`

	// Questions is the ordered list of questions within the phase.
	Questions []Question `yaml:"questions"`
}

// Question is a single wizard prompt with its display metadata and
// persistence contract.
type Question struct {
	// QID is the stable question identifier (Q01, Q14, …). A retired QID
	// is reserved and never reused; this ensures answers.yaml remains
	// diffable across taxonomy versions.
	QID string `yaml:"qid"`

	// Prompt is the question text shown on screen.
	Prompt string `yaml:"prompt"`

	// Framing is 1–3 sentences of context shown above the options list.
	// Optional.
	Framing string `yaml:"framing"`

	// Time is a human-readable estimate of how long the question takes to
	// answer (e.g. "~5s", "~3m"). Display only; not machine-interpreted.
	Time string `yaml:"time"`

	// Options is the ordered list of selectable answers. Empty for
	// free-text-only questions.
	Options []Option `yaml:"options"`

	// AllowFreeText, when true, shows a "Type my own" escape hatch below
	// the option list.
	AllowFreeText bool `yaml:"allow_free_text"`

	// AllowChat, when true, shows a "Chat with the assistant" escape hatch.
	AllowChat bool `yaml:"allow_chat"`

	// AllowDefer, when true, shows a "Decide later" escape hatch.
	AllowDefer bool `yaml:"allow_defer"`

	// Informational, when true, means the question has exactly one
	// option (an acknowledgment) and is non-overridable.
	Informational bool `yaml:"informational"`

	// Default is the value used when the user accepts the default without
	// selecting an option explicitly.
	Default string `yaml:"default"`

	// PeristsTo names the canonical file the answer shapes (e.g.
	// "Constitution.md", "branch-guard.json").
	PeristsTo string `yaml:"persists_to"`

	// PersistsToSection is the file-relative section reference (e.g.
	// "§3.2 + branch-guard.json").
	PersistsToSection string `yaml:"persists_to_section"`
}

// Option is one selectable answer within a question.
type Option struct {
	// Label is the short human-readable label shown in the TUI.
	Label string `yaml:"label"`

	// Value is the machine value persisted to answers.yaml.
	Value string `yaml:"value"`

	// Note is an optional inline gloss shown next to the label.
	Note string `yaml:"note"`

	// Warning is an optional warning string shown when this option is
	// selected (e.g. "Removes a safety gate."). Empty means no warning.
	Warning string `yaml:"warning"`
}

// ParseTaxonomy decodes src (the raw bytes of questions.yaml) into a
// Taxonomy. It returns an error if the YAML is malformed or if the
// decoded document is semantically invalid (empty version, no phases,
// questions without a qid).
func ParseTaxonomy(src []byte) (*Taxonomy, error) {
	var t Taxonomy
	if err := yaml.Unmarshal(src, &t); err != nil {
		return nil, fmt.Errorf("wizard: parse questions.yaml: %w", err)
	}
	if err := validate(&t); err != nil {
		return nil, err
	}
	return &t, nil
}

// validate performs semantic checks on a freshly decoded Taxonomy.
func validate(t *Taxonomy) error {
	if t.Version == "" {
		return fmt.Errorf("wizard: questions.yaml missing required field: version")
	}
	if len(t.Phases) == 0 {
		return fmt.Errorf("wizard: questions.yaml has no phases")
	}
	seen := make(map[string]struct{})
	for pi, p := range t.Phases {
		if p.ID == "" {
			return fmt.Errorf("wizard: phase[%d] missing required field: id", pi)
		}
		for qi, q := range p.Questions {
			if q.QID == "" {
				return fmt.Errorf("wizard: phase %s question[%d] missing required field: qid", p.ID, qi)
			}
			if _, dup := seen[q.QID]; dup {
				return fmt.Errorf("wizard: duplicate qid %q in phase %s", q.QID, p.ID)
			}
			seen[q.QID] = struct{}{}
		}
	}
	return nil
}

// QuestionCount returns the total number of questions across all phases.
func (t *Taxonomy) QuestionCount() int {
	n := 0
	for _, p := range t.Phases {
		n += len(p.Questions)
	}
	return n
}

// PhaseByID returns the Phase with the given ID and true, or the zero
// value and false if no such phase exists.
func (t *Taxonomy) PhaseByID(id string) (Phase, bool) {
	for _, p := range t.Phases {
		if p.ID == id {
			return p, true
		}
	}
	return Phase{}, false
}

// QuestionByQID searches all phases for a question with the given QID.
// Returns the question and true if found, or the zero value and false.
func (t *Taxonomy) QuestionByQID(qid string) (Question, bool) {
	for _, p := range t.Phases {
		for _, q := range p.Questions {
			if q.QID == qid {
				return q, true
			}
		}
	}
	return Question{}, false
}

// ActiveQuestions returns all questions across all phases. The answers
// parameter is accepted for API compatibility but unused in v2 (all questions
// are mandatory and have no dependencies).
func (t *Taxonomy) ActiveQuestions(_ map[string]string) []Question {
	var out []Question
	for _, phase := range t.Phases {
		out = append(out, phase.Questions...)
	}
	return out
}

// QuestionType classifies the input mechanism for a question.
// The v2 schema infers type from Options/AllowFreeText/AllowChat fields,
// but the TUI requires an explicit type for rendering.
type QuestionType string

const (
	TypeText        QuestionType = "text"
	TypeConfirm     QuestionType = "confirm"
	TypeSelect      QuestionType = "select"
	TypeMultiSelect QuestionType = "multiselect"
)

// Type returns the inferred question type based on options and flags.
func (q Question) Type() QuestionType {
	if len(q.Options) > 1 {
		if q.AllowFreeText {
			return TypeMultiSelect
		}
		return TypeSelect
	}
	if q.Informational {
		return TypeConfirm
	}
	return TypeText
}

// Required returns true when the question has no default value and must
// be explicitly answered. Used by TUI rendering for validation.
func (q Question) Required() bool {
	return q.Default == "" && !q.AllowDefer
}
