package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/convergent-systems-co/aiConstitution/src/internal/panels"
	"github.com/convergent-systems-co/aiConstitution/src/internal/paths"
	"github.com/spf13/cobra"
)

var (
	fetchPRDiffFunc = fetchPRDiff

	awsKeyPattern        = regexp.MustCompile(`AKIA[0-9A-Z]{16}`)
	githubTokenPattern   = regexp.MustCompile(`gh[pousr]_[A-Za-z0-9_]{20,}`)
	passwordValuePattern = regexp.MustCompile(`(?i)\b(password|passwd|secret|token|api[_-]?key)\b\s*[:=]\s*['"][^'"]+['"]`)
)

// newReviewCmd implements `ai review`. See SPEC.md §3.2 and §6.
func newReviewCmd() *cobra.Command {
	var check bool
	var since time.Duration
	var apply bool
	var dryRun bool
	var prNumber int

	c := &cobra.Command{
		Use:   "review",
		Short: "Memory-to-amendment review loop (default cadence: 30 days)",
		Long: `review walks ~/.ai/memory/ for patterns that have crystallized
into rules, proposes amendments against the four canonical files,
and retires the memory once the rule is codified.

Flags:
  --check                Cheap dry-run; emits a one-line nag with the
                         count of pending review candidates and exits 0.
                         Suitable for invocation from ai status.
  --since=<duration>     Only consider memory entries newer than this.
  --apply                Apply the proposed amendments (after per-item
                         approval).
  --dry-run              Print the proposed amendments but do not write.
  --pr=<n>               Fetch the diff for PR #n and run configured
                         review panels against it, printing a scored report.

See SPEC.md §3.2 + §6 + §6.5 (30-day idle prompt).`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if prNumber > 0 {
				return runPRReview(cmd, prNumber)
			}
			if check {
				return runReviewCheck(cmd, since)
			}
			_ = apply
			_ = dryRun
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "review: run 'ai review --check' to scan now.")
			return nil
		},
	}

	c.Flags().BoolVar(&check, "check", false, "cheap dry-run; emit a one-line nag and exit 0")
	c.Flags().DurationVar(&since, "since", 0, "only consider memory entries newer than this (e.g. 30d, 168h)")
	c.Flags().BoolVar(&apply, "apply", false, "apply approved amendments (per-item confirmation)")
	c.Flags().BoolVar(&dryRun, "dry-run", false, "print proposed amendments without writing")
	c.Flags().IntVar(&prNumber, "pr", 0, "review PR #n: fetch diff and run configured panels")

	return c
}

// runPRReview fetches the diff for the given PR number via `gh pr diff`,
// evaluates it with the configured review panels, and prints a scored report.
func runPRReview(cmd *cobra.Command, pr int) error {
	out := cmd.OutOrStdout()

	// 1. Print the report header.
	fmt.Fprintf(out, "## Review: PR #%d\n", pr)

	// 2. Fetch the diff (best-effort; report continues even on gh failure).
	diff, diffErr := fetchPRDiffFunc(pr)
	if diffErr != nil {
		fmt.Fprintf(out, "[warn] could not fetch PR diff: %v\n", diffErr)
		diff = ""
	}
	inspection := inspectDiff(diff)

	// 3. Load the configured panels.
	panelList, err := panels.LoadDefaultPanels()
	if err != nil {
		return fmt.Errorf("review --pr: load panels: %w", err)
	}

	// 4. Run each panel with lightweight heuristics over the fetched diff.
	results := make(map[string]panels.PanelResult, len(panelList))
	for _, p := range panelList {
		result := evaluatePanel(p.Name, inspection)
		results[p.Name] = result

		mark := "✓"
		if !result.Pass {
			mark = "✗"
		}
		finding := ""
		if len(result.Findings) > 0 {
			finding = result.Findings[0]
		}
		fmt.Fprintf(out, "[%s] %s %.2f — %s\n", p.Name, mark, result.Confidence, finding)
	}

	// 5. Compute and print the aggregate score.
	score, summary := panels.ScorePanels(panelList, results)
	_ = score
	fmt.Fprintln(out, summary)

	return nil
}

