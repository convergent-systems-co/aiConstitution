package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/convergent-systems-co/aiConstitution/src/internal/paths"

	"github.com/spf13/cobra"
)

// auditTimestamp formats a UTC time as a compact ISO-8601 filename prefix
// matching the canonical pattern used across the audit subsystem:
// 20060102T150405Z (no colons, no dashes, terminating Z).
func auditTimestamp(t time.Time) string {
	return t.UTC().Format("20060102T150405Z")
}

// newAuditCmd implements `ai audit {override,violation,list,show,rotate}`.
// Mentioned in SPEC.md §11.2 as part of the existing/stays-in-CLI set.
// The override/violation file shape is governed by
// ~/.ai/Constitution.md §5.1 + §5.2.
func newAuditCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "audit",
		Short: "Record overrides and violations into ~/.ai/audit/",
		Long: `audit is the canonical surface for adding override and violation
records. The file format is defined by Constitution.md §5.1 (overrides)
and §5.2 (violations).`,
	}

	c.AddCommand(
		newAuditOverrideCmd(),
		newAuditViolationCmd(),
		newAuditListCmd(),
		newAuditShowCmd(),
		newAuditRotateCmd(),
	)
	return c
}

// newAuditOverrideCmd implements `ai audit override`.
// Writes a Constitution.md §1.5.1 override record to
// ~/.ai/audit/overrides/<UTC>.md.
func newAuditOverrideCmd() *cobra.Command {
	var (
		tool         string
		section      string
		scope        string
		strict       string
		relaxed      string
		risk         string
		confirmation string
		artifacts    string
	)

	c := &cobra.Command{
		Use:   "override",
		Short: "Record an override (writes audit/overrides/<UTC>.md)",
		Long: `override writes a Constitution.md §1.5.1 override record to
~/.ai/audit/overrides/<UTC-ISO8601>.md. All flags are required.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			// Validate required flags.
			if section == "" {
				return fmt.Errorf("audit override: --section is required")
			}
			if scope == "" {
				return fmt.Errorf("audit override: --scope is required")
			}
			if strict == "" {
				return fmt.Errorf("audit override: --strict is required")
			}
			if relaxed == "" {
				return fmt.Errorf("audit override: --relaxed is required")
			}
			if risk == "" {
				return fmt.Errorf("audit override: --risk is required")
			}
			if confirmation == "" {
				return fmt.Errorf("audit override: --confirmation is required")
			}

			now := time.Now().UTC()
			ts := auditTimestamp(now)
			humanTS := now.Format(time.RFC3339)

			overridesDir := filepath.Join(paths.AuditDir(), "overrides")
			if err := os.MkdirAll(overridesDir, 0o750); err != nil {
				return fmt.Errorf("audit override: mkdir overrides: %w", err)
			}

			outPath := filepath.Join(overridesDir, ts+".md")
			body := fmt.Sprintf("# Override — %s\n\n"+
				"- **Tool / Agent:** %s\n"+
				"- **Section / Rule relaxed:** %s\n"+
				"- **Scope:** %s\n"+
				"- **Strict behavior:** %s\n"+
				"- **Relaxed behavior:** %s\n"+
				"- **Risk acknowledged:** %s\n"+
				"- **Reasoning (AI):** %s\n"+
				"- **Principal confirmation:** %s\n"+
				"- **Artifacts affected:** %s\n",
				humanTS,
				tool,
				section,
				scope,
				strict,
				relaxed,
				risk,
				confirmation,
				confirmation,
				artifacts,
			)

			//nolint:gosec // G306: audit record (user config dir); 0o600 is intentional
			if err := os.WriteFile(outPath, []byte(body), 0o600); err != nil {
				return fmt.Errorf("audit override: write file: %w", err)
			}
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), outPath)
			return nil
		},
	}

	c.Flags().StringVar(&tool, "tool", "Claude Code", "tool or agent name (e.g. Claude Code)")
	c.Flags().StringVar(&section, "section", "", "section / rule relaxed (e.g. §3.2) [required]")
	c.Flags().StringVar(&scope, "scope", "", "scope: task|session|project|global [required]")
	c.Flags().StringVar(&strict, "strict", "", "one sentence: what strict behavior would have been [required]")
	c.Flags().StringVar(&relaxed, "relaxed", "", "one sentence: what will be done instead [required]")
	c.Flags().StringVar(&risk, "risk", "", "one sentence: concrete failure modes [required]")
	c.Flags().StringVar(&confirmation, "confirmation", "", "verbatim principal confirmation [required]")
	c.Flags().StringVar(&artifacts, "artifacts", "", "comma-separated affected paths or descriptions")

	return c
}

// newAuditViolationCmd implements `ai audit violation`.
// Writes a Constitution.md §1.5.2 violation record to
// ~/.ai/audit/violations/<UTC>.md.
func newAuditViolationCmd() *cobra.Command {
	var (
		section     string
		what        string
		noticed     string
		remediation string
		amendment   string
	)

	c := &cobra.Command{
		Use:   "violation",
		Short: "Record a self-noticed violation (writes audit/violations/<UTC>.md)",
		Long: `violation writes a Constitution.md §1.5.2 violation record to
~/.ai/audit/violations/<UTC-ISO8601>.md.
--section, --what, --noticed, and --remediation are required.
--amendment is optional.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			// Validate required flags.
			if section == "" {
				return fmt.Errorf("audit violation: --section is required")
			}
			if what == "" {
				return fmt.Errorf("audit violation: --what is required")
			}
			if noticed == "" {
				return fmt.Errorf("audit violation: --noticed is required")
			}
			if remediation == "" {
				return fmt.Errorf("audit violation: --remediation is required")
			}

			now := time.Now().UTC()
			ts := auditTimestamp(now)
			humanTS := now.Format(time.RFC3339)

			violationsDir := filepath.Join(paths.AuditDir(), "violations")
			if err := os.MkdirAll(violationsDir, 0o750); err != nil {
				return fmt.Errorf("audit violation: mkdir violations: %w", err)
			}

			outPath := filepath.Join(violationsDir, ts+".md")
			body := fmt.Sprintf("# Violation — %s\n\n"+
				"- **Section / Rule violated:** %s\n"+
				"- **What happened:** %s\n"+
				"- **How noticed:** %s\n"+
				"- **Remediation:** %s\n"+
				"- **Proposed amendment (if any):** %s\n",
				humanTS,
				section,
				what,
				noticed,
				remediation,
				amendment,
			)

			//nolint:gosec // G306: audit record (user config dir); 0o600 is intentional
			if err := os.WriteFile(outPath, []byte(body), 0o600); err != nil {
				return fmt.Errorf("audit violation: write file: %w", err)
			}
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), outPath)
			return nil
		},
	}

	c.Flags().StringVar(&section, "section", "", "section / rule violated (e.g. §3.1.P2) [required]")
	c.Flags().StringVar(&what, "what", "", "one paragraph: what happened [required]")
	c.Flags().StringVar(&noticed, "noticed", "", "how noticed: self-detected|user-flagged|tool-flagged [required]")
	c.Flags().StringVar(&remediation, "remediation", "", "what was done about it [required]")
	c.Flags().StringVar(&amendment, "amendment", "", "proposed amendment link or text (optional)")

	return c
}

