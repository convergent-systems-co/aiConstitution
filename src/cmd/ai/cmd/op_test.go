package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// fakeOpScript writes a fake `op` shell script to dir and returns the
// directory so the caller can prepend it to PATH.
// content is the shell body executed after #!/usr/bin/env bash.
func fakeOpScript(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	script := filepath.Join(dir, "op")
	body := "#!/usr/bin/env bash\n" + content + "\n"
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil { //nolint:gosec
		t.Fatalf("fakeOpScript: %v", err)
	}
	return dir
}

// prependPATH adds dir to the front of PATH for the duration of the test.
func prependPATH(t *testing.T, dir string) {
	t.Helper()
	orig := os.Getenv("PATH")
	t.Cleanup(func() { os.Setenv("PATH", orig) }) //nolint:errcheck
	os.Setenv("PATH", dir+string(os.PathListSeparator)+orig) //nolint:errcheck
}

// fakeClipboard installs a fake clipboard command that writes stdin to a file.
// On macOS it creates "pbcopy"; on Linux it creates "wl-copy" (matching the
// wl-copy preference in clipboardCmd). Returns the dir and the output file.
func fakeClipboard(t *testing.T) (dir string, outputFile string) {
	t.Helper()
	dir = t.TempDir()
	outputFile = filepath.Join(dir, "clipboard.txt")
	name := "pbcopy"
	if runtime.GOOS == "linux" {
		name = "wl-copy"
	}
	script := filepath.Join(dir, name)
	body := "#!/usr/bin/env bash\ncat > " + outputFile + "\n"
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil { //nolint:gosec
		t.Fatalf("fakeClipboard: %v", err)
	}
	return dir, outputFile
}

// --- op env tests ---

// opItemListJSON is a minimal valid response from `op item list --format json`.
const opItemListJSON = `[
  {"id":"aaa111","title":"MyDB","vault":{"id":"v1","name":"Private"}},
  {"id":"bbb222","title":"API Key","vault":{"id":"v2","name":"Work"}}
]`

// TestOpEnv_DotenvFormat verifies that `ai op env` prints
// KEY=op://vault/item/field lines in dotenv format.
func TestOpEnv_DotenvFormat(t *testing.T) {
	fakeDir := fakeOpScript(t, `printf '%s\n' '`+opItemListJSON+`'`)
	prependPATH(t, fakeDir)

	buf := &bytes.Buffer{}
	root := NewRootCmd()
	root.SetOut(buf)
	root.SetErr(&bytes.Buffer{})
	root.SetArgs([]string{"op", "env"})

	if err := root.Execute(); err != nil {
		t.Fatalf("op env returned error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "MYDB=op://Private/aaa111/") {
		t.Errorf("expected MYDB=op://... line, got:\n%s", out)
	}
	if !strings.Contains(out, "API_KEY=op://Work/bbb222/") {
		t.Errorf("expected API_KEY=op://... line, got:\n%s", out)
	}
}

// TestOpEnv_ExportFormat verifies that `ai op env --format export` prefixes
// each line with `export `.
func TestOpEnv_ExportFormat(t *testing.T) {
	fakeDir := fakeOpScript(t, `printf '%s\n' '`+opItemListJSON+`'`)
	prependPATH(t, fakeDir)

	buf := &bytes.Buffer{}
	root := NewRootCmd()
	root.SetOut(buf)
	root.SetErr(&bytes.Buffer{})
	root.SetArgs([]string{"op", "env", "--format", "export"})

	if err := root.Execute(); err != nil {
		t.Fatalf("op env --format export returned error: %v", err)
	}

	out := buf.String()
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if line == "" {
			continue
		}
		if !strings.HasPrefix(line, "export ") {
			t.Errorf("expected 'export ' prefix, got: %q", line)
		}
	}
}

// TestOpEnv_VaultFilter verifies that `ai op env --vault Work` only shows
// items from the Work vault.
func TestOpEnv_VaultFilter(t *testing.T) {
	// The fake op script supports --vault by filtering. We model that the
	// real op CLI does filtering; our fake emits the full list so the Go
	// code must filter client-side OR pass --vault to the subprocess.
	// Either way, the test asserts the output is filtered.
	fakeDir := fakeOpScript(t, `printf '%s\n' '`+opItemListJSON+`'`)
	prependPATH(t, fakeDir)

	buf := &bytes.Buffer{}
	root := NewRootCmd()
	root.SetOut(buf)
	root.SetErr(&bytes.Buffer{})
	root.SetArgs([]string{"op", "env", "--vault", "Work"})

	if err := root.Execute(); err != nil {
		t.Fatalf("op env --vault returned error: %v", err)
	}

	out := buf.String()
	if strings.Contains(out, "MYDB=") {
		t.Errorf("vault filter: expected MYDB (Private vault) to be absent, got:\n%s", out)
	}
	if !strings.Contains(out, "API_KEY=op://Work/") {
		t.Errorf("vault filter: expected API_KEY (Work vault), got:\n%s", out)
	}
}