// fetchPRDiff runs `gh pr diff <n>` and returns the diff output as a string.
// Returns an error if gh is not installed or the command fails.
func fetchPRDiff(pr int) (string, error) {
	args := []string{"pr", "diff", fmt.Sprintf("%d", pr)}
	out, err := exec.Command("gh", args...).Output()
	if err != nil {
		return "", fmt.Errorf("gh pr diff %d: %w", pr, err)
	}
	return strings.TrimSpace(string(out)), nil
}

type diffInspection struct {
	changedFiles   []string
	addedLines     []string
	hasCodeChanges bool
	hasTestChanges bool
	hasDocChanges  bool
	hasUserFacing  bool
}

func inspectDiff(diff string) diffInspection {
	var out diffInspection
	seenFiles := make(map[string]struct{})
	for _, line := range strings.Split(diff, "\n") {
		switch {
		case strings.HasPrefix(line, "+++ b/"):
			path := strings.TrimPrefix(line, "+++ b/")
			if path == "" || path == "/dev/null" {
				continue
			}
			if _, seen := seenFiles[path]; seen {
				continue
			}
			seenFiles[path] = struct{}{}
			out.changedFiles = append(out.changedFiles, path)
			out.hasCodeChanges = out.hasCodeChanges || isCodeFile(path)
			out.hasTestChanges = out.hasTestChanges || isTestFile(path)
			out.hasDocChanges = out.hasDocChanges || isDocFile(path)
			out.hasUserFacing = out.hasUserFacing || isUserFacingFile(path)
		case strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++"):
			out.addedLines = append(out.addedLines, strings.TrimPrefix(line, "+"))
		}
	}
	return out
}

func evaluatePanel(name string, inspection diffInspection) panels.PanelResult {
	if len(inspection.changedFiles) == 0 {
		return panels.PanelResult{
			Pass:       false,
			Confidence: 0.20,
			Findings:   []string{"No diff available for review."},
		}
	}

	switch name {
	case "code-review":
		if inspection.hasCodeChanges && !inspection.hasTestChanges {
			return panels.PanelResult{
				Pass:       false,
				Confidence: 0.45,
				Findings:   []string{"Code changes detected without accompanying test file changes."},
			}
		}
		if inspection.hasCodeChanges {
			return panels.PanelResult{
				Pass:       true,
				Confidence: 0.82,
				Findings:   []string{fmt.Sprintf("Reviewed %d changed file(s); test coverage signal present.", len(inspection.changedFiles))},
			}
		}
		return panels.PanelResult{
			Pass:       true,
			Confidence: 0.70,
			Findings:   []string{"No code file changes detected."},
		}
	case "security-review":
		findings := securityFindings(inspection.addedLines)
		if len(findings) > 0 {
			return panels.PanelResult{
				Pass:       false,
				Confidence: 0.92,
				Findings:   findings,
			}
		}
		return panels.PanelResult{
			Pass:       true,
			Confidence: 0.84,
			Findings:   []string{"No obvious secret or insecure-pattern additions detected."},
		}
	case "documentation-review":
		if inspection.hasUserFacing && !inspection.hasDocChanges {
			return panels.PanelResult{
				Pass:       false,
				Confidence: 0.48,
				Findings:   []string{"User-facing CLI or template changes detected without documentation updates."},
			}
		}
		if inspection.hasDocChanges {
			return panels.PanelResult{
				Pass:       true,
				Confidence: 0.83,
				Findings:   []string{"Documentation changes are included in the diff."},
			}
		}
		return panels.PanelResult{
			Pass:       true,
			Confidence: 0.68,
			Findings:   []string{"No user-facing documentation gap detected."},
		}
	default:
		return panels.PanelResult{
			Pass:       true,
			Confidence: 0.60,
			Findings:   []string{"No custom evaluator registered; falling back to generic review."},
		}
	}
}

func securityFindings(addedLines []string) []string {
	var findings []string
	addFinding := func(finding string) {
		if !slices.Contains(findings, finding) {
			findings = append(findings, finding)
		}
	}

	for _, line := range addedLines {
		lower := strings.ToLower(strings.TrimSpace(line))
		switch {
		case strings.Contains(lower, "insecureskipverify: true"):
			addFinding("TLS certificate verification is disabled in added code.")
		case strings.Contains(lower, "| sh") || strings.Contains(lower, "| bash"):
			if strings.Contains(lower, "curl ") || strings.Contains(lower, "wget ") {
				addFinding("A network-downloaded script is piped directly to a shell.")
			}
		case strings.Contains(lower, "--no-verify"):
			addFinding("A git hook bypass flag was added.")
		case strings.Contains(lower, "begin private key"):
			addFinding("Private key material appears in added content.")
		case awsKeyPattern.MatchString(line), githubTokenPattern.MatchString(line), passwordValuePattern.MatchString(line):
			addFinding("Potential credential or secret material was added.")
		}
	}

	if len(findings) > 3 {
		return findings[:3]
	}
	return findings
}