// newAuditListCmd enumerates the markdown records under audit/violations/
// and audit/overrides/, sorted newest-first by filename (filenames are
// prefixed with a UTC timestamp in the canonical writer, so lexical sort
// = chronological sort).
func newAuditListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List violation and override records (newest first)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			entries, err := collectAuditEntries()
			if err != nil {
				return fmt.Errorf("audit list: %w", err)
			}
			if len(entries) == 0 {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "(no audit records)")
				return nil
			}
			for _, e := range entries {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), e)
			}
			return nil
		},
	}
}

// collectAuditEntries returns the combined file list from
// audit/violations/ and audit/overrides/, formatted as
// "<kind>/<filename>" and sorted newest-first.
func collectAuditEntries() ([]string, error) {
	var out []string
	for _, sub := range []string{"violations", "overrides"} {
		dir := filepath.Join(paths.AuditDir(), sub)
		ents, err := os.ReadDir(dir)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return nil, err
		}
		for _, e := range ents {
			if e.IsDir() {
				continue
			}
			if !strings.HasSuffix(e.Name(), ".md") {
				continue
			}
			out = append(out, sub+"/"+e.Name())
		}
	}
	// Newest-first: filenames begin with a UTC timestamp in canonical
	// writers, so reverse-lexical sort matches chronological order.
	sort.Sort(sort.Reverse(sort.StringSlice(out)))
	return out, nil
}

// newAuditShowCmd prints a single audit file. The name argument is the
// bare filename; the command looks in violations/ first, then overrides/.
func newAuditShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show <filename>",
		Short: "Print a violation or override file from the audit directory",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := resolveAuditFile(args[0])
			if err != nil {
				return fmt.Errorf("audit show: %w", err)
			}
			data, err := os.ReadFile(filepath.Clean(path))
			if err != nil {
				return fmt.Errorf("audit show: %w", err)
			}
			_, _ = fmt.Fprint(cmd.OutOrStdout(), string(data))
			if !strings.HasSuffix(string(data), "\n") {
				_, _ = fmt.Fprintln(cmd.OutOrStdout())
			}
			return nil
		},
	}
}

// resolveAuditFile tries violations/ then overrides/ for an exact or
// substring match. Lookup order: (1) subdir/name exactly, (2) first
// file in subdir whose name contains the slug as a substring.
func resolveAuditFile(name string) (string, error) {
	if strings.Contains(name, "/") {
		candidate := filepath.Join(paths.AuditDir(), name)
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		} else if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
	}
	for _, sub := range []string{"violations", "overrides"} {
		// Exact match first.
		candidate := filepath.Join(paths.AuditDir(), sub, name)
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
		// Substring match — walk the subdir.
		dir := filepath.Join(paths.AuditDir(), sub)
		entries, err := os.ReadDir(dir)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
		for _, e := range entries {
			if !e.IsDir() && strings.Contains(e.Name(), name) {
				return filepath.Join(dir, e.Name()), nil
			}
		}
	}
	return "", fmt.Errorf("audit file %q not found in violations/ or overrides/", name)
}
