package cmd

import (
	"fmt"
	"path/filepath"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/convergent-systems-co/aiConstitution/src/internal/panels"
	"github.com/convergent-systems-co/aiConstitution/src/internal/paths"
	"github.com/spf13/cobra"
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
// runs each enabled panel against the diff, and prints a scored report.
//
// Panel invocations are stubs in this release: each panel prints a
// placeholder score. Real panel evaluation is deferred to the panel-eval
// milestone. See Epic #26.
func runPRReview(cmd *cobra.Command, pr int) error {
	out := cmd.OutOrStdout()

	// 1. Print the report header.
	fmt.Fprintf(out, "## Review: PR #%d\n", pr)

	// 2. Fetch the diff (best-effort; report continues even on gh failure).
	diff, diffErr := fetchPRDiff(pr)
	if diffErr != nil {
		fmt.Fprintf(out, "[warn] could not fetch PR diff: %v\n", diffErr)
		diff = "" // proceed with empty diff — panels still run (stubbed)
	}
	_ = diff // panels will consume this when eval is implemented

	// 3. Load the configured panels.
	panelList, err := panels.LoadDefaultPanels()
	if err != nil {
		return fmt.Errorf("review --pr: load panels: %w", err)
	}

	// 4. Run each panel (stubbed: placeholder scores only).
	results := make(map[string]panels.PanelResult, len(panelList))
	for _, p := range panelList {
		// Stub: every panel passes with a placeholder confidence of 0.75.
		// TODO: wire real panel evaluators once panel-eval milestone lands.
		result := panels.PanelResult{
			Pass:       true,
			Confidence: 0.75,
			Findings:   []string{"(stub — real evaluation not yet implemented)"},
		}
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
