package cmd_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/convergent-systems-co/aiConstitution/src/cmd/ai/cmd"
)

// startIssueServer starts an httptest.Server that captures incoming requests
// into reqs and returns a fake GitHub issue creation response.
// If status is non-zero it is used as the response status; otherwise 201.
func startIssueServer(t *testing.T, reqs *[]*issueRequest, status int) *httptest.Server {
	t.Helper()
	if status == 0 {
		status = http.StatusCreated
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if reqs != nil {
			var req issueRequest
			_ = json.NewDecoder(r.Body).Decode(&req)
			req.Path = r.URL.Path
			req.Method = r.Method
			req.AuthHeader = r.Header.Get("Authorization")
			*reqs = append(*reqs, &req)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		if status == http.StatusCreated {
			resp := map[string]interface{}{
				"html_url": "https://github.com/test/repo/issues/1",
				"number":   1,
			}
			_ = json.NewEncoder(w).Encode(resp)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

// issueRequest captures the decoded body plus request metadata.
type issueRequest struct {
	Title  string   `json:"title"`
	Body   string   `json:"body"`
	Labels []string `json:"labels"`
	Path   string
	Method string
	AuthHeader string
}

// setShareBaseURL overrides cmd.ShareBaseURL for the duration of the test.
func setShareBaseURL(t *testing.T, baseURL string) {
	t.Helper()
	old := cmd.ShareBaseURL
	cmd.ShareBaseURL = baseURL
	t.Cleanup(func() { cmd.ShareBaseURL = old })
}

// runShareCmd is a helper that calls ai <args> with GH_TOKEN and AI_ROOT set.
func runShareCmd(t *testing.T, aiRoot, token string, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	if aiRoot != "" {
		t.Setenv("AI_ROOT", aiRoot)
	}
	if token != "" {
		t.Setenv("GH_TOKEN", token)
	}
	var outBuf, errBuf bytes.Buffer
	c := cmd.NewRootCmd()
	c.SetOut(&outBuf)
	c.SetErr(&errBuf)
	c.SetArgs(args)
	err = c.Execute()
	return outBuf.String(), errBuf.String(), err
}

// ---------------------------------------------------------------------------
// runShareUpstream core tests
// ---------------------------------------------------------------------------

func TestShareUpstream_HooksShare_PostsCorrectTitleAndBody(t *testing.T) {
	var reqs []*issueRequest
	srv := startIssueServer(t, &reqs, 0)
	setShareBaseURL(t, srv.URL)

	// Create a temp AI_ROOT with a hook file.
	aiRoot := t.TempDir()
	hooksDir := filepath.Join(aiRoot, "hooks")
	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		t.Fatal(err)
	}
	hookContent := "#!/usr/bin/env python3\nprint('hello')\n"
	hookPath := filepath.Join(hooksDir, "my-hook.py")
	if err := os.WriteFile(hookPath, []byte(hookContent), 0o644); err != nil {
		t.Fatal(err)
	}

	out, _, err := runShareCmd(t, aiRoot, "test-token", "hooks", "share", "my-hook.py")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(reqs) != 1 {
		t.Fatalf("expected 1 HTTP request, got %d", len(reqs))
	}
	req := reqs[0]
	if req.Title != "[Contribution] my-hook.py" {
		t.Errorf("title = %q; want %q", req.Title, "[Contribution] my-hook.py")
	}
	if !strings.Contains(req.Body, hookContent) {
		t.Errorf("body should contain hook content; got:\n%s", req.Body)
	}
	if !strings.Contains(out, "https://github.com/test/repo/issues/1") {
		t.Errorf("output should contain issue URL; got:\n%s", out)
	}
}

func TestShareUpstream_ConfigGate_ShareEnabledFalse_NoHTTPRequest(t *testing.T) {
	var reqs []*issueRequest
	srv := startIssueServer(t, &reqs, 0)
	setShareBaseURL(t, srv.URL)

	aiRoot := t.TempDir()
	hooksDir := filepath.Join(aiRoot, "hooks")
	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hooksDir, "my-hook.py"), []byte("#!/usr/bin/env python3\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Write a settings.toml that disables sharing.
	configDir := filepath.Join(t.TempDir(), "config")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	settingsContent := "[upstream]\nshareEnabled = false\n"
	settingsPath := filepath.Join(configDir, "settings.toml")
	if err := os.WriteFile(settingsPath, []byte(settingsContent), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AICONST_CONFIG_DIR", configDir)

	out, _, err := runShareCmd(t, aiRoot, "test-token", "hooks", "share", "my-hook.py")
	if err != nil {
		t.Fatalf("should exit 0 when sharing is disabled; got: %v", err)
	}

	if len(reqs) != 0 {
		t.Errorf("expected 0 HTTP requests when disabled; got %d", len(reqs))
	}
	if !strings.Contains(out, "disabled") {
		t.Errorf("output should mention 'disabled'; got:\n%s", out)
	}
}

func TestShareUpstream_FileNotFound_ReturnsError(t *testing.T) {
	aiRoot := t.TempDir()
	// No hook file created.
	_, _, err := runShareCmd(t, aiRoot, "test-token", "hooks", "share", "nonexistent.py")
	if err == nil {
		t.Fatal("expected error for nonexistent file, got nil")
	}
}

func TestShareUpstream_NoToken_ReturnsError(t *testing.T) {
	aiRoot := t.TempDir()
	hooksDir := filepath.Join(aiRoot, "hooks")
	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hooksDir, "my-hook.py"), []byte("#!/usr/bin/env python3\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Ensure no token env vars are set.
	t.Setenv("GH_TOKEN", "")
	t.Setenv("GITHUB_TOKEN", "")

	_, _, err := runShareCmd(t, aiRoot, "", "hooks", "share", "my-hook.py")
	if err == nil {
		t.Fatal("expected error when no token available, got nil")
	}
	if !strings.Contains(err.Error(), "token") {
		t.Errorf("error should mention 'token'; got: %v", err)
	}
}

func TestShareUpstream_TruncatesLargeBody(t *testing.T) {
	var reqs []*issueRequest
	srv := startIssueServer(t, &reqs, 0)
	setShareBaseURL(t, srv.URL)

	aiRoot := t.TempDir()
	hooksDir := filepath.Join(aiRoot, "hooks")
	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Create a file larger than 65000 bytes.
	bigContent := strings.Repeat("a", 70000)
	if err := os.WriteFile(filepath.Join(hooksDir, "big-hook.py"), []byte(bigContent), 0o644); err != nil {
		t.Fatal(err)
	}

	_, _, err := runShareCmd(t, aiRoot, "test-token", "hooks", "share", "big-hook.py")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(reqs) != 1 {
		t.Fatalf("expected 1 HTTP request, got %d", len(reqs))
	}
	body := reqs[0].Body
	if len(body) > 65100 {
		t.Errorf("body too long (%d bytes); expected ≤65100", len(body))
	}
	if !strings.Contains(body, "[truncated]") {
		t.Errorf("body should contain '[truncated]'; got:\n%s", body[:200])
	}
}

func TestShareUpstream_SkillsShare_UsesSkillMD(t *testing.T) {
	var reqs []*issueRequest
	srv := startIssueServer(t, &reqs, 0)
	setShareBaseURL(t, srv.URL)

	aiRoot := t.TempDir()
	skillDir := filepath.Join(aiRoot, "skills", "my-skill")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	skillContent := "---\nname: my-skill\ndescription: A test skill.\n---\n"
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skillContent), 0o644); err != nil {
		t.Fatal(err)
	}

	_, _, err := runShareCmd(t, aiRoot, "test-token", "skills", "share", "my-skill")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(reqs) != 1 {
		t.Fatalf("expected 1 HTTP request, got %d", len(reqs))
	}
	if reqs[0].Title != "[Contribution] my-skill" {
		t.Errorf("title = %q; want %q", reqs[0].Title, "[Contribution] my-skill")
	}
	if !strings.Contains(reqs[0].Body, skillContent) {
		t.Errorf("body should contain SKILL.md content")
	}
}

func TestShareUpstream_PersonaShare_PostsCorrectly(t *testing.T) {
	var reqs []*issueRequest
	srv := startIssueServer(t, &reqs, 0)
	setShareBaseURL(t, srv.URL)

	aiRoot := t.TempDir()
	personasDir := filepath.Join(aiRoot, "personas")
	if err := os.MkdirAll(personasDir, 0o755); err != nil {
		t.Fatal(err)
	}
	personaContent := "kind: agentic\nname: security\n"
	if err := os.WriteFile(filepath.Join(personasDir, "security.yaml"), []byte(personaContent), 0o644); err != nil {
		t.Fatal(err)
	}

	_, _, err := runShareCmd(t, aiRoot, "test-token", "persona", "share", "security")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(reqs) != 1 {
		t.Fatalf("expected 1 HTTP request, got %d", len(reqs))
	}
	if !strings.Contains(reqs[0].Body, personaContent) {
		t.Errorf("body should contain persona content")
	}
}

func TestShareUpstream_ProfileShare_PostsCorrectly(t *testing.T) {
	var reqs []*issueRequest
	srv := startIssueServer(t, &reqs, 0)
	setShareBaseURL(t, srv.URL)

	// profiles are in ConfigDir, not AIRoot.
	configDir := t.TempDir()
	profilesDir := filepath.Join(configDir, "profiles")
	if err := os.MkdirAll(profilesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	profileContent := "name: my-profile\n"
	if err := os.WriteFile(filepath.Join(profilesDir, "my-profile.yaml"), []byte(profileContent), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AICONST_CONFIG_DIR", configDir)

	aiRoot := t.TempDir()
	_, _, err := runShareCmd(t, aiRoot, "test-token", "profile", "share", "my-profile")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(reqs) != 1 {
		t.Fatalf("expected 1 HTTP request, got %d", len(reqs))
	}
	if !strings.Contains(reqs[0].Body, profileContent) {
		t.Errorf("body should contain profile content")
	}
}
