package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// newPlanCmd implements `ai plan new`, `ai plan list`, and `ai plan show`.
// Plans live under <ai_root>/governance/plans/ per SPEC.md §5.4 and §15.
// Their shape follows the MADR format defined in Code.md §11.1.
func newPlanCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "plan",
		Short: "Manage work-product plans under ~/.ai/governance/plans/",
		Long: `plan operates on the work-product plans at ~/.ai/governance/plans/.

Plans follow the MADR template from Code.md §11.1: title, status, date,
Context, Decision, Consequences, and an Alternatives Considered table.

When the superpowers Claude plugin is enabled, plan new produces plans
with - [ ] task lists compatible with subagent-driven-development.`,
	}

	c.AddCommand(
		newPlanNewCmd(),
		newPlanListCmd(),
		newPlanShowCmd(),
	)
	return c
}

// plansDir returns the canonical governance plans directory.
// Mirrors the pattern in amend.go: <ai_root>/governance/plans/.
func plansDir() string {
	return filepath.Join(aiRoot(), "governance", "plans")
}

// planSlugify converts a free-form title into a filename-safe kebab-case slug.
// Example: "Add Widget Feature" → "add-widget-feature"
func planSlugify(title string) string {
	re := regexp.MustCompile(`[^a-z0-9]+`)
	slug := re.ReplaceAllString(strings.ToLower(title), "-")
	slug = strings.Trim(slug, "-")
	// Collapse repeated dashes that may result from multi-char replacements.
	for strings.Contains(slug, "--") {
		slug = strings.ReplaceAll(slug, "--", "-")
	}
	return slug
}

// planMADRTemplate renders a MADR plan file body for the given title and date.
func planMADRTemplate(title, dateStr string) string {
	return fmt.Sprintf(`# %s

**Status:** draft
**Date:** %s

## Context

<!-- What is the problem or opportunity? -->

## Decision

<!-- What was decided? -->

## Consequences

<!-- What are the effects of this decision? -->

## Alternatives Considered

| Alternative | Pros | Cons | Verdict |
|---|---|---|---|
| Option A | ... | ... | Chosen / Rejected |
`, title, dateStr)
}

// ─── plan new ─────────────────────────────────────────────────────────────────

