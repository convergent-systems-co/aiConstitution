package panels_test

import (
	"math"
	"testing"

	"github.com/convergent-systems-co/aiConstitution/src/internal/panels"
)

// --- #253: LoadDefaultPanels ---

func TestLoadDefaultPanels_ReturnsPanels(t *testing.T) {
	got, err := panels.LoadDefaultPanels()
	if err != nil {
		t.Fatalf("LoadDefaultPanels returned error: %v", err)
	}
	if len(got) == 0 {
		t.Fatal("expected at least one panel, got zero")
	}
}

func TestLoadDefaultPanels_WeightsSumToOne(t *testing.T) {
	got, err := panels.LoadDefaultPanels()
	if err != nil {
		t.Fatalf("LoadDefaultPanels returned error: %v", err)
	}
	var total float64
	for _, p := range got {
		total += p.Weight
	}
	// Allow ±0.01 tolerance for floating-point representation
	if math.Abs(total-1.0) > 0.01 {
		t.Errorf("panel weights sum: want ~1.0, got %f", total)
	}
}

func TestLoadDefaultPanels_EachPanelHasName(t *testing.T) {
	got, err := panels.LoadDefaultPanels()
	if err != nil {
		t.Fatalf("LoadDefaultPanels returned error: %v", err)
	}
	for i, p := range got {
		if p.Name == "" {
			t.Errorf("panel[%d].Name is empty", i)
		}
	}
}

func TestLoadDefaultPanels_ContainsCodeReview(t *testing.T) {
	got, err := panels.LoadDefaultPanels()
	if err != nil {
		t.Fatalf("LoadDefaultPanels returned error: %v", err)
	}
	for _, p := range got {
		if p.Name == "code-review" {
			return
		}
	}
	t.Error("expected panel named 'code-review', not found")
}

// --- #254: ScorePanels — weighted confidence aggregation ---

func TestScorePanels_TwoPassOneFail_ComputesWeightedScore(t *testing.T) {
	panelList := []panels.Panel{
		{Name: "code-review", Weight: 0.5, Domains: []string{"code"}},
		{Name: "security-review", Weight: 0.3, Domains: []string{"security"}},
		{Name: "docs-review", Weight: 0.2, Domains: []string{"docs"}},
	}
	results := map[string]panels.PanelResult{
		"code-review":     {Pass: true, Confidence: 0.9, Findings: []string{"LGTM"}},
		"security-review": {Pass: true, Confidence: 0.8, Findings: []string{"no issues"}},
		"docs-review":     {Pass: false, Confidence: 0.4, Findings: []string{"missing doc"}},
	}

	score, summary := panels.ScorePanels(panelList, results)

	// Weighted average: 0.5*0.9 + 0.3*0.8 + 0.2*0.4 = 0.45+0.24+0.08 = 0.77
	wantScore := 0.5*0.9 + 0.3*0.8 + 0.2*0.4
	if math.Abs(score-wantScore) > 0.001 {
		t.Errorf("score: want %.4f, got %.4f", wantScore, score)
	}
	if summary == "" {
		t.Error("summary must not be empty")
	}
}

func TestScorePanels_MajorityPass_OverallPass(t *testing.T) {
	panelList := []panels.Panel{
		{Name: "a", Weight: 0.4},
		{Name: "b", Weight: 0.4},
		{Name: "c", Weight: 0.2},
	}
	results := map[string]panels.PanelResult{
		"a": {Pass: true, Confidence: 0.9},
		"b": {Pass: true, Confidence: 0.9},
		"c": {Pass: false, Confidence: 0.1},
	}
	_, summary := panels.ScorePanels(panelList, results)
	// 2 of 3 panels pass => summary should contain "PASS"
	if summary == "" {
		t.Fatal("summary must not be empty")
	}
	// Verify the summary signals PASS
	found := false
	for _, needle := range []string{"PASS", "pass"} {
		for i := 0; i+len(needle) <= len(summary); i++ {
			if summary[i:i+len(needle)] == needle {
				found = true
				break
			}
		}
		if found {
			break
		}
	}
	if !found {
		t.Errorf("expected PASS in summary for majority-pass scenario, got: %q", summary)
	}
}

func TestScorePanels_MajorityFail_OverallFail(t *testing.T) {
	panelList := []panels.Panel{
		{Name: "a", Weight: 0.5},
		{Name: "b", Weight: 0.3},
		{Name: "c", Weight: 0.2},
	}
	results := map[string]panels.PanelResult{
		"a": {Pass: false, Confidence: 0.2},
		"b": {Pass: false, Confidence: 0.3},
		"c": {Pass: true, Confidence: 0.9},
	}
	_, summary := panels.ScorePanels(panelList, results)
	found := false
	for _, needle := range []string{"FAIL", "fail"} {
		for i := 0; i+len(needle) <= len(summary); i++ {
			if summary[i:i+len(needle)] == needle {
				found = true
				break
			}
		}
		if found {
			break
		}
	}
	if !found {
		t.Errorf("expected FAIL in summary for majority-fail scenario, got: %q", summary)
	}
}

func TestScorePanels_EmptyResults_ReturnsZeroScore(t *testing.T) {
	panelList := []panels.Panel{
		{Name: "a", Weight: 1.0},
	}
	score, summary := panels.ScorePanels(panelList, map[string]panels.PanelResult{})
	if score != 0.0 {
		t.Errorf("expected score 0.0 for empty results, got %f", score)
	}
	if summary == "" {
		t.Error("summary must not be empty even for empty results")
	}
}
