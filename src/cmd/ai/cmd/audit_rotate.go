package cmd

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/convergent-systems-co/aiConstitution/src/internal/paths"

	"github.com/spf13/cobra"
)

// retentionDays is the rolling window for interaction logs. Files
// whose YYYY-MM cohort lies entirely before today minus this many
// days are deleted. Hard-coded for v0.8; a settings.toml knob is
// a future task per SPEC.md §17.4.
const retentionDays = 30

// newAuditRotateCmd implements `ai audit rotate`. Per Common.md §5.2,
// interaction logs are local-only audit data; they accumulate quickly
// and the 30-day window is the published retention budget. This
// command deletes interaction JSONL cohorts (one file per
// YYYY-MM) older than retentionDays.
//
// Behavior change in v0.8: prior versions gzipped non-current months;
// the new behavior deletes per the task spec for #172. The audit log
// retains its tail in-process (kept by the running hook); rotated
// historical evidence flows into the override / violation records
// (audit/{overrides,violations}/), which are tracked and synced.
func newAuditRotateCmd() *cobra.Command {
	var dryRun bool
	c := &cobra.Command{
		Use:   "rotate",
		Short: "Delete interaction-log cohorts older than the retention window",
		Long: `rotate deletes audit/interactions/<YYYY-MM>.jsonl(.gz) files whose
month cohort lies entirely before (today - 30 days). Idempotent.

The current month is always preserved. The retention window is 30
days; the published rationale lives in Common.md §5.2.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			dir := filepath.Join(paths.AuditDir(), "interactions")
			info, err := os.Stat(dir)
			if err != nil {
				if errors.Is(err, os.ErrNotExist) {
					_, _ = fmt.Fprintln(cmd.OutOrStdout(), "(no interactions/ directory; nothing to do)")
					return nil
				}
				return fmt.Errorf("audit rotate: %w", err)
			}
			if !info.IsDir() {
				return fmt.Errorf("audit rotate: %s is not a directory", dir)
			}

			cutoff := time.Now().UTC().AddDate(0, 0, -retentionDays)
			removed, err := pruneInteractions(cmd.OutOrStdout(), dir, cutoff, dryRun)
			if err != nil {
				return fmt.Errorf("audit rotate: %w", err)
			}
			if removed == 0 {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "audit rotate: nothing to do.")
			}
			return nil
		},
	}
	c.Flags().BoolVar(&dryRun, "dry-run", false, "show what would be deleted without removing files")
	return c
}

// pruneInteractions iterates the interaction-log directory, parses each
// filename's YYYY-MM cohort, and deletes files whose cohort precedes
// the cutoff. Returns the number of files removed (or that would be
// removed in dry-run).
func pruneInteractions(out io.Writer, dir string, cutoff time.Time, dryRun bool) (int, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0, err
	}
	removed := 0
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		cohort, ok := interactionCohort(e.Name())
		if !ok {
			continue
		}
		// A cohort is "older than" the cutoff if the END of its month
		// is still before the cutoff. Otherwise the current month
		// (and the immediately-prior one within the window) survive.
		endOfMonth := cohort.AddDate(0, 1, 0).Add(-time.Nanosecond)
		if !endOfMonth.Before(cutoff) {
			continue
		}
		path := filepath.Join(dir, e.Name())
		if dryRun {
			_, _ = fmt.Fprintf(out, "[would delete] %s\n", path)
			removed++
			continue
		}
		if err := os.Remove(path); err != nil {
			return removed, fmt.Errorf("remove %s: %w", path, err)
		}
		_, _ = fmt.Fprintf(out, "[deleted] %s\n", path)
		removed++
	}
	return removed, nil
}

// interactionCohort parses a filename of the form `YYYY-MM.jsonl` (or
// `YYYY-MM.jsonl.gz`) and returns the first-of-month UTC time for
// that cohort, plus a true flag. Filenames that don't fit the shape
// are skipped (ok=false).
func interactionCohort(name string) (time.Time, bool) {
	base := name
	switch {
	case strings.HasSuffix(base, ".jsonl.gz"):
		base = strings.TrimSuffix(base, ".jsonl.gz")
	case strings.HasSuffix(base, ".jsonl"):
		base = strings.TrimSuffix(base, ".jsonl")
	default:
		return time.Time{}, false
	}
	t, err := time.Parse("2006-01", base)
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}
