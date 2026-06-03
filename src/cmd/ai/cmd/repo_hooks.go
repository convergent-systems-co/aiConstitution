package cmd

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type repoHookTemplate struct {
	name string
	mode os.FileMode
	body string
}

var repoManagedGitHookTemplates = []repoHookTemplate{
	{
		name: "pre-commit",
		mode: 0o755,
		body: `#!/bin/sh
set -eu

exec ai guard --git-hook pre-commit
`,
	},
	{
		name: "pre-push",
		mode: 0o755,
		body: `#!/bin/sh
set -eu

exec ai guard --git-hook pre-push "$@"
`,
	},
	{
		name: "commit-msg",
		mode: 0o755,
		body: `#!/bin/sh
set -eu

message_file="${1:?commit message file required}"

if grep -Eiq '(^|[[:space:]])(AI-authored|Generated-by:.*(AI|Codex|Copilot|Claude)|Co-authored-by:.*(OpenAI|Codex|Copilot|Claude))' "$message_file"; then
	if ! grep -Eq '^Co-authored-by: .+ <.+>$' "$message_file"; then
		printf '%s\n' "commit-msg: AI-authored commits require a Co-authored-by trailer" >&2
		exit 1
	fi
fi
`,
	},
	{
		name: "pre-commit.ps1",
		mode: 0o644,
		body: `$ErrorActionPreference = "Stop"
& ai guard --git-hook pre-commit
exit $LASTEXITCODE
`,
	},
	{
		name: "pre-push.ps1",
		mode: 0o644,
		body: `param(
    [Parameter(ValueFromRemainingArguments = $true)]
    [string[]] $HookArgs
)

$ErrorActionPreference = "Stop"
& ai guard --git-hook pre-push @HookArgs
exit $LASTEXITCODE
`,
	},
	{
		name: "commit-msg.ps1",
		mode: 0o644,
		body: `param(
    [Parameter(Mandatory = $true)]
    [string] $MessageFile
)

$ErrorActionPreference = "Stop"
$message = Get-Content -LiteralPath $MessageFile -Raw
$aiAuthored = $message -match '(^|\s)(AI-authored|Generated-by:.*(AI|Codex|Copilot|Claude)|Co-authored-by:.*(OpenAI|Codex|Copilot|Claude))'
$hasTrailer = $message -match '(?m)^Co-authored-by: .+ <.+>$'

if ($aiAuthored -and -not $hasTrailer) {
    Write-Error "commit-msg: AI-authored commits require a Co-authored-by trailer"
    exit 1
}
`,
	},
}

func installRepoManagedGitHooks(repoDir, source string, out io.Writer) error {
	hooksPath, err := repoGitHooksPath(repoDir)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(hooksPath, 0o750); err != nil {
		return fmt.Errorf("create hooks dir: %w", err)
	}

	for _, hook := range repoManagedGitHookTemplates {
		dst := filepath.Join(hooksPath, hook.name)
		if _, err := os.Stat(dst); err == nil {
			fmt.Fprintf(out, "%s already present at %s - leaving in place\n", hook.name, dst)
			continue
		}
		body := repoHookBodyWithSource(hook.body, source)
		if err := os.WriteFile(filepath.Clean(dst), []byte(body), hook.mode); err != nil { //nolint:gosec // G306: git hooks require executable bits on POSIX templates.
			return fmt.Errorf("write %s: %w", dst, err)
		}
		fmt.Fprintf(out, "installed %s\n", dst)
	}
	return nil
}

func repoHookBodyWithSource(body, source string) string {
	comment := fmt.Sprintf("# Installed by `%s`.\n", source)
	if strings.HasPrefix(body, "#!") {
		if idx := strings.IndexByte(body, '\n'); idx >= 0 {
			return body[:idx+1] + comment + body[idx+1:]
		}
	}
	return comment + body
}

func repoGitHooksPath(repoDir string) (string, error) {
	cmd := exec.Command("git", "-C", repoDir, "rev-parse", "--git-path", "hooks") //nolint:gosec // repoDir is the user-selected repository path.
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("%s is not a git repo", repoDir)
	}
	hooksPath := strings.TrimSpace(string(out))
	if filepath.IsAbs(hooksPath) {
		return hooksPath, nil
	}
	return filepath.Join(repoDir, hooksPath), nil
}
