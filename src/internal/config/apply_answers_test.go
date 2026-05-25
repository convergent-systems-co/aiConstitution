package config_test

import (
	"testing"

	"github.com/convergent-systems-co/aiConstitution/src/internal/config"
)

func TestApplyAnswersMapsDefaultMode(t *testing.T) {
	s := config.Defaults()
	config.ApplyAnswers(&s, map[string]string{"defaultMode": "writer"})
	if s.Focus.DefaultMode != "writer" {
		t.Errorf("Focus.DefaultMode = %q, want %q", s.Focus.DefaultMode, "writer")
	}
}

func TestApplyAnswersParsesShareNewHooksBool(t *testing.T) {
	s := config.Defaults()
	config.ApplyAnswers(&s, map[string]string{"shareNewHooks": "false"})
	if s.Upstream.ShareNewHooks != false {
		t.Errorf("Upstream.ShareNewHooks = %v, want false", s.Upstream.ShareNewHooks)
	}

	s = config.Defaults()
	config.ApplyAnswers(&s, map[string]string{"shareNewHooks": "yes"})
	if s.Upstream.ShareNewHooks != true {
		t.Errorf("Upstream.ShareNewHooks = %v, want true (from 'yes')", s.Upstream.ShareNewHooks)
	}
}

func TestApplyAnswersParsesReviewCadenceDaysInt(t *testing.T) {
	s := config.Defaults()
	config.ApplyAnswers(&s, map[string]string{"reviewCadenceDays": "14"})
	if s.Review.CadenceDays != 14 {
		t.Errorf("Review.CadenceDays = %d, want 14", s.Review.CadenceDays)
	}
}

func TestApplyAnswersParsesSyncIncludeSettingsBool(t *testing.T) {
	s := config.Defaults()
	config.ApplyAnswers(&s, map[string]string{"syncIncludeSettings": "false"})
	if s.Sync.IncludeSettingsFile != false {
		t.Errorf("Sync.IncludeSettingsFile = %v, want false", s.Sync.IncludeSettingsFile)
	}
}

func TestApplyAnswersIgnoresUnknownKeys(t *testing.T) {
	s := config.Defaults()
	// Should not error or panic.
	config.ApplyAnswers(&s, map[string]string{"nonsenseKey": "whatever"})
	if s.SchemaVersion != config.Defaults().SchemaVersion {
		t.Error("ApplyAnswers mutated unrelated field for unknown key")
	}
}

func TestApplyAnswersIgnoresParseErrors(t *testing.T) {
	s := config.Defaults()
	originalCadence := s.Review.CadenceDays
	config.ApplyAnswers(&s, map[string]string{"reviewCadenceDays": "not-a-number"})
	if s.Review.CadenceDays != originalCadence {
		t.Errorf("Review.CadenceDays changed despite parse error: %d", s.Review.CadenceDays)
	}
}

func TestApplyAnswersAppliesMultipleKeys(t *testing.T) {
	s := config.Defaults()
	config.ApplyAnswers(&s, map[string]string{
		"defaultMode":         "developer",
		"shareNewHooks":       "false",
		"reviewCadenceDays":   "7",
		"syncIncludeSettings": "true",
	})
	if s.Focus.DefaultMode != "developer" {
		t.Errorf("Focus.DefaultMode = %q", s.Focus.DefaultMode)
	}
	if s.Upstream.ShareNewHooks != false {
		t.Errorf("Upstream.ShareNewHooks = %v", s.Upstream.ShareNewHooks)
	}
	if s.Review.CadenceDays != 7 {
		t.Errorf("Review.CadenceDays = %d", s.Review.CadenceDays)
	}
	if !s.Sync.IncludeSettingsFile {
		t.Errorf("Sync.IncludeSettingsFile = %v", s.Sync.IncludeSettingsFile)
	}
}