// TestOpEnv_OpNotOnPATH verifies that `ai op env` returns a descriptive error
// when the `op` binary is not on PATH.
func TestOpEnv_OpNotOnPATH(t *testing.T) {
	// Remove all real op from PATH by pointing to an empty temp dir.
	prependPATH(t, t.TempDir())
	// Additionally remove any system op that might be on the PATH by
	// replacing PATH entirely with our empty dir only.
	orig := os.Getenv("PATH")
	t.Cleanup(func() { os.Setenv("PATH", orig) }) //nolint:errcheck
	os.Setenv("PATH", t.TempDir())                 //nolint:errcheck

	buf := &bytes.Buffer{}
	errBuf := &bytes.Buffer{}
	root := NewRootCmd()
	root.SetOut(buf)
	root.SetErr(errBuf)
	root.SetArgs([]string{"op", "env"})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error when op not on PATH, got nil")
	}
	if !strings.Contains(err.Error(), "op CLI not found") {
		t.Errorf("expected 'op CLI not found' in error, got: %v", err)
	}
}

// --- op clip tests ---

// TestOpClip_CopiesSecretToClipboard verifies that `ai op clip op://vault/item/field`
// pipes the secret to the OS clipboard command (mocked) and prints "Secret copied to clipboard."
func TestOpClip_CopiesSecretToClipboard(t *testing.T) {
	// Fake op that returns a secret value when `op read` is called.
	fakeOpDir := fakeOpScript(t, `
case "$1" in
  read) printf 'supersecret'; exit 0;;
  *) exit 1;;
esac
`)
	clipDir, clipboardFile := fakeClipboard(t)
	// Combine dirs into PATH: fake op first, then fake clipboard command.
	orig := os.Getenv("PATH")
	t.Cleanup(func() { os.Setenv("PATH", orig) }) //nolint:errcheck
	os.Setenv("PATH", fakeOpDir+string(os.PathListSeparator)+clipDir+string(os.PathListSeparator)+orig) //nolint:errcheck

	buf := &bytes.Buffer{}
	errBuf := &bytes.Buffer{}
	root := NewRootCmd()
	root.SetOut(buf)
	root.SetErr(errBuf)
	root.SetArgs([]string{"op", "clip", "op://Private/MyItem/password"})

	if err := root.Execute(); err != nil {
		t.Fatalf("op clip returned error: %v", err)
	}

	// Verify secret was written to clipboard, not stdout.
	if strings.Contains(buf.String(), "supersecret") {
		t.Error("secret must NOT appear on stdout")
	}

	clipData, err := os.ReadFile(clipboardFile)
	if err != nil {
		t.Fatalf("could not read clipboard file: %v", err)
	}
	if string(clipData) != "supersecret" {
		t.Errorf("expected clipboard to contain 'supersecret', got %q", string(clipData))
	}

	if !strings.Contains(buf.String(), "Secret copied to clipboard.") {
		t.Errorf("expected confirmation message, got: %q", buf.String())
	}
}

// TestOpClip_OpNotOnPATH verifies a clear error when op is missing.
func TestOpClip_OpNotOnPATH(t *testing.T) {
	orig := os.Getenv("PATH")
	t.Cleanup(func() { os.Setenv("PATH", orig) }) //nolint:errcheck
	os.Setenv("PATH", t.TempDir())                 //nolint:errcheck

	root := NewRootCmd()
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	root.SetArgs([]string{"op", "clip", "op://v/i/f"})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error when op not on PATH")
	}
	if !strings.Contains(err.Error(), "op CLI not found") {
		t.Errorf("expected 'op CLI not found', got: %v", err)
	}
}

// TestOpClip_NoClipboardCommand verifies a clear error when no clipboard
// command is available.
func TestOpClip_NoClipboardCommand(t *testing.T) {
	fakeOpDir := fakeOpScript(t, `
case "$1" in
  read) printf 'supersecret'; exit 0;;
  *) exit 1;;
esac
`)
	// PATH has op but no pbcopy/xclip/wl-copy.
	orig := os.Getenv("PATH")
	t.Cleanup(func() { os.Setenv("PATH", orig) }) //nolint:errcheck
	os.Setenv("PATH", fakeOpDir)                   //nolint:errcheck

	root := NewRootCmd()
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	root.SetArgs([]string{"op", "clip", "op://v/i/f"})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error when no clipboard command available")
	}
	if !strings.Contains(err.Error(), "clipboard") {
		t.Errorf("expected 'clipboard' in error, got: %v", err)
	}
}

// --- op signin / signout / whoami tests ---