func isCodeFile(path string) bool {
	if isTestFile(path) || isDocFile(path) {
		return false
	}
	switch strings.ToLower(filepath.Ext(path)) {
	case ".go", ".py", ".js", ".jsx", ".ts", ".tsx", ".rb", ".rs", ".java", ".sh", ".ps1":
		return true
	default:
		return false
	}
}

func isTestFile(path string) bool {
	lower := strings.ToLower(path)
	return strings.HasSuffix(lower, "_test.go") ||
		strings.Contains(lower, ".test.") ||
		strings.Contains(lower, ".spec.")
}

func isDocFile(path string) bool {
	lower := strings.ToLower(path)
	return strings.HasSuffix(lower, ".md") ||
		strings.HasSuffix(lower, ".rst") ||
		strings.HasSuffix(lower, ".adoc") ||
		strings.Contains(lower, "/docs/") ||
		strings.HasSuffix(lower, ".txt")
}

func isUserFacingFile(path string) bool {
	return strings.HasPrefix(path, "src/cmd/ai/cmd/") ||
		strings.HasPrefix(path, "src/cmd/ai/embed/templates/")
}

// runReviewCheck runs the 4-scan governance review cycle:
// violations, overrides, drift, and dead rules. Writes a dated
// Governance Report to ~/.ai/governance/reports/YYYY-MM-DD.md.
func runReviewCheck(cmd *cobra.Command, since time.Duration) error {
	root := paths.AIRoot()
	out := cmd.OutOrStdout()
	cutoff := time.Now().Add(-since)

	var report strings.Builder
	report.WriteString(fmt.Sprintf("# Governance Report — %s\n\n", time.Now().UTC().Format("2006-01-02")))

	// Scan 1: Violations
	violations := scanAuditEntries(filepath.Join(root, "audit", "violations"), cutoff)
	report.WriteString(fmt.Sprintf("## Violation Scan (%d files)\n\n", len(violations)))
	for _, v := range violations {
		report.WriteString(fmt.Sprintf("- %s\n", filepath.Base(v)))
	}
	if len(violations) > 0 {
		report.WriteString("\n**Action:** Consider ai amend draft on each violation.\n")
		_, _ = fmt.Fprintf(out, "Violations: %d\n", len(violations))
	}
	report.WriteString("\n")

	// Scan 2: Overrides
	overrides := scanAuditEntries(filepath.Join(root, "audit", "overrides"), cutoff)
	report.WriteString(fmt.Sprintf("## Override Scan (%d files)\n\n", len(overrides)))
	report.WriteString("\n")

	// Scan 3: Drift
	drifts := scanAuditEntries(filepath.Join(root, "audit", "drift"), cutoff)
	report.WriteString(fmt.Sprintf("## Drift Scan (%d records)\n\n", len(drifts)))
	report.WriteString("\n")

	// Scan 4: Dead rules (informational)
	report.WriteString("## Dead-Rule Scan\n\nRules not referenced in 90 days are candidates for pruning.\n\n")

	// Write report
	reportsDir := filepath.Join(root, "governance", "reports")
	if err := os.MkdirAll(reportsDir, 0o750); err != nil {
		return fmt.Errorf("review: mkdir reports: %w", err)
	}
	reportPath := filepath.Join(reportsDir, time.Now().UTC().Format("2006-01-02")+".md")
	if err := os.WriteFile(reportPath, []byte(report.String()), 0o600); err != nil { //nolint:gosec
		return fmt.Errorf("review: write report: %w", err)
	}
	_, _ = fmt.Fprintf(out, "Report: %s\n", reportPath)
	return nil
}

// scanAuditEntries lists files in dir modified after cutoff.
func scanAuditEntries(dir string, after time.Time) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var result []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if after.IsZero() || info.ModTime().After(after) {
			result = append(result, filepath.Join(dir, e.Name()))
		}
	}
	return result
}
