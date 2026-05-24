package constitution

import (
	"fmt"
	"strings"
)

type RuntimeContent struct {
	Header              string
	BehavioralStandards string
	PrimeDirectives     string
	AutonomyGates       string
	DomainSummaries     []string
}

func ExtractRuntime(content string) (RuntimeContent, error) {
	sections := splitBySectionHeading(content)
	var rc RuntimeContent

	for heading, body := range sections {
		switch {
		case heading == "" || strings.HasPrefix(heading, "# "):
			rc.Header = firstNLines(body, 10)
		case strings.Contains(heading, "§1 Governance"):
			// skip
		case strings.Contains(heading, "§2 Behavioral"):
			rc.BehavioralStandards = body
		case strings.Contains(heading, "§3 Universal"):
			rc.PrimeDirectives = extractSubSection(body, "§3.1")
			rc.AutonomyGates = extractSubSection(body, "§3.2")
		default:
			if strings.Contains(heading, "§") {
				rc.DomainSummaries = append(rc.DomainSummaries,
					fmt.Sprintf("%s\n%s", heading, firstParagraph(body)))
			}
		}
	}

	if rc.PrimeDirectives == "" {
		return rc, fmt.Errorf("runtime extraction: §3.1 Prime Directives not found")
	}
	return rc, nil
}

func FormatRuntime(rc RuntimeContent) string {
	var sb strings.Builder
	sb.WriteString("# AI Constitution (Runtime)\n\n")
	sb.WriteString(rc.Header)
	sb.WriteString("\n---\n\n## §2 Behavioral Standards\n\n")
	sb.WriteString(rc.BehavioralStandards)
	sb.WriteString("\n---\n\n## §3 Core Rules (condensed)\n\n")
	sb.WriteString("### Prime Directives\n\n")
	sb.WriteString(rc.PrimeDirectives)
	sb.WriteString("\n\n### Autonomy Gates\n\n")
	sb.WriteString(rc.AutonomyGates)
	if len(rc.DomainSummaries) > 0 {
		sb.WriteString("\n---\n\n## Domains\n\n")
		for _, d := range rc.DomainSummaries {
			sb.WriteString(d)
			sb.WriteString("\n\n")
		}
	}
	return sb.String()
}

func splitBySectionHeading(content string) map[string]string {
	result := make(map[string]string)
	var currentHeading, currentBody strings.Builder
	for _, line := range strings.Split(content, "\n") {
		if strings.HasPrefix(line, "## ") {
			result[currentHeading.String()] = currentBody.String()
			currentHeading.Reset()
			currentBody.Reset()
			currentHeading.WriteString(line)
		} else {
			currentBody.WriteString(line)
			currentBody.WriteByte('\n')
		}
	}
	result[currentHeading.String()] = currentBody.String()
	return result
}

func extractSubSection(body, prefix string) string {
	lines := strings.Split(body, "\n")
	var out strings.Builder
	inSection := false
	for _, l := range lines {
		if strings.Contains(l, prefix) {
			inSection = true
		} else if inSection && strings.HasPrefix(l, "### §") && !strings.Contains(l, prefix) {
			break
		}
		if inSection {
			out.WriteString(l)
			out.WriteByte('\n')
		}
	}
	return out.String()
}

func firstParagraph(s string) string {
	parts := strings.SplitN(strings.TrimSpace(s), "\n\n", 2)
	return parts[0]
}

func firstNLines(s string, n int) string {
	lines := strings.Split(s, "\n")
	if len(lines) <= n {
		return s
	}
	return strings.Join(lines[:n], "\n")
}
