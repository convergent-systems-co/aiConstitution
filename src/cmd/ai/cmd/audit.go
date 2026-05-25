package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/convergent-systems-co/aiConstitution/src/internal/paths"

	"github.com/spf13/cobra"
)

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
		&cobra.Command{
			Use:   "override",
			Short: "Record an override (writes audit/overrides/<UTC>.md)",
			RunE: func(cmd *cobra.Command, _ []string) error {
				notice("audit override:", "would prompt for the canonical override fields")
				return stub("audit override", "Constitution.md §5.1")
			},
		},
		&cobra.Command{
			Use:   "violation",
			Short: "Record a self-noticed violation (writes audit/violations/<UTC>.md)",
			RunE: func(cmd *cobra.Command, _ []string) error {
				notice("audit violation:", "would prompt for the canonical violation fields")
				return stub("audit violation", "Constitution.md §5.2")
			},
		},
		newAuditListCmd(),
		newAuditShowCmd(),
		newAuditRotateCmd(),
	)
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
