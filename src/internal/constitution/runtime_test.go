package constitution_test

import (
	"strings"
	"testing"
	"github.com/convergent-systems-co/aiConstitution/src/internal/constitution"
)

const sampleConstitution = `# AI Constitution — Alice

**Principal:** Alice
**Version:** 1.0

## §1 Governance

Override protocol lives here.

## §2 Behavioral Standards

### §2.1 Conviction

Agreement is not the goal.

### §2.5 Helpfulness

Helpfulness is compliance with actual intent.

## §3 Universal Rules

### §3.1 Prime Directives

P1. Civilization-grade output.

### §3.2 Autonomy Gates

Cost ceiling: $5
Protected: main,

## §4 Technical Work

Code and engineering domain.
`

func TestExtractRuntime_HasRequiredSections(t *testing.T) {
	rc, err := constitution.ExtractRuntime(sampleConstitution)
	if err != nil {
		t.Fatalf("ExtractRuntime() error: %v", err)
	}
	if rc.Header == "" {
		t.Error("Header is empty")
	}
	if rc.BehavioralStandards == "" {
		t.Error("BehavioralStandards is empty")
	}
	if rc.PrimeDirectives == "" {
		t.Error("PrimeDirectives is empty")
	}
	if rc.AutonomyGates == "" {
		t.Error("AutonomyGates is empty")
	}
	if len(rc.DomainSummaries) == 0 {
		t.Error("DomainSummaries is empty")
	}
}

func TestFormatRuntime_UnderTokenBudget(t *testing.T) {
	rc, _ := constitution.ExtractRuntime(sampleConstitution)
	out := constitution.FormatRuntime(rc)
	// 4000 tokens × 4 chars/token = 16000 chars
	if len(out) > 16_000 {
		t.Errorf("FormatRuntime() output too large: %d chars", len(out))
	}
	if !strings.Contains(out, "Alice") {
		t.Error("expected principal name in runtime output")
	}
}

func TestFormatRuntime_ContainsKeyHeadings(t *testing.T) {
	rc, _ := constitution.ExtractRuntime(sampleConstitution)
	out := constitution.FormatRuntime(rc)
	for _, want := range []string{"Behavioral Standards", "Prime Directives", "Autonomy Gates"} {
		if !strings.Contains(out, want) {
			t.Errorf("FormatRuntime() missing section %q", want)
		}
	}
}
