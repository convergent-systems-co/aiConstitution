package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/convergent-systems-co/aiConstitution/src/internal/constitution"
	"github.com/convergent-systems-co/aiConstitution/src/internal/paths"
	"github.com/spf13/cobra"
)

// newDoctorCmd implements `ai doctor`. See SPEC.md §3.3.
func newDoctorCmd() *cobra.Command {
	var fix bool
	var resetHead string

	c := &cobra.Command{
		Use:   "doctor",
		Short: "Detect and repair structural damage to the ~/.ai/ tree",
		Long: `doctor checks the predictable failure modes of the constitution
tree and either reports them or fixes them.

Checks currently implemented:
  1. Constitution files present (~/.ai/{Constitution,Common,Code,Writing}.md)
  2. ~/.ai/bin/ on PATH ahead of /usr/local/bin and /opt/homebrew/bin

See SPEC.md §3.3 for the full 10-point check list (in progress).`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			_ = fix
			_ = resetHead
			root := paths.AIRoot()
			status := constitution.FileStatus(root)
			out := cmd.OutOrStdout()

			allOK := true
			for _, name := range constitution.FileNames {
				present := status[name]
				mark := "✓"
				if !present {
					mark = "✗"
					allOK = false
				}
				_, _ = fmt.Fprintf(out, "  [%s] %s\n", mark, name)
			}
			if status["Constitution.local.md"] {
				_, _ = fmt.Fprintf(out, "  [✓] Constitution.local.md (local override)\n")
			}

			// Bin-PATH check is a warn (not fatal): doctor still reports
			// it but only the constitution-files check returns an error.
			printBinPathCheck(out, paths.BinDir(), os.Getenv("PATH"))

			if !allOK {
				return fmt.Errorf("doctor: missing required constitution files in %s", root)
			}
			_, _ = fmt.Fprintln(out, "Constitution files: OK")
			return nil
		},
	}

	c.Flags().BoolVar(&fix, "fix", false, "attempt to repair each detected issue")
	c.Flags().StringVar(&resetHead, "reset-head", "", "reset HEAD to <ref> (use with caution)")
	return c
}

// PathStatus describes the result of checkBinPath.
type PathStatus int

// PathStatus values.
const (
	// PathOK: binDir is on PATH and earlier than the listed system bin
	// dirs (or those system dirs are absent from PATH).
	PathOK PathStatus = iota
	// PathMissing: binDir is not on PATH at all.
	PathMissing
	// PathShadowed: binDir is on PATH but appears AFTER one of the
	// system bin dirs that contain real `git` / `gh` — meaning the
	// wrapper interception will not fire.
	PathShadowed
)

// checkBinPath determines whether binDir is on pathVar (the PATH env
// var contents) and ahead of the listed system bin dirs.
//
// Comparison is canonicalized via filepath.Clean. The first matching
// system dir found AFTER binDir (or the absence of binDir entirely)
// dictates the status.
func checkBinPath(binDir, pathVar string) (status PathStatus, message string) {
	if binDir == "" {
		return PathOK, ""
	}
	binDir = filepath.Clean(binDir)
	systemBins := []string{"/usr/local/bin", "/opt/homebrew/bin"}

	entries := strings.Split(pathVar, string(os.PathListSeparator))
	binIdx := -1
	systemIdxs := map[string]int{}
	for i, e := range entries {
		clean := filepath.Clean(strings.TrimSpace(e))
		if clean == "." || clean == "" {
			continue
		}
		if clean == binDir && binIdx < 0 {
			binIdx = i
		}
		for _, s := range systemBins {
			if clean == s {
				if _, seen := systemIdxs[s]; !seen {
					systemIdxs[s] = i
				}
			}
		}
	}

	if binIdx < 0 {
		return PathMissing, fmt.Sprintf("%s is not on PATH — wrapper interception will not fire", binDir)
	}
	for _, s := range systemBins {
		if si, ok := systemIdxs[s]; ok && si < binIdx {
			return PathShadowed, fmt.Sprintf("%s is on PATH but %s appears earlier — move %s before it", binDir, s, binDir)
		}
	}
	return PathOK, fmt.Sprintf("%s is on PATH before system bins", binDir)
}

// printBinPathCheck emits the doctor row for the PATH check.
func printBinPathCheck(out io.Writer, binDir, pathVar string) {
	status, msg := checkBinPath(binDir, pathVar)
	mark := "✓"
	if status != PathOK {
		mark = "!"
	}
	_, _ = fmt.Fprintf(out, "  [%s] %s\n", mark, msg)
}
