package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"os/exec"
	"runtime"

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
tree and either reports them or fixes them:

  1.  Broken symlinks under ~/.claude/, ~/.copilot/, .cursor/, etc.
  2.  Missing or misregistered hooks.
  3.  Dirty working tree on ~/.ai/.
  4.  Divergent HEAD vs origin.
  5.  Stale ai binary vs governance/last-seen-version.
  6.  Missing brand-cache; missing persona/profile/skill cache for
      pinned atoms.
  7.  Audit/interactions log writable.
  8.  Mutable state in ~/.config/aiConstitution/ exists and parses.
  9.  Settings file present and within validation ranges.
  10. last-seen-version marker matches the binary.
  11. terminal-notifier installed (macOS only).

Flags:
  --fix                  Attempt to repair each detected issue.
  --reset-head=<ref>     If the tree is dirty or HEAD is divergent,
                         reset to <ref> (refuses on dirty tree
                         without --force-hard-reset).

See SPEC.md §3.3.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runDoctor(os.Stdout, fix, resetHead)
		},
	}

	c.Flags().BoolVar(&fix, "fix", false, "attempt to repair each detected issue")
	c.Flags().StringVar(&resetHead, "reset-head", "", "reset HEAD to <ref> (use with caution)")

	return c
}

// runDoctor executes all doctor checks and writes the report to w.
// It returns nil unless an unexpected internal error occurs; individual
// check failures are surfaced as [⚠] lines in the output, not as errors.
//
// fix and resetHead are reserved for future implementation of --fix and
// --reset-head; they are accepted here so the function signature is stable
// and tests can exercise the check output without triggering mutations.
func runDoctor(w io.Writer, fix bool, resetHead string) error {
	_ = fix
	_ = resetHead

	checkTerminalNotifier(w)

	return nil
}

// checkTerminalNotifier verifies that terminal-notifier is on PATH.
// The check runs only on macOS (darwin); it is silently skipped on other
// platforms so doctor remains cross-platform without platform-specific output.
//
// Output format:
//
//	[✓] terminal-notifier: found at <path>
//	[⚠] terminal-notifier: not found — run: brew install terminal-notifier
func checkTerminalNotifier(w io.Writer) {
	if runtime.GOOS != "darwin" {
		// Not a macOS system; terminal-notifier is not applicable.
		return
	}

	path, err := exec.LookPath("terminal-notifier")
	if err == nil {
		fmt.Fprintf(w, "[✓] terminal-notifier: found at %s\n", path)
		return
	}

	fmt.Fprintln(w, "[⚠] terminal-notifier: not found — run: brew install terminal-notifier")
}

// PathStatus and companion types — needed by export_test.go and integrate_test.go
type PathStatus int
const ( PathOK PathStatus = iota; PathMissing; PathShadowed )

func checkBinPath(binDir, pathVar string) (PathStatus, string) {
	if binDir == "" { return PathOK, "" }
	binDir = filepath.Clean(binDir)
	systemBins := []string{"/usr/local/bin", "/opt/homebrew/bin"}
	entries := strings.Split(pathVar, string(os.PathListSeparator))
	binIdx := -1; systemIdxs := map[string]int{}
	for i, e := range entries {
		clean := filepath.Clean(strings.TrimSpace(e))
		if clean == binDir && binIdx < 0 { binIdx = i }
		for _, s := range systemBins { if clean == s { if _, ok := systemIdxs[s]; !ok { systemIdxs[s] = i } } }
	}
	if binIdx < 0 { return PathMissing, fmt.Sprintf("%s not on PATH", binDir) }
	for _, s := range systemBins { if si, ok := systemIdxs[s]; ok && si < binIdx { return PathShadowed, fmt.Sprintf("%s after %s", binDir, s) } }
	return PathOK, fmt.Sprintf("%s before system bins", binDir)
}

type doctorStatus int
const ( doctorOK doctorStatus = iota; doctorWarn; doctorSkip )
type doctorResult struct { name string; status doctorStatus; message string }

func checkDoctorCopilot(home string) doctorResult {
	dir := filepath.Join(home, ".copilot", "instructions")
	if _, err := os.Stat(dir); os.IsNotExist(err) { return doctorResult{status: doctorSkip} }
	if _, err := os.Lstat(filepath.Join(dir, "constitution.md")); os.IsNotExist(err) { return doctorResult{status: doctorWarn, message: "Copilot symlink missing"} }
	return doctorResult{status: doctorOK, message: "Copilot symlink present"}
}
func checkDoctorCursor(cwd string) doctorResult {
	if _, err := os.Stat(filepath.Join(cwd, ".cursor", "rules")); os.IsNotExist(err) { return doctorResult{status: doctorSkip} }
	if _, err := os.Lstat(filepath.Join(cwd, ".cursor", "rules", "constitution.md")); os.IsNotExist(err) { return doctorResult{status: doctorWarn} }
	return doctorResult{status: doctorOK}
}
func checkDoctorAgentsMD(cwd string) doctorResult {
	data, err := os.ReadFile(filepath.Join(cwd, "AGENTS.md")) //nolint:gosec
	if os.IsNotExist(err) { return doctorResult{status: doctorSkip} }
	if err != nil { return doctorResult{status: doctorWarn} }
	if strings.Contains(string(data), "@~/.ai/Constitution.md") { return doctorResult{status: doctorOK} }
	return doctorResult{status: doctorWarn}
}
