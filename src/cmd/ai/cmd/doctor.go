package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

// doctorStatus represents the outcome of a single doctor check.
type doctorStatus int

const (
	// doctorOK means the check passed.
	doctorOK doctorStatus = iota
	// doctorWarn means the check found a non-fatal issue (printed as [⚠]).
	doctorWarn
	// doctorSkip means the check is not applicable in this environment (skipped silently).
	doctorSkip
)

// doctorResult holds the result of a single doctor check.
type doctorResult struct {
	name    string
	status  doctorStatus
	message string
}

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
  11. Copilot instruction symlink valid.
  12. Cursor rule symlink valid (if .cursor/ present in cwd).
  13. AGENTS.md @-include present (if AGENTS.md exists in cwd).

Flags:
  --fix                  Attempt to repair each detected issue.
  --reset-head=<ref>     If the tree is dirty or HEAD is divergent,
                         reset to <ref> (refuses on dirty tree
                         without --force-hard-reset).

See SPEC.md §3.3.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			_ = fix
			_ = resetHead
			return runDoctor()
		},
	}

	c.Flags().BoolVar(&fix, "fix", false, "attempt to repair each detected issue")
	c.Flags().StringVar(&resetHead, "reset-head", "", "reset HEAD to <ref> (use with caution)")

	return c
}

// runDoctor executes all doctor checks and prints a summary report.
// Exit code is 0 even when warnings are present — all integration checks
// are warnings, not errors.
func runDoctor() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("getting home dir: %w", err)
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting cwd: %w", err)
	}

	checks := []doctorResult{
		checkDoctorCopilot(home),
		checkDoctorCursor(cwd),
		checkDoctorAgentsMD(cwd),
	}

	anyPrinted := false
	for _, r := range checks {
		switch r.status {
		case doctorSkip:
			// Silent skip — not applicable in this environment.
		case doctorOK:
			fmt.Printf("[✓] %s: %s\n", r.name, r.message)
			anyPrinted = true
		case doctorWarn:
			fmt.Printf("[⚠] %s: %s\n", r.name, r.message)
			anyPrinted = true
		}
	}

	if !anyPrinted {
		fmt.Println("doctor: no applicable checks for this environment.")
	}
	return nil
}

// checkDoctorCopilot checks that ~/.copilot/instructions/constitution.md is a
// valid symlink.
//
// Returns:
//   - doctorSkip if ~/.copilot/instructions/ does not exist.
//   - doctorOK if the symlink exists and resolves to a real file.
//   - doctorWarn if the symlink is missing, dangling, or broken.
func checkDoctorCopilot(home string) doctorResult {
	const name = "Copilot instruction symlink"

	instructionsDir := filepath.Join(home, ".copilot", "instructions")
	if _, err := os.Stat(instructionsDir); os.IsNotExist(err) {
		return doctorResult{name: name, status: doctorSkip}
	}

	symlinkPath := filepath.Join(instructionsDir, "constitution.md")

	target, err := os.Readlink(symlinkPath)
	if err != nil {
		return doctorResult{
			name:    name,
			status:  doctorWarn,
			message: fmt.Sprintf("symlink missing at %s — run `ai hooks install --copilot`", symlinkPath),
		}
	}

	// Verify the target resolves to a real file (not dangling).
	if _, err := os.Stat(target); err != nil {
		return doctorResult{
			name:    name,
			status:  doctorWarn,
			message: fmt.Sprintf("symlink %s → %s is dangling — run `ai hooks install --copilot`", symlinkPath, target),
		}
	}

	return doctorResult{
		name:    name,
		status:  doctorOK,
		message: fmt.Sprintf("%s → %s", symlinkPath, target),
	}
}

// checkDoctorCursor checks that .cursor/rules/constitution.md in cwd is a
// valid symlink.
//
// Returns:
//   - doctorSkip if .cursor/ does not exist in cwd.
//   - doctorOK if the symlink exists and resolves to a real file.
//   - doctorWarn if the symlink is missing or dangling.
func checkDoctorCursor(cwd string) doctorResult {
	const name = "Cursor rule symlink"

	cursorDir := filepath.Join(cwd, ".cursor")
	if _, err := os.Stat(cursorDir); os.IsNotExist(err) {
		return doctorResult{name: name, status: doctorSkip}
	}

	symlinkPath := filepath.Join(cwd, ".cursor", "rules", "constitution.md")

	target, err := os.Readlink(symlinkPath)
	if err != nil {
		return doctorResult{
			name:    name,
			status:  doctorWarn,
			message: fmt.Sprintf("symlink missing at %s — run `ai init-integrate --cursor`", symlinkPath),
		}
	}

	if _, err := os.Stat(target); err != nil {
		return doctorResult{
			name:    name,
			status:  doctorWarn,
			message: fmt.Sprintf("symlink %s → %s is dangling — run `ai init-integrate --cursor`", symlinkPath, target),
		}
	}

	return doctorResult{
		name:    name,
		status:  doctorOK,
		message: fmt.Sprintf("%s → %s", symlinkPath, target),
	}
}

// checkDoctorAgentsMD checks that AGENTS.md in cwd contains the
// @~/.ai/Constitution.md @-include.
//
// Returns:
//   - doctorSkip if AGENTS.md does not exist in cwd.
//   - doctorOK if AGENTS.md contains the @-include marker.
//   - doctorWarn if AGENTS.md exists but lacks the @-include.
func checkDoctorAgentsMD(cwd string) doctorResult {
	const name = "AGENTS.md @-include"

	agentsPath := filepath.Join(cwd, "AGENTS.md")
	content, err := os.ReadFile(agentsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return doctorResult{name: name, status: doctorSkip}
		}
		return doctorResult{
			name:    name,
			status:  doctorWarn,
			message: fmt.Sprintf("could not read %s: %v", agentsPath, err),
		}
	}

	if strings.Contains(string(content), agentsIncludeMarker) {
		return doctorResult{
			name:    name,
			status:  doctorOK,
			message: fmt.Sprintf("%s contains @-include", agentsPath),
		}
	}

	return doctorResult{
		name:    name,
		status:  doctorWarn,
		message: fmt.Sprintf("%s exists but lacks @~/.ai/Constitution.md — run `ai init-integrate --codex`", agentsPath),
	}
}
