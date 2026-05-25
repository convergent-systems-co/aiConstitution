package wizard_test

import (
	"strings"
	"testing"

	internalwizard "github.com/convergent-systems-co/aiConstitution/src/internal/wizard"

	"github.com/convergent-systems-co/aiConstitution/src/cmd/ai/internal/wizard"
)

// TestRenderTextShowsPrompt verifies that the text renderer includes the
// question prompt and a text-input cue.
func TestRenderTextShowsPrompt(t *testing.T) {
	q := internalwizard.Question{
		ID:     "name",
		Prompt: "What is your name?",
		Type:   internalwizard.TypeText,
	}
	got := wizard.RenderQuestion(wizard.NewModel(internalwizard.Taxonomy{Questions: []internalwizard.Question{q}}), q)
	if !strings.Contains(got, "What is your name?") {
		t.Errorf("RenderText: output does not contain prompt; got:\n%s", got)
	}
}

// TestRenderSelectShowsOptions verifies that the select renderer lists all
// option labels and includes a cursor marker.
func TestRenderSelectShowsOptions(t *testing.T) {
	q := internalwizard.Question{
		ID:     "color",
		Prompt: "Pick a color?",
		Type:   internalwizard.TypeSelect,
		Options: []internalwizard.Option{
			{Label: "Red", Value: "red"},
			{Label: "Blue", Value: "blue"},
			{Label: "Green", Value: "green"},
		},
	}
	m := wizard.NewModel(internalwizard.Taxonomy{Questions: []internalwizard.Question{q}})
	got := wizard.RenderQuestion(m, q)
	for _, label := range []string{"Red", "Blue", "Green"} {
		if !strings.Contains(got, label) {
			t.Errorf("RenderSelect: output missing %q; got:\n%s", label, got)
		}
	}
	// At least one cursor indicator should be present.
	if !strings.Contains(got, ">") && !strings.Contains(got, "→") && !strings.Contains(got, "▶") {
		t.Errorf("RenderSelect: no cursor indicator found; got:\n%s", got)
	}
}

// TestRenderMultiSelectShowsCheckboxes verifies that multi-select renders
// checkboxes for each option.
func TestRenderMultiSelectShowsCheckboxes(t *testing.T) {
	q := internalwizard.Question{
		ID:     "tools",
		Prompt: "Select tools?",
		Type:   internalwizard.TypeMultiSelect,
		Options: []internalwizard.Option{
			{Label: "git", Value: "git"},
			{Label: "gh", Value: "gh"},
		},
	}
	m := wizard.NewModel(internalwizard.Taxonomy{Questions: []internalwizard.Question{q}})
	got := wizard.RenderQuestion(m, q)
	// Should contain checkbox indicators ([ ] or similar).
	if !strings.Contains(got, "[") {
		t.Errorf("RenderMultiSelect: no checkbox-like syntax found; got:\n%s", got)
	}
	if !strings.Contains(got, "git") || !strings.Contains(got, "gh") {
		t.Errorf("RenderMultiSelect: option labels missing; got:\n%s", got)
	}
}

// TestRenderConfirmShowsYNPrompt verifies that the confirm renderer includes
// a y/n cue.
func TestRenderConfirmShowsYNPrompt(t *testing.T) {
	q := internalwizard.Question{
		ID:     "ok",
		Prompt: "Are you sure?",
		Type:   internalwizard.TypeConfirm,
	}
	m := wizard.NewModel(internalwizard.Taxonomy{Questions: []internalwizard.Question{q}})
	got := wizard.RenderQuestion(m, q)
	lower := strings.ToLower(got)
	if !strings.Contains(lower, "y") || !strings.Contains(lower, "n") {
		t.Errorf("RenderConfirm: output does not contain y/n cue; got:\n%s", got)
	}
}
