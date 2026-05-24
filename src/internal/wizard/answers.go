package wizard

import (
	"fmt"
	"strings"

	"github.com/convergent-systems-co/aiConstitution/src/internal/constitution"
)

var domainTemplateMap = map[string]string{
	"technical": "technical",
	"prose":     "prose",
	"data":      "custom",
	"legal":     "custom",
}

var domainNameMap = map[string]string{
	"technical": "Technical Work",
	"prose":     "Prose & Writing",
	"data":      "Data & Analysis",
	"legal":     "Legal & Compliance",
}

// AnswersToAnswerSet converts wizard answers (map[qid]value) to a
// constitution.AnswerSet ready for template rendering.
func AnswersToAnswerSet(answers map[string]string) (constitution.AnswerSet, error) {
	principal := strings.TrimSpace(answers["Q01"])
	if principal == "" {
		return constitution.AnswerSet{}, fmt.Errorf("wizard: Q01 (principal name) is required")
	}

	var as constitution.AnswerSet
	as.Principal = principal

	// Q02 — tools (comma-separated)
	if toolsRaw := answers["Q02"]; toolsRaw != "" {
		for _, t := range strings.Split(toolsRaw, ",") {
			if t = strings.TrimSpace(t); t != "" {
				as.Tools = append(as.Tools, toolDisplayName(t))
			}
		}
	}
	if len(as.Tools) == 0 {
		as.Tools = []string{"Claude Code"}
	}

	as.WorkContext = answers["Q03"]

	// Q04 — domains; Q05 — personal rules
	personalRules := answers["Q05"]
	sectionNum := 4
	if domainsRaw := answers["Q04"]; domainsRaw != "" {
		for _, d := range strings.Split(domainsRaw, ",") {
			d = strings.TrimSpace(d)
			if d == "" {
				continue
			}
			tmpl := domainTemplateMap[d]
			if tmpl == "" {
				tmpl = "custom"
			}
			name := domainNameMap[d]
			if name == "" {
				name = d
			}
			as.Domains = append(as.Domains, constitution.Domain{
				Name:          name,
				SectionNum:    sectionNum,
				Preamble:      fmt.Sprintf("This section governs %s work.", strings.ToLower(name)),
				PersonalRules: personalRules,
				Template:      tmpl,
			})
			sectionNum++
		}
	}

	as.CostCeiling = orDefault(answers["Q06"], "$5")
	as.BlastRadius = parseIntOrDefault(answers["Q07"], 100)

	if branches := answers["Q08"]; branches != "" {
		for _, b := range strings.Split(branches, ",") {
			if b = strings.TrimSpace(b); b != "" {
				as.ProtectedBranches = append(as.ProtectedBranches, b)
			}
		}
	}
	if len(as.ProtectedBranches) == 0 {
		as.ProtectedBranches = []string{"main"}
	}

	as.AutonomyPosture = orDefault(answers["Q09"], "autonomous")
	as.PushbackPersistence = orDefault(answers["Q10"], "flag-once")
	as.ResponseLength = orDefault(answers["Q11"], "match-complexity")
	as.DisagreementTone = orDefault(answers["Q12"], "direct-framing")
	as.ProvenanceInCommits = answers["Q13"] == "true"

	return as, nil
}

func orDefault(s, def string) string {
	if strings.TrimSpace(s) == "" {
		return def
	}
	return strings.TrimSpace(s)
}

func parseIntOrDefault(s string, def int) int {
	var n int
	if _, err := fmt.Sscanf(strings.TrimSpace(s), "%d", &n); err == nil && n > 0 {
		return n
	}
	return def
}

func toolDisplayName(value string) string {
	m := map[string]string{
		"claude-code": "Claude Code",
		"copilot-cli": "Copilot CLI",
		"cursor":      "Cursor",
		"codex":       "Codex",
	}
	if name, ok := m[value]; ok {
		return name
	}
	return value
}
