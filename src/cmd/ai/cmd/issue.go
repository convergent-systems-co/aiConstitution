package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
)

// IssueBaseURL is the base GitHub API URL used by `ai issue file`.
// Tests may override this to point at an httptest server.
var IssueBaseURL = "https://api.github.com"

// reGitHubHTTPS matches https://github.com/owner/repo.git or
// https://github.com/owner/repo
var reGitHubHTTPS = regexp.MustCompile(`https://github\.com/([^/]+/[^/\s]+?)(?:\.git)?$`)

// reGitHubSSH matches git@github.com:owner/repo.git
var reGitHubSSH = regexp.MustCompile(`git@github\.com:([^/\s]+/[^/\s]+?)(?:\.git)?$`)

// newIssueCmd implements `ai issue file`. See SPEC.md §3.12 + §9.5.
func newIssueCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "issue",
		Short: "File hook / finding issues upstream",
		Long: `issue is the direct surface for upstreaming hooks and major
findings. Bodies are redacted against hooks/patterns.json before
submission; the user reviews the body unless
settings.upstream.skipReviewWindow=true.

See SPEC.md §3.12 + §9.5.`,
	}

	var fileType string
	var major bool
	var title string
	var body string
	var fromAudit string

	file := &cobra.Command{
		Use:   "file",
		Short: "File an issue on the current repo",
		Long: `file creates a GitHub issue on the repository inferred from
'git remote get-url origin'. Labels are derived from --type (kind/<type>)
and optionally --major (severity/major).

Examples:
  ai issue file --title "Bug in hook" --type bug
  ai issue file --title "From audit" --from-audit ~/.ai/audit/violations/2026-01-01T00:00:00Z.md`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runIssueFile(cmd, title, body, fileType, fromAudit, major)
		},
	}
	file.Flags().StringVar(&fileType, "type", "task", "issue type: bug|finding|task|story")
	file.Flags().BoolVar(&major, "major", false, "add severity/major label")
	file.Flags().StringVar(&title, "title", "", "issue title (required)")
	file.Flags().StringVar(&body, "body", "", "issue body (reads stdin if omitted)")
	file.Flags().StringVar(&fromAudit, "from-audit", "", "path to an audit violations/*.md or overrides/*.md file; populates body")
	if err := file.MarkFlagRequired("title"); err != nil {
		panic("issue file: MarkFlagRequired title: " + err.Error())
	}

	c.AddCommand(file)
	return c
}

// runIssueFile is the implementation of `ai issue file`.
func runIssueFile(cmd *cobra.Command, title, body, fileType, fromAudit string, major bool) error {
	// 1. Resolve body.
	issueBody, err := resolveIssueBody(body, fromAudit, cmd.InOrStdin())
	if err != nil {
		return err
	}

	// 2. Detect owner/repo from git remote.
	ownerRepo, err := detectOwnerRepo()
	if err != nil {
		return fmt.Errorf("issue file: %w", err)
	}

	// 3. Resolve token.
	token, err := getShareToken()
	if err != nil {
		return err
	}

	// 4. Build labels.
	labels := []string{"kind/" + fileType}
	if major {
		labels = append(labels, "severity/major")
	}

	// 5. Post to GitHub.
	issueURL, err := postIssueToRepo(ownerRepo, token, title, issueBody, labels)
	if err != nil {
		return err
	}

	fmt.Fprintln(cmd.OutOrStdout(), issueURL)
	return nil
}

// resolveIssueBody builds the final issue body from the flag, stdin, or audit file.
func resolveIssueBody(body, fromAudit string, stdin io.Reader) (string, error) {
	if fromAudit != "" {
		data, err := os.ReadFile(fromAudit) //nolint:gosec // G304: user-supplied audit file path
		if err != nil {
			return "", fmt.Errorf("issue file: read audit file %s: %w", fromAudit, err)
		}
		return "From audit log:\n\n" + string(data), nil
	}
	if body != "" {
		return body, nil
	}
	// Read from stdin.
	data, err := io.ReadAll(stdin)
	if err != nil {
		return "", fmt.Errorf("issue file: read body from stdin: %w", err)
	}
	return strings.TrimRight(string(data), "\n"), nil
}

// detectOwnerRepo resolves "owner/repo" from `git remote get-url origin`.
// Environment variable ISSUE_GIT_WORK_DIR sets the working directory for the
// git command (used in tests). GIT_DIR is passed through if set.
func detectOwnerRepo() (string, error) {
	workDir := os.Getenv("ISSUE_GIT_WORK_DIR")

	gitCmd := exec.Command("git", "remote", "get-url", "origin") //nolint:gosec // G204: fixed args
	if workDir != "" {
		gitCmd.Dir = workDir
	}
	// Inherit the full environment so GIT_DIR, HOME, etc. are available.
	gitCmd.Env = os.Environ()

	out, err := gitCmd.Output()
	if err != nil {
		return "", fmt.Errorf("detect repo: run git remote get-url origin: %w", err)
	}

	remote := strings.TrimSpace(string(out))
	return parseOwnerRepo(remote)
}

// parseOwnerRepo extracts "owner/repo" from a GitHub remote URL.
func parseOwnerRepo(remoteURL string) (string, error) {
	if m := reGitHubHTTPS.FindStringSubmatch(remoteURL); m != nil {
		return m[1], nil
	}
	if m := reGitHubSSH.FindStringSubmatch(remoteURL); m != nil {
		return m[1], nil
	}
	return "", fmt.Errorf("cannot parse GitHub owner/repo from remote URL: %q", remoteURL)
}

// postIssueToRepo creates a GitHub issue on ownerRepo and returns its URL.
func postIssueToRepo(ownerRepo, token, title, body string, labels []string) (string, error) {
	payload := issueCreateRequest{
		Title:  title,
		Body:   body,
		Labels: labels,
	}
	reqBody, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("issue file: marshal request: %w", err)
	}

	url := IssueBaseURL + "/repos/" + ownerRepo + "/issues"
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(reqBody)) //nolint:noctx // CLI tool
	if err != nil {
		return "", fmt.Errorf("issue file: build request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("issue file: HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("issue file: read response: %w", err)
	}

	if resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("issue file: GitHub API returned HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	var created issueCreateResponse
	if jsonErr := json.Unmarshal(respBody, &created); jsonErr != nil {
		return "", fmt.Errorf("issue file: parse response: %w", jsonErr)
	}
	return created.HTMLURL, nil
}
