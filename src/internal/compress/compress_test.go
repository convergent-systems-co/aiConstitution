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

// TestExtract_CapturesBulletSubRules verifies that a block of bullet-format
// sub-rules (e.g. "- **13.1 Capacity gate.** ...") is captured as individual
// rules with their explicit N.M IDs.
func TestExtract_CapturesBulletSubRules(t *testing.T) {
	body := "**U13. Context-window discipline.** Treat it as a budget, not a buffer.\n\n" +
		"- **13.1 Capacity gate.** At or above 80% utilization, stop. MUST stop.\n" +
		"- **13.2 Clean tree.** You MUST NOT auto-compact on a dirty tree.\n" +
		"- **13.3 Checkpoint then summarize.** Update HANDOFF.md per U10."
	s := section(3, "Common", body)
	ds, err := compress.Extract(s, "1.0")
	if err != nil {
		t.Fatalf("Extract() error = %v", err)
	}
	yaml := string(ds.YAML)
	for _, wantID := range []string{`"13.1"`, `"13.2"`, `"13.3"`} {
		if !strings.Contains(yaml, "id: "+wantID) {
			t.Errorf("YAML missing sub-rule id %s:\n%s", wantID, yaml)
		}
	}
}

// TestExtract_BulletSubRuleGateInference verifies that MUST in a bullet
// sub-rule is correctly inferred as a hard gate.
func TestExtract_BulletSubRuleGateInference(t *testing.T) {
	body := "**U15. Bounded self-correction.** When not converging, stop.\n\n" +
		"- **15.1 Three-cycle local cap.** After three failed attempts MUST stop.\n" +
		"- **15.2 Five-cycle total cap.** After five total attempts, escalate.\n" +
		"- **15.3 No silent retries.** A retry MUST be visible in your output."
	s := section(3, "Common", body)
	ds, err := compress.Extract(s, "1.0")
	if err != nil {
		t.Fatalf("Extract() error = %v", err)
	}
	yaml := string(ds.YAML)
	if !strings.Contains(yaml, `id: "15.1"`) {
		t.Errorf("YAML missing 15.1:\n%s", yaml)
	}
	if !strings.Contains(yaml, "gate: hard") {
		t.Errorf("YAML missing hard gate for MUST rules:\n%s", yaml)
	}
}

// TestRuleIDs_IncludesBulletSubRules verifies RuleIDs returns N.M sub-rule IDs.
func TestRuleIDs_IncludesBulletSubRules(t *testing.T) {
	body := "**U13. Context discipline.** Treat it as a budget.\n\n" +
		"- **13.1 Capacity gate.** MUST stop at 80%.\n" +
		"- **13.2 Clean tree.** MUST NOT compact on dirty tree.\n\n" +
		"**U14. Independent verification.** MUST cross-reference."
	s := section(3, "Common", body)
	ids := compress.RuleIDs(s)

	want := map[string]bool{"13.1": true, "13.2": true}
	for _, id := range ids {
		delete(want, id)
	}
	for id := range want {
		t.Errorf("RuleIDs missing expected ID %q; got %v", id, ids)
	}
}

// TestRuleIDs_StableOrder verifies RuleIDs returns IDs in consistent order.
func TestRuleIDs_StableOrder(t *testing.T) {
	body := "**P1. Honesty.** MUST NOT fabricate.\n\n**P2. Cost.** Ask before exceeding."
	s := section(1, "Common", body)
	ids1 := compress.RuleIDs(s)
	ids2 := compress.RuleIDs(s)
	if len(ids1) != len(ids2) {
		t.Fatalf("RuleIDs not stable: %v vs %v", ids1, ids2)
	}
	for i := range ids1 {
		if ids1[i] != ids2[i] {
			t.Errorf("RuleIDs[%d] differs: %q vs %q", i, ids1[i], ids2[i])
		}
	}
}

// TestExtract_ThreeLevelBoldHead verifies that **N.M.K Label.** heads
// (e.g. 4.2.1 after stripping § from template) are captured with the full ID.
func TestExtract_ThreeLevelBoldHead(t *testing.T) {
	body := "**4.2.1 Pattern selection.** Apply Gang-of-Four patterns. MUST apply."
	s := section(4, "Technical", body)
	ds, err := compress.Extract(s, "1.0")
	if err != nil {
		t.Fatalf("Extract() error = %v", err)
	}
	yaml := string(ds.YAML)
	// 4.2.1 contains two dots so YAML may output it unquoted — accept both forms.
	if !strings.Contains(yaml, "id: 4.2.1") && !strings.Contains(yaml, `id: "4.2.1"`) {
		t.Errorf("YAML missing three-level id 4.2.1:\n%s", yaml)
	}
}

// TestExtract_ThreeLevelBulletSubRule verifies that - **N.M.K Label.** bullets
// are captured with the full three-level ID.
func TestExtract_ThreeLevelBulletSubRule(t *testing.T) {
	body := "**4.1 Clean Code.** The rules:\n\n- **4.1.1 Names reveal intent.** MUST reveal intent.\n- **4.1.2 Function length.** SHOULD stay under 30 lines."
	s := section(4, "Technical", body)
	ds, err := compress.Extract(s, "1.0")
	if err != nil {
		t.Fatalf("Extract() error = %v", err)
	}
	yaml := string(ds.YAML)
	for _, wantID := range []string{"4.1.1", "4.1.2"} {
		if !strings.Contains(yaml, "id: "+wantID) && !strings.Contains(yaml, `id: "`+wantID+`"`) {
			t.Errorf("YAML missing sub-rule id %s:\n%s", wantID, yaml)
		}
	}
}

// TestCompactRules_EmitsIDPrefixedLines verifies that CompactRules returns
// §ID-prefixed rule lines without the HTML comment header.
func TestCompactRules_EmitsIDPrefixedLines(t *testing.T) {
	body := "**P1. Honesty.** MUST NOT fabricate. *(Non-overridable.)*\n\n" +
		"**U1. State assumptions.** MUST name gap-fill assumptions.\n\n" +
		"- **13.1 Capacity gate.** MUST stop at 80%%."
	s := section(3, "Universal", body)
	out := compress.CompactRules(s)

	if strings.Contains(out, "<!--") {
		t.Error("CompactRules output must not contain HTML comment header")
	}
	if !strings.Contains(out, "§") {
		t.Errorf("CompactRules output must contain § prefixes, got:\n%s", out)
	}
	if !strings.Contains(out, "[HARD]") {
		t.Errorf("CompactRules output must contain gate tags, got:\n%s", out)
	}
	if !strings.Contains(out, "NON-OVERRIDABLE") {
		t.Errorf("CompactRules output must mark non-overridable rules, got:\n%s", out)
	}
	if !strings.Contains(out, "13.1") {
		t.Errorf("CompactRules output must include bullet sub-rule ID 13.1, got:\n%s", out)
	}
}

// TestCompactRules_EmptySection verifies that CompactRules returns empty
// string when no rules are extractable.
func TestCompactRules_EmptySection(t *testing.T) {
	s := section(4, "Technical", "Just prose with no rule heads or bullets.")
	out := compress.CompactRules(s)
	if out != "" {
		t.Errorf("expected empty string for section with no rules, got %q", out)
	}
}