// newPlanNewCmd implements `ai plan new [--title <title>]`.
// It scaffolds a new MADR plan file at <plans-dir>/<UTC>-<slug>.md,
// then opens it in $EDITOR (if set) or prints the path to stdout.
func newPlanNewCmd() *cobra.Command {
	var title string

	c := &cobra.Command{
		Use:   "new",
		Short: "Scaffold a new plan at ~/.ai/governance/plans/<UTC>-<slug>.md",
		Long: `new creates a MADR-formatted plan file at
~/.ai/governance/plans/<UTC>-<slug>.md.

The slug is derived from --title (lowercase, hyphens). When --title is
omitted the slug is "new-plan".

If $EDITOR is set the file is opened immediately. Otherwise the path is
printed to stdout.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if title == "" {
				title = "new-plan"
			}

			slug := planSlugify(title)
			now := time.Now().UTC()
			utcStamp := now.Format("20060102T150405Z")
			dateStr := now.Format("2006-01-02")

			filename := fmt.Sprintf("%s-%s.md", utcStamp, slug)
			dir := plansDir()

			if err := os.MkdirAll(dir, 0o755); err != nil {
				return fmt.Errorf("plan new: mkdir %s: %w", dir, err)
			}

			planPath := filepath.Join(dir, filename)
			body := planMADRTemplate(title, dateStr)

			//nolint:gosec // G306: user plan file — 0o644 is intentional
			if err := os.WriteFile(planPath, []byte(body), 0o644); err != nil {
				return fmt.Errorf("plan new: write %s: %w", planPath, err)
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Created plan: %s\n", planPath)

			editor := os.Getenv("EDITOR")
			if editor != "" {
				return openInEditor(editor, planPath)
			}
			return nil
		},
	}

	c.Flags().StringVar(&title, "title", "", "plan title (used to derive slug and H1 heading)")
	return c
}

// openInEditor launches the user's $EDITOR with the given file path.
// Honors multi-word EDITOR values such as "code --wait".
func openInEditor(editor, path string) error {
	parts := strings.Fields(editor)
	args := append(parts[1:], path)
	c := exec.Command(parts[0], args...) //nolint:gosec // editor is user-controlled
	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return c.Run()
}

// ─── plan list ────────────────────────────────────────────────────────────────

// newPlanListCmd implements `ai plan list`.
// It walks <plans-dir>/*.md and prints a DATE | SLUG | TITLE table.
func newPlanListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List plans under ~/.ai/governance/plans/",
		RunE: func(cmd *cobra.Command, _ []string) error {
			dir := plansDir()

			entries, err := os.ReadDir(dir)
			if err != nil {
				if errors.Is(err, os.ErrNotExist) {
					_, _ = fmt.Fprintln(cmd.OutOrStdout(), "(no plans yet)")
					return nil
				}
				return fmt.Errorf("plan list: read dir %s: %w", dir, err)
			}

			// Collect only .md files.
			type planEntry struct {
				date  string
				slug  string
				title string
			}
			var plans []planEntry
			for _, e := range entries {
				if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
					continue
				}
				date, slug := parsePlanFilename(e.Name())
				title := readPlanTitle(filepath.Join(dir, e.Name()))
				plans = append(plans, planEntry{date: date, slug: slug, title: title})
			}

			if len(plans) == 0 {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "(no plans yet)")
				return nil
			}

			// Print aligned table header.
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%-12s  %-36s  %s\n", "DATE", "SLUG", "TITLE")
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%-12s  %-36s  %s\n",
				strings.Repeat("-", 12), strings.Repeat("-", 36), strings.Repeat("-", 20))
			for _, p := range plans {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%-12s  %-36s  %s\n", p.date, p.slug, p.title)
			}
			return nil
		},
	}
}

// parsePlanFilename extracts the human-readable date and slug from a plan
// filename of the form <UTC>-<slug>.md.
//
// "20260101T120000Z-my-first-plan.md" → ("2026-01-01", "my-first-plan")
//
// For filenames that don't start with a UTC timestamp the date is left empty
// and the full stem (without .md) is used as the slug.
func parsePlanFilename(name string) (date, slug string) {
	stem := strings.TrimSuffix(name, ".md")

	// Expected prefix: 8 digits + "T" + 6 digits + "Z" = 16 chars (e.g. "20260101T120000Z").
	const stampLen = 16
	if len(stem) > stampLen+1 && stem[stampLen] == '-' {
		raw := stem[:stampLen]
		// Parse date portion: first 8 chars "YYYYMMDD".
		if len(raw) >= 8 {
			date = raw[0:4] + "-" + raw[4:6] + "-" + raw[6:8]
		}
		slug = stem[stampLen+1:]
		return
	}

	// No timestamp prefix: use stem as slug, date empty.
	slug = stem
	return
}

// readPlanTitle reads the first non-empty line from a plan file and strips the
// leading "# " Markdown heading marker if present.
func readPlanTitle(path string) string {
	//nolint:gosec // G304: path is derived from os.ReadDir output — trusted
	f, err := os.Open(path)
	if err != nil {
		return "(unreadable)"
	}
	defer func() { _ = f.Close() }()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		return strings.TrimPrefix(line, "# ")
	}
	return "(empty)"
}

// ─── plan show ────────────────────────────────────────────────────────────────

// newPlanShowCmd implements `ai plan show <slug>`.
// Resolution order: (1) <slug>.md direct match, (2) *-<slug>.md glob match.
func newPlanShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show <slug>",
		Short: "Print a plan to stdout",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			slug := args[0]
			dir := plansDir()

			path, err := resolvePlanFile(dir, slug)
			if err != nil {
				return fmt.Errorf("plan show: %w", err)
			}

			//nolint:gosec // G304: path is resolved from plansDir + user slug — trusted
			data, err := os.ReadFile(path)
			if err != nil {
				return fmt.Errorf("plan show: read %s: %w", path, err)
			}

			_, _ = fmt.Fprint(cmd.OutOrStdout(), string(data))
			if !strings.HasSuffix(string(data), "\n") {
				_, _ = fmt.Fprintln(cmd.OutOrStdout())
			}
			return nil
		},
	}
}

// resolvePlanFile resolves a slug to an absolute plan file path.
// Resolution order:
//  1. <dir>/<slug>.md (exact match)
//  2. <dir>/*-<slug>.md (suffix match on any timestamped filename)
func resolvePlanFile(dir, slug string) (string, error) {
	// 1. Direct match.
	direct := filepath.Join(dir, slug+".md")
	if _, err := os.Stat(direct); err == nil {
		return direct, nil
	}

	// 2. Walk for a timestamped file ending in -<slug>.md.
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("plan %q not found (plans dir does not exist)", slug)
		}
		return "", fmt.Errorf("read plans dir: %w", err)
	}

	suffix := "-" + slug + ".md"
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), suffix) {
			return filepath.Join(dir, e.Name()), nil
		}
	}

	return "", fmt.Errorf("plan %q not found", slug)
}
