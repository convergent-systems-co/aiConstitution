package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/convergent-systems-co/aiConstitution/src/internal/config"
)


// ShareBaseURL is the base GitHub API URL used by runShareUpstream.
// Tests may override this to point at an httptest server.
var ShareBaseURL = "https://api.github.com"

const shareMaxBodyBytes = 65000

// getShareToken resolves the GitHub token from the environment and settings.
// Lookup order: GH_TOKEN → GITHUB_TOKEN → settings.upstream (future).
// Returns an error with instructions when no token is available.
func getShareToken() (string, error) {
	if t := os.Getenv("GH_TOKEN"); t != "" {
		return t, nil
	}
	if t := os.Getenv("GITHUB_TOKEN"); t != "" {
		return t, nil
	}
	return "", fmt.Errorf("no GitHub token found: set GH_TOKEN or GITHUB_TOKEN in your environment")
}

// issueCreateRequest is the JSON body for POST /repos/<owner>/<repo>/issues.
type issueCreateRequest struct {
	Title  string   `json:"title"`
	Body   string   `json:"body"`
	Labels []string `json:"labels,omitempty"`
}

// issueCreateResponse captures the fields we care about from GitHub's response.
type issueCreateResponse struct {
	HTMLURL string `json:"html_url"`
	Number  int    `json:"number"`
}

// postGitHubIssue creates a GitHub issue via the REST API and returns its URL.
func postGitHubIssue(upstreamRepo, token string, payload issueCreateRequest) (string, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("share: marshal request: %w", err)
	}

	url := ShareBaseURL + "/repos/" + upstreamRepo + "/issues"
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body)) //nolint:noctx // CLI tool
	if err != nil {
		return "", fmt.Errorf("share: build request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("share: HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("share: read response: %w", err)
	}

	if resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("share: GitHub API returned HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	var created issueCreateResponse
	if jsonErr := json.Unmarshal(respBody, &created); jsonErr != nil {
		return "", fmt.Errorf("share: parse response: %w", jsonErr)
	}
	return created.HTMLURL, nil
}

// runShareUpstream is the shared implementation used by all `ai <cmd> share`
// subcommands. It reads a local file and files it as a GitHub issue on
// upstreamRepo. Output is written to out.
//
// name is the human-readable contribution name (used as issue title prefix).
// filePath is the absolute path to the file to share.
// upstreamRepo is the "owner/repo" string for the target repository.
// token is the GitHub bearer token (pass "" to resolve from environment).
// out is the writer for success/info messages (typically cmd.OutOrStdout()).
func runShareUpstream(name, filePath, upstreamRepo, token string, out io.Writer) error {
	// 1. Read the local file.
	content, err := os.ReadFile(filePath) //nolint:gosec // G304: filePath is constructed from known safe roots
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("share: file not found: %s", filePath)
		}
		return fmt.Errorf("share: read %s: %w", filePath, err)
	}

	// 2. Check settings gate.
	cfg, cfgErr := config.Load()
	if cfgErr == nil && !cfg.Upstream.ShareEnabled {
		fmt.Fprintln(out, "Sharing disabled in settings (upstream.shareEnabled=false)")
		return nil
	}

	// 3. Resolve token.
	if token == "" {
		token, err = getShareToken()
		if err != nil {
			return err
		}
	}

	// 4. Build issue body (truncate at 65000 chars).
	body := string(content)
	if len(body) > shareMaxBodyBytes {
		body = body[:shareMaxBodyBytes] + "\n\n[truncated]"
	}

	// 5. Post to GitHub.
	issueURL, err := postGitHubIssue(upstreamRepo, token, issueCreateRequest{
		Title:  "[Contribution] " + name,
		Body:   body,
		Labels: []string{"contribution"},
	})
	if err != nil {
		return err
	}

	fmt.Fprintln(out, "Created upstream issue:", issueURL)
	return nil
}
