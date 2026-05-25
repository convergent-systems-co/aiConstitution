// Package panels provides the panel configuration and weighted scoring
// system for `ai review --pr`. See SPEC.md §6 and Epic #26 (#253, #254).
package panels

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"
)

// defaultPanelsJSON is the embedded fallback panel configuration.
// It is embedded at compile time from panels.defaults.json in this
// package directory. This ensures `ai review --pr` works on a fresh
// install before the user has a ~/.ai/panels.defaults.json.
//
//go:embed panels.defaults.json
var defaultPanelsJSON []byte

// Panel describes one review panel: its name, description, scoring
// weight, and the code/doc domains it covers.
type Panel struct {
	// Name is the kebab-case panel identifier (e.g. "code-review").
	Name string `json:"name"`

	// Description is a human-readable summary of what this panel checks.
	Description string `json:"description"`

	// Weight is the fractional contribution of this panel to the aggregate
	// score (0.0–1.0). All weights in the defaults file sum to 1.0.
	Weight float64 `json:"weight"`

	// Domains is the list of concern areas this panel covers.
	Domains []string `json:"domains"`
}

// PanelResult captures the outcome of one panel's evaluation of a diff.
type PanelResult struct {
	// Pass is true when this panel considers the diff acceptable.
	Pass bool

	// Confidence is this panel's confidence in its verdict (0.0–1.0).
	Confidence float64

	// Findings is the list of observations (concerns or positives).
	Findings []string
}

// LoadDefaultPanels returns the panel definitions from the embedded
// panels.defaults.json. If the embedded file cannot be parsed, an error
// is returned.
func LoadDefaultPanels() ([]Panel, error) {
	var ps []Panel
	if err := json.Unmarshal(defaultPanelsJSON, &ps); err != nil {
		return nil, fmt.Errorf("panels.LoadDefaultPanels: parse embedded defaults: %w", err)
	}
	return ps, nil
}

// ScorePanels computes the weighted average confidence across all panels
// that have a result in the results map. Panels without a matching entry
// in results are skipped (they do not contribute to the weighted sum).
//
// Returns:
//   - score: weighted average confidence (0.0–1.0). Zero if no results.
//   - summary: human-readable verdict string ending in "PASS" or "FAIL".
//
// Overall verdict rule: PASS when strictly more than half of the panels
// (by count, not weight) that appear in results have Pass == true.
func ScorePanels(panels []Panel, results map[string]PanelResult) (float64, string) {
	if len(results) == 0 {
		return 0.0, "Overall: 0.00 (FAIL — no panel results)"
	}

	var weightedSum, totalWeight float64
	var passCount, resultCount int

	for _, p := range panels {
		res, ok := results[p.Name]
		if !ok {
			continue
		}
		weightedSum += p.Weight * res.Confidence
		totalWeight += p.Weight
		resultCount++
		if res.Pass {
			passCount++
		}
	}

	var score float64
	if totalWeight > 0 {
		score = weightedSum / totalWeight
	}

	verdict := "FAIL"
	if resultCount > 0 && passCount*2 > resultCount {
		verdict = "PASS"
	}

	summary := buildSummary(score, verdict, passCount, resultCount)
	return score, summary
}

// buildSummary formats the one-line verdict string.
func buildSummary(score float64, verdict string, passCount, total int) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "Overall: %.2f (%s — %d/%d panels passed)", score, verdict, passCount, total)
	return sb.String()
}
