package wizard

import (
	"fmt"
	"strings"

	"github.com/convergent-systems-co/aiConstitution/src/internal/constitution"
)

// AnswersToAnswerSet converts wizard answers (map[qid]value) to a
// constitution.AnswerSet ready for template rendering.
//
// Domain selection (Q07) is now wired: "code" → technical only,
// "writing" → prose only, everything else → both (recommended default).
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

	// Q07 — domain selection: "code", "writing", "both", or "other".
	// Build the Domains slice based on the user's selection; default to both
	// when the answer is absent or unrecognised (safe fallback, see §U1).
	as.Domains = domainsFromQ07(answers["Q07"])

	// Q06 — cost ceiling
	as.CostCeiling = orDefault(answers["Q06"], "$5")

	// Blast radius — safe default; not a user-facing question in v2.1.
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

// domainsFromQ07 converts the Q07 wizard answer ("code", "writing", "both",
// "other") into the slice of constitution.Domain values used by the template.
//
// Section numbers follow the template layout: §4 for Technical Work, §5 for
// Prose & Writing. When both domains are included, §4 precedes §5.
//
// Assumption: any answer that is not "code" or "writing" is treated as "both"
// (the recommended default). This includes the empty string, "other", and any
// future option values we have not yet mapped.
func domainsFromQ07(q07 string) []constitution.Domain {
	technical := constitution.Domain{
		Name:          "Technical Work",
		SectionNum:    4,
		Preamble:      "This section governs software engineering, infrastructure, and technical work.",
		PersonalRules: "",
		Template:      "technical",
	}
	prose := constitution.Domain{
		Name:          "Prose & Writing",
		SectionNum:    5,
		Preamble:      "This section governs written artifacts: essays, documentation, journalism, theology, philosophy, and fiction.",
		PersonalRules: "",
		Template:      "prose",
	}

	switch strings.TrimSpace(q07) {
	case "code":
		return []constitution.Domain{technical}
	case "writing":
		// Prose is the only domain; give it §4 to keep section numbering tidy.
		prose.SectionNum = 4
		return []constitution.Domain{prose}
	default: // "both", "other", ""
		return []constitution.Domain{technical, prose}
	}
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
