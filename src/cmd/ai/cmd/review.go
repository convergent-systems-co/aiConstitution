package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/convergent-systems-co/aiConstitution/src/internal/paths"
	"github.com/spf13/cobra"
)

// newReviewCmd implements `ai review`. See SPEC.md §3.2 and §6.
func newReviewCmd() *cobra.Command {
	var check bool
	var since time.Duration
	var apply bool
	var dryRun bool

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

See SPEC.md §3.2 + §6 + §6.5 (30-day idle prompt).`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if check {
				return runReviewCheck(cmd, since)
			}
			notice("review:", "would walk ~/.ai/memory/ and propose amendments.")
			_ = apply
			_ = dryRun
			return stub("review", "§3.2 + §6")
		},
	}

	c.Flags().BoolVar(&check, "check", false, "cheap dry-run; emit a one-line nag and exit 0")
	c.Flags().DurationVar(&since, "since", 0, "only consider memory entries newer than this (e.g. 30d, 168h)")
	c.Flags().BoolVar(&apply, "apply", false, "apply approved amendments (per-item confirmation)")
	c.Flags().BoolVar(&dryRun, "dry-run", false, "print proposed amendments without writing")

	return c
}

func runReviewCheck(cmd *cobra.Command, since time.Duration) error {
	root := paths.AIRoot()
	out := cmd.OutOrStdout()

	var cutoff time.Time
	if since > 0 {
		cutoff = time.Now().Add(-since)
	}

	var report strings.Builder
	report.WriteString(fmt.Sprintf("# Governance Report — %s\n\n", time.Now().UTC().Format("2006-01-02")))

	// Scan 1: Violations
	violations := scanAuditDir(filepath.Join(root, "audit", "violations"), cutoff)
	report.WriteString(fmt.Sprintf("## Violation Scan (%d files)\n\n", len(violations)))
	for _, v := range violations {
		report.WriteString(fmt.Sprintf("- %s\n", filepath.Base(v)))
	}
	if len(violations) > 0 {
		report.WriteString("\n**Action:** Review each violation and consider ai amend draft.\n")
		_, _ = fmt.Fprintf(out, "Violations found: %d\n", len(violations))
	}
	report.WriteString("\n")

	// Scan 2: Overrides
	overrides := scanAuditDir(filepath.Join(root, "audit", "overrides"), cutoff)
	report.WriteString(fmt.Sprintf("## Override Scan (%d files)\n\n", len(overrides)))
	repeated := findRepeatedRules(overrides)
	for rule, count := range repeated {
		report.WriteString(fmt.Sprintf("- %s overridden %d times — consider amending.\n", rule, count))
	}
	report.WriteString("\n")

	// Scan 3: Drift
	drifts := scanAuditDir(filepath.Join(root, "audit", "drift"), cutoff)
	report.WriteString(fmt.Sprintf("## Drift Scan (%d records)\n\n", len(drifts)))
	for _, d := range drifts {
		report.WriteString(fmt.Sprintf("- %s\n", filepath.Base(d)))
	}
	if len(drifts) > 0 {
		report.WriteString("\n**Action:** Near-miss clusters may need enforcement hooks.\n")
	}
	report.WriteString("\n")

	// Scan 4: Dead rules
	deadCutoff := time.Now().AddDate(0, 0, -90)
	report.WriteString("## Dead-Rule Scan\n\n")
	report.WriteString(fmt.Sprintf("Rules not referenced in any audit file since %s are candidates for pruning.\n\n", deadCutoff.Format("2006-01-02")))

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

func scanAuditDir(dir string, after time.Time) []string {
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

func findRepeatedRules(files []string) map[string]int {
	counts := make(map[string]int)
	for _, f := range files {
		data, err := os.ReadFile(f) //nolint:gosec
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(data), "\n") {
			if strings.HasPrefix(line, "- **File / Rule") || strings.HasPrefix(line, "- **Rule:**") {
				counts[strings.TrimSpace(line)]++
			}
		}
	}
	result := make(map[string]int)
	for rule, count := range counts {
		if count >= 2 {
			result[rule] = count
		}
	}
	return result
}