// TestOpSignin_NoAddress verifies that `ai op signin` (no --address)
// prints the eval instruction and exits 0.
func TestOpSignin_NoAddress(t *testing.T) {
	buf := &bytes.Buffer{}
	root := NewRootCmd()
	root.SetOut(buf)
	root.SetErr(&bytes.Buffer{})
	root.SetArgs([]string{"op", "signin"})

	if err := root.Execute(); err != nil {
		t.Fatalf("op signin returned error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "eval") || !strings.Contains(out, "op signin") {
		t.Errorf("expected eval $(op signin) instruction, got: %q", out)
	}
}

// TestOpSignin_WithAddress verifies that `ai op signin --address <addr>` runs
// `op account add --address <addr>`.
func TestOpSignin_WithAddress(t *testing.T) {
	var gotArgs []string
	fakeDir := t.TempDir()
	script := filepath.Join(fakeDir, "op")
	// The fake script writes its args to a temp file so we can assert on them.
	argsFile := filepath.Join(fakeDir, "called-with.txt")
	body := "#!/usr/bin/env bash\nprintf '%s\n' \"$@\" > " + argsFile + "\n"
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil { //nolint:gosec
		t.Fatalf("fakeOpScript: %v", err)
	}
	prependPATH(t, fakeDir)

	root := NewRootCmd()
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	root.SetArgs([]string{"op", "signin", "--address", "my.1password.com"})

	if err := root.Execute(); err != nil {
		t.Fatalf("op signin --address returned error: %v", err)
	}

	data, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatalf("could not read args file: %v", err)
	}
	gotArgs = strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(gotArgs) < 3 {
		t.Fatalf("expected at least 3 args (account add --address), got: %v", gotArgs)
	}
	if gotArgs[0] != "account" || gotArgs[1] != "add" || gotArgs[2] != "--address" {
		t.Errorf("expected [account add --address ...], got: %v", gotArgs)
	}
}

// TestOpSignout verifies that `ai op signout` runs `op signout --forget`.
func TestOpSignout(t *testing.T) {
	fakeDir := t.TempDir()
	argsFile := filepath.Join(fakeDir, "called-with.txt")
	script := filepath.Join(fakeDir, "op")
	body := "#!/usr/bin/env bash\nprintf '%s\n' \"$@\" > " + argsFile + "\n"
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil { //nolint:gosec
		t.Fatalf("fakeOpScript: %v", err)
	}
	prependPATH(t, fakeDir)

	root := NewRootCmd()
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	root.SetArgs([]string{"op", "signout"})

	if err := root.Execute(); err != nil {
		t.Fatalf("op signout returned error: %v", err)
	}

	data, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatalf("could not read args file: %v", err)
	}
	got := strings.TrimSpace(string(data))
	if !strings.Contains(got, "signout") || !strings.Contains(got, "--forget") {
		t.Errorf("expected op signout --forget, got: %q", got)
	}
}

// TestOpWhoami verifies that `ai op whoami` runs `op whoami --format json`
// and streams the output.
func TestOpWhoami(t *testing.T) {
	const fakeJSON = `{"email":"user@example.com","url":"my.1password.com"}`
	fakeDir := fakeOpScript(t, `printf '%s\n' '`+fakeJSON+`'`)
	prependPATH(t, fakeDir)

	buf := &bytes.Buffer{}
	root := NewRootCmd()
	root.SetOut(buf)
	root.SetErr(&bytes.Buffer{})
	root.SetArgs([]string{"op", "whoami"})

	if err := root.Execute(); err != nil {
		t.Fatalf("op whoami returned error: %v", err)
	}

	if !strings.Contains(buf.String(), "user@example.com") {
		t.Errorf("expected whoami output on stdout, got: %q", buf.String())
	}
}

// TestOpSignout_OpNotOnPATH verifies a clear error when op is missing.
func TestOpSignout_OpNotOnPATH(t *testing.T) {
	orig := os.Getenv("PATH")
	t.Cleanup(func() { os.Setenv("PATH", orig) }) //nolint:errcheck
	os.Setenv("PATH", t.TempDir())                 //nolint:errcheck

	root := NewRootCmd()
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	root.SetArgs([]string{"op", "signout"})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error when op not on PATH")
	}
	if !strings.Contains(err.Error(), "op CLI not found") {
		t.Errorf("expected 'op CLI not found', got: %v", err)
	}
}

// TestOpWhoami_OpNotOnPATH verifies a clear error when op is missing.
func TestOpWhoami_OpNotOnPATH(t *testing.T) {
	orig := os.Getenv("PATH")
	t.Cleanup(func() { os.Setenv("PATH", orig) }) //nolint:errcheck
	os.Setenv("PATH", t.TempDir())                 //nolint:errcheck

	root := NewRootCmd()
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	root.SetArgs([]string{"op", "whoami"})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error when op not on PATH")
	}
	if !strings.Contains(err.Error(), "op CLI not found") {
		t.Errorf("expected 'op CLI not found', got: %v", err)
	}
}
