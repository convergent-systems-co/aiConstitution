package cmd_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/convergent-systems-co/aiConstitution/src/cmd/ai/cmd"
)

// startGitHubIssueServer starts a mock GitHub Issues API server.
// It captures requests into reqs and returns a fake created issue.
func startGitHubIssueServer(t *testing.T, reqs *[]*githubIssueReq) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if reqs != nil {
			var req githubIssueReq
			_ = json.NewDecoder(r.Body).Decode(&req)
			req.Path = r.URL.Path
			req.Method = r.Method
			*reqs = append(*reqs, &req)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		resp := map[string]interface{}{
			"html_url": "https://github.com/owner/repo/issues/42",
			"number":   42,
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	t.Cleanup(srv.Close)
	return srv
}

type githubIssueReq struct {
	Title  string   `json:"title"`
	Body   string   `json:"body"`
	Labels []string `json:"labels"`
	Path   string
	Method string
}

// setIssueBaseURL overrides cmd.IssueBaseURL for the duration of the test.
func setIssueBaseURL(t *testing.T, baseURL string) {
	t.Helper()
	old := cmd.IssueBaseURL
	cmd.IssueBaseURL = baseURL
	t.Cleanup(func() { cmd.IssueBaseURL = old })
}

// initGitRepoWithRemote initialises a minimal git repo in dir and adds
// an "origin" remote pointing to https://github.com/owner/repo.git.
// It returns the repo directory for use as ISSUE_GIT_WORK_DIR.
func initGitRepoWithRemote(t *testing.T) string {
	t.Helper()
	repoDir := t.TempDir()

	// git init
	run := func(args ...string) {
		t.Helper()
		c := exec.Command("git", args...) //nolint:gosec
		c.Dir = repoDir
		// Suppress user identity requirements.
		c.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Test",
			"GIT_AUTHOR_EMAIL=test@test.com",
			"GIT_COMMITTER_NAME=Test",
			"GIT_COMMITTER_EMAIL=test@test.com",
		)
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}

	run("init", "-b", "main")
	run("remote", "add", "origin", "https://github.com/owner/repo.git")
	return repoDir
}

// runIssueCmd runs `ai issue file` with given flags in a temp repo dir.
func runIssueCmd(t *testing.T, token string, args ...string) (stdout, stderr string, err error) {
	t.Helper()

	repoDir := initGitRepoWithRemote(t)

	t.Setenv("ISSUE_GIT_WORK_DIR", repoDir)
	if token != "" {
		t.Setenv("GH_TOKEN", token)
	}

	var outBuf, errBuf bytes.Buffer
	c := cmd.NewRootCmd()
	c.SetOut(&outBuf)
	c.SetErr(&errBuf)
	c.SetArgs(append([]string{"issue", "file"}, args...))
	err = c.Execute()
	return outBuf.String(), errBuf.String(), err
}

// ---------------------------------------------------------------------------
// Tests for ai issue file (#385)
// ---------------------------------------------------------------------------

func TestIssueFile_HappyPath_CreatesIssueWithKindLabel(t *testing.T) {
	var reqs []*githubIssueReq
	srv := startGitHubIssueServer(t, &reqs)
	setIssueBaseURL(t, srv.URL)

	out, _, err := runIssueCmd(t, "test-token",
		"--title", "Test issue",
		"--body", "Test body",
		"--type", "finding",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(reqs) != 1 {
		t.Fatalf("expected 1 HTTP request, got %d", len(reqs))
	}
	req := reqs[0]
	if req.Title != "Test issue" {
		t.Errorf("title = %q; want %q", req.Title, "Test issue")
	}
	if req.Body != "Test body" {
		t.Errorf("body = %q; want %q", req.Body, "Test body")
	}

	hasLabel := false
	for _, l := range req.Labels {
		if l == "kind/finding" {
			hasLabel = true
		}
	}
	if !hasLabel {
		t.Errorf("labels should contain 'kind/finding'; got: %v", req.Labels)
	}

	if !strings.Contains(out, "https://github.com/owner/repo/issues/42") {
		t.Errorf("output should contain issue URL; got:\n%s", out)
	}
}

func TestIssueFile_MajorFlag_AddsSeverityLabel(t *testing.T) {
	var reqs []*githubIssueReq
	srv := startGitHubIssueServer(t, &reqs)
	setIssueBaseURL(t, srv.URL)

	_, _, err := runIssueCmd(t, "test-token",
		"--title", "Major bug",
		"--body", "Something broke badly",
		"--type", "bug",
		"--major",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(reqs) != 1 {
		t.Fatalf("expected 1 request, got %d", len(reqs))
	}
	labels := reqs[0].Labels
	hasMajor := false
	hasKind := false
	for _, l := range labels {
		if l == "severity/major" {
			hasMajor = true
		}
		if l == "kind/bug" {
			hasKind = true
		}
	}
	if !hasMajor {
		t.Errorf("labels missing 'severity/major'; got: %v", labels)
	}
	if !hasKind {
		t.Errorf("labels missing 'kind/bug'; got: %v", labels)
	}
}

func TestIssueFile_DefaultType_IsTask(t *testing.T) {
	var reqs []*githubIssueReq
	srv := startGitHubIssueServer(t, &reqs)
	setIssueBaseURL(t, srv.URL)

	_, _, err := runIssueCmd(t, "test-token",
		"--title", "Some task",
		"--body", "Do a thing",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(reqs) != 1 {
		t.Fatalf("expected 1 request, got %d", len(reqs))
	}
	hasTask := false
	for _, l := range reqs[0].Labels {
		if l == "kind/task" {
			hasTask = true
		}
	}
	if !hasTask {
		t.Errorf("default type should produce 'kind/task' label; got: %v", reqs[0].Labels)
	}
}

func TestIssueFile_FromAudit_ReadsFileAsBody(t *testing.T) {
	var reqs []*githubIssueReq
	srv := startGitHubIssueServer(t, &reqs)
	setIssueBaseURL(t, srv.URL)

	// Create a fake audit file.
	auditFile := filepath.Join(t.TempDir(), "violation.md")
	auditContent := "# Violation — 2026-01-01\n- What happened: something bad\n"
	if err := os.WriteFile(auditFile, []byte(auditContent), 0o644); err != nil {
		t.Fatal(err)
	}

	_, _, err := runIssueCmd(t, "test-token",
		"--title", "Audit finding",
		"--from-audit", auditFile,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(reqs) != 1 {
		t.Fatalf("expected 1 request, got %d", len(reqs))
	}
	body := reqs[0].Body
	if !strings.Contains(body, "From audit log:") {
		t.Errorf("body should start with 'From audit log:'; got:\n%s", body)
	}
	if !strings.Contains(body, auditContent) {
		t.Errorf("body should contain audit file content; got:\n%s", body)
	}
}

func TestIssueFile_MissingTitle_ReturnsError(t *testing.T) {
	_, _, err := runIssueCmd(t, "test-token",
		"--body", "No title given",
	)
	if err == nil {
		t.Fatal("expected error when --title is missing, got nil")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "title") {
		t.Errorf("error should mention 'title'; got: %v", err)
	}
}

func TestIssueFile_NoToken_ReturnsError(t *testing.T) {
	t.Setenv("GH_TOKEN", "")
	t.Setenv("GITHUB_TOKEN", "")

	_, _, err := runIssueCmd(t, "",
		"--title", "No token test",
		"--body", "body",
	)
	if err == nil {
		t.Fatal("expected error when no token, got nil")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "token") {
		t.Errorf("error should mention 'token'; got: %v", err)
	}
}
