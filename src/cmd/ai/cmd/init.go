package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

// newInitCmd implements `ai init` — project.yaml scaffold (#213) and
// AI-tool integration file generation (#214).
//
// Stack detection (checks cwd):
//   go.mod          → Go    → go test ./...
//   package.json    → Node  → npm test
//   pyproject.toml  → Python → pytest
//   requirements.txt → Python → pytest
//   (none)          → unknown → # TODO: set test_command
//
// Integration files written (idempotent):
//   .claude/CLAUDE.md               — @~/.ai/Constitution.md @-include
//   .cursor/rules/constitution.md   — symlink to ~/.ai/Constitution.runtime.md
//   .github/copilot-instructions.md — @-include header
//
// --dry-run prints what would be written without creating any files.
func newInitCmd() *cobra.Command {
	var dryRun bool

	c := &cobra.Command{
		Use:   "init",
		Short: "Scaffold project.yaml and AI-tool integration files in the current directory",
		Long: `init writes project.yaml (stack-detected) and the canonical AI-tool
integration files (.claude/CLAUDE.md, .cursor/rules/constitution.md,
.github/copilot-instructions.md) into the current working directory.

All writes are idempotent: running init twice on the same project is safe.
Use --dry-run to preview changes without writing anything.

Stack detection:
  go.mod             → Go
  package.json       → Node
  pyproject.toml     → Python
  requirements.txt   → Python
  (none of the above) → unknown

See issues #213 and #214.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("init: could not determine working directory: %w", err)
			}
			return runInit(cmd, cwd, dryRun)
		},
	}

	c.Flags().BoolVar(&dryRun, "dry-run", false, "print what would be written without creating files")

	return c
}

// runInit is the testable core of `ai init`. It operates on an explicit cwd
// rather than calling os.Getwd() internally, which makes it easy to test
// without changing the process working directory.
func runInit(cmd *cobra.Command, cwd string, dryRun bool) error {
	stack, testCmd := detectStack(cwd)

	// --- project.yaml (#213) ---
	projectYAML := filepath.Join(cwd, "project.yaml")
	if _, err := os.Stat(projectYAML); err == nil {
		// Already exists — idempotent exit.
		fmt.Fprintln(cmd.OutOrStdout(), "project.yaml already exists — skipping scaffold.")
		return nil
	}

	name := filepath.Base(cwd)
	yamlContent := buildProjectYAML(name, stack, testCmd)

	if dryRun {
		fmt.Fprintf(cmd.OutOrStdout(), "[dry-run] would write %s:\n%s\n", projectYAML, yamlContent)
	} else {
		if err := os.WriteFile(projectYAML, []byte(yamlContent), 0o644); err != nil {
			return fmt.Errorf("init: write project.yaml: %w", err)
		}
	}

	// --- .claude/CLAUDE.md (#214) ---
	claudeDir := filepath.Join(cwd, ".claude")
	claudeMD := filepath.Join(claudeDir, "CLAUDE.md")
	const claudeInclude = "@~/.ai/Constitution.md\n"

	if dryRun {
		if _, err := os.Stat(claudeMD); os.IsNotExist(err) {
			fmt.Fprintf(cmd.OutOrStdout(), "[dry-run] would write %s\n", claudeMD)
		} else {
			fmt.Fprintf(cmd.OutOrStdout(), "[dry-run] would ensure %q is in %s\n", strings.TrimSpace(claudeInclude), claudeMD)
		}
	} else {
		if err := ensureFileContains(claudeDir, claudeMD, claudeInclude, 0o750, 0o644); err != nil {
			return fmt.Errorf("init: write .claude/CLAUDE.md: %w", err)
		}
	}

	// --- .cursor/rules/constitution.md symlink (#214) ---
	cursorDir := filepath.Join(cwd, ".cursor", "rules")
	cursorLink := filepath.Join(cursorDir, "constitution.md")
	const cursorTarget = "~/.ai/Constitution.runtime.md"

	if dryRun {
		fmt.Fprintf(cmd.OutOrStdout(), "[dry-run] would create symlink %s -> %s\n", cursorLink, cursorTarget)
	} else {
		if err := os.MkdirAll(cursorDir, 0o750); err != nil {
			return fmt.Errorf("init: mkdir .cursor/rules: %w", err)
		}
		if _, err := os.Lstat(cursorLink); os.IsNotExist(err) {
			if err := os.Symlink(cursorTarget, cursorLink); err != nil {
				// Symlink failure is non-fatal (e.g., unsupported FS); warn and continue.
				fmt.Fprintf(cmd.ErrOrStderr(), "init: warning: could not create symlink %s: %v\n", cursorLink, err)
			}
		}
	}

	// --- .github/copilot-instructions.md (#214) ---
	ghDir := filepath.Join(cwd, ".github")
	copilotMD := filepath.Join(ghDir, "copilot-instructions.md")
	const copilotContent = "<!-- Load constitution -->\n@~/.ai/Constitution.runtime.md\n"

	if dryRun {
		if _, err := os.Stat(copilotMD); os.IsNotExist(err) {
			fmt.Fprintf(cmd.OutOrStdout(), "[dry-run] would write %s\n", copilotMD)
		} else {
			fmt.Fprintf(cmd.OutOrStdout(), "[dry-run] would ensure @-include is in %s\n", copilotMD)
		}
	} else {
		if err := ensureFileContains(ghDir, copilotMD, copilotContent, 0o750, 0o644); err != nil {
			return fmt.Errorf("init: write .github/copilot-instructions.md: %w", err)
		}
	}

	return nil
}

// detectStack returns the stack name and test command for the project in dir.
func detectStack(dir string) (stack, testCmd string) {
	checks := []struct {
		file    string
		stack   string
		testCmd string
	}{
		{"go.mod", "go", "go test ./..."},
		{"package.json", "node", "npm test"},
		{"pyproject.toml", "python", "pytest"},
		{"requirements.txt", "python", "pytest"},
	}
	for _, c := range checks {
		if _, err := os.Stat(filepath.Join(dir, c.file)); err == nil {
			return c.stack, c.testCmd
		}
	}
	return "unknown", ""
}

// buildProjectYAML returns the YAML content for project.yaml.
func buildProjectYAML(name, stack, testCmd string) string {
	var sb strings.Builder
	sb.WriteString("name: " + name + "\n")
	sb.WriteString("stack: " + stack + "\n")
	sb.WriteString("tooling:\n")
	if testCmd == "" {
		sb.WriteString("  test_command: # TODO: set test_command\n")
	} else {
		sb.WriteString("  test_command: " + testCmd + "\n")
	}
	return sb.String()
}

// ensureFileContains writes content to filePath if the file does not exist, or
// appends content to filePath if the file exists but does not already contain
// the content. The directory dir is created with dirPerm if needed.
func ensureFileContains(dir, filePath, content string, dirPerm, filePerm os.FileMode) error {
	if err := os.MkdirAll(dir, dirPerm); err != nil {
		return err
	}

	existing, err := os.ReadFile(filePath) //nolint:gosec // G304: filePath is caller-constructed from cwd
	if os.IsNotExist(err) {
		// New file.
		return os.WriteFile(filePath, []byte(content), filePerm) //nolint:gosec // G306
	}
	if err != nil {
		return err
	}

	// Already contains the include line — idempotent.
	if strings.Contains(string(existing), strings.TrimSpace(content)) {
		return nil
	}

	// Append.
	combined := strings.TrimRight(string(existing), "\n") + "\n" + content
	return os.WriteFile(filePath, []byte(combined), filePerm) //nolint:gosec // G306
}
