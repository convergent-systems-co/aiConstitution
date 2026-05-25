package compress_test

import (
	"strings"
	"testing"

	"github.com/convergent-systems-co/aiConstitution/src/internal/compress"
	"github.com/convergent-systems-co/aiConstitution/src/internal/constitution"
)

func section(num int, name, body string) constitution.Section {
	return constitution.Section{
		Number:   num,
		Name:     name,
		Slug:     strings.ToLower(name),
		FileName: name + ".md",
		Body:     body,
	}
}

func TestExtractYAMLContainsPersonaMeta(t *testing.T) {
	s := section(1, "Common", "**P1. Honesty.** MUST NOT fabricate. *(Non-overridable.)*\n\n**P2. Cost.** Ask before exceeding $5.")
	ds, err := compress.Extract(s, "0.17")
	if err != nil {
		t.Fatalf("Extract() error = %v", err)
	}
	yaml := string(ds.YAML)
	if !strings.Contains(yaml, "persona: common") {
		t.Errorf("YAML missing persona field, got:\n%s", yaml)
	}
	if !strings.Contains(yaml, `version: "0.17"`) {
		t.Errorf("YAML missing version, got:\n%s", yaml)
	}
}

func TestExtractYAMLRulesGateInference(t *testing.T) {
	s := section(1, "Common", "**P1. Hard rule.** You MUST NOT do this.\n\n**P2. Soft rule.** You SHOULD prefer this.\n\n**P3. Permission.** You MAY skip this.")
	ds, err := compress.Extract(s, "0.1")
	if err != nil {
		t.Fatalf("Extract() error = %v", err)
	}
	yaml := string(ds.YAML)
	if !strings.Contains(yaml, "gate: hard") {
		t.Errorf("YAML missing hard gate, got:\n%s", yaml)
	}
	if !strings.Contains(yaml, "gate: soft") {
		t.Errorf("YAML missing soft gate, got:\n%s", yaml)
	}
	if !strings.Contains(yaml, "gate: permission") {
		t.Errorf("YAML missing permission gate, got:\n%s", yaml)
	}
}

func TestExtractYAMLNonOverridable(t *testing.T) {
	s := section(1, "Common", "**P1. Honesty.** MUST NOT fabricate. *(Non-overridable.)*")
	ds, err := compress.Extract(s, "0.1")
	if err != nil {
		t.Fatalf("Extract() error = %v", err)
	}
	if !strings.Contains(string(ds.YAML), "non_overridable: true") {
		t.Errorf("YAML missing non_overridable: true, got:\n%s", string(ds.YAML))
	}
}

func TestExtractHashIsStable(t *testing.T) {
	s := section(1, "Common", "**P1. Honesty.** MUST NOT fabricate.")
	ds1, _ := compress.Extract(s, "0.1")
	ds2, _ := compress.Extract(s, "0.1")
	if ds1.Hash != ds2.Hash {
		t.Errorf("Hash not stable: %q vs %q", ds1.Hash, ds2.Hash)
	}
}

func TestExtractCompactContainsRuleLabels(t *testing.T) {
	s := section(1, "Common", "**P1. No fabrication.** MUST NOT invent APIs.\n\n**P2. No secrets.** MUST NOT write credentials.")
	ds, err := compress.Extract(s, "0.1")
	if err != nil {
		t.Fatalf("Extract() error = %v", err)
	}
	compact := string(ds.Compact)
	if !strings.Contains(compact, "No fabrication") {
		t.Errorf("Compact missing rule label, got:\n%s", compact)
	}
}

func TestExtractRuleIDsUsesSectionDotIndex(t *testing.T) {
	s := section(2, "Code", "**2.1 Function length.** MUST be ≤30 lines.\n\n**2.2 Cyclomatic.** MUST be ≤10.")
	ds, err := compress.Extract(s, "0.6")
	if err != nil {
		t.Fatalf("Extract() error = %v", err)
	}
	yaml := string(ds.YAML)
	if !strings.Contains(yaml, `id: "2.1"`) {
		t.Errorf("YAML missing rule id 2.1, got:\n%s", yaml)
	}
	if !strings.Contains(yaml, `id: "2.2"`) {
		t.Errorf("YAML missing rule id 2.2, got:\n%s", yaml)
	}
}
