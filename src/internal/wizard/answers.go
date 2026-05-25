package wizard

import (
	"fmt"
	"strings"

	"github.com/convergent-systems-co/aiConstitution/src/internal/constitution"
)

// AnswersToAnswerSet converts wizard answers (map[qid]value) to a
// constitution.AnswerSet ready for template rendering.
//
// v2.1 reference-first: domains are always technical+prose. Only
// the 8 personalisation slots are taken from answers; everything else
// uses the reference constitution's invariant content.
func AnswersToAnswerSet(answers map[string]string) (constitution.AnswerSet, error) {
	principal := strings.TrimSpace(answers["Q01"])
	if principal == "" {
		principal = "Principal"
	}

	var as constitution.AnswerSet
	as.Principal = principal

	// Q02 — AI tools (comma-separated option values)
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

	as.WorkContext = orDefault(answers["Q03"], "personal")

	// Domains are always both technical and prose — the reference constitution
	// includes both by default. Users who need legal/data/research can add
	// sections after setup via ai amend.
	as.Domains = []constitution.Domain{
		{
			Name:          "Technical Work",
			SectionNum:    4,
			Preamble:      "This section governs software engineering, infrastructure, and technical work.",
			PersonalRules: "",
			Template:      "technical",
		},
		{
			Name:          "Prose & Writing",
			SectionNum:    5,
			Preamble:      "This section governs written artifacts: essays, documentation, journalism, theology, philosophy, and fiction.",
			PersonalRules: "",
			Template:      "prose",
		},
	}

	// Q06 — cost ceiling
	as.CostCeiling = orDefault(answers["Q06"], "$5")

	// Q07 — blast radius (not a question in v2.1; use safe default)
	as.BlastRadius = 100

	// Q08 — protected branches
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

	// Autonomy posture — always autonomous (not a question in v2.1)
	as.AutonomyPosture = "autonomous"

	// Q10 — pushback persistence
	as.PushbackPersistence = orDefault(answers["Q10"], "flag-once")

	// Q11 — response length
	as.ResponseLength = orDefault(answers["Q11"], "match-complexity")

	// Disagreement tone — always direct-framing (not a question in v2.1)
	as.DisagreementTone = "direct-framing"

	// Q13 — provenance in commits
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

// domainTemplateMap and domainNameMap kept for compatibility with any
// callers that still pass Q04 answers (migration path).
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

// FindingSlug is the exported slug helper used by memory codify.
func FindingSlug(rule string) string { return findingSlug(rule) }

func findingSlug(rule string) string {
	s := strings.ToLower(rule)
	var b strings.Builder
	for _, r := range s {
		switch r {
		case ' ', '\t', '/', '\\', ':', '*', '?', '"', '<', '>', '|':
			b.WriteRune('-')
		default:
			b.WriteRune(r)
		}
	}
	result := b.String()
	if len(result) > 32 {
		return result[:32]
	}
	return result
}
