//go:build utm

// utm_e2e_test.go — end-to-end tests against the local UTM virtual machines.
//
// Run with:
//
//	go test -v -tags utm -run TestUTM ./src/cmd/ai/cmd/... -timeout 5m
//
// Required env (or ~/workspace/convergent-system-co/aiConstitution/env file):
//
//	AI_UTM_USERNAME  — shared username for both VMs (default: file Username=)
//	AI_UTM_PASSWORD  — shared password           (default: file Password=)
//	AI_UTM_LINUX_HOST   — Linux VM address  (default: 192.168.64.4)
//	AI_UTM_WINDOWS_HOST — Windows VM address (default: 192.168.64.3)
package cmd_test

import (
	"bufio"
	"context"
	"encoding/base64"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// utmConfig holds connection details for the two UTM VMs.
type utmConfig struct {
	username    string
	linuxHost   string
	windowsHost string
	// password is kept in a closure so it is never stored in a readable field.
	winrmRun func(t *testing.T, ps string) string
	// winrmRunRaw returns output without stripping Python preamble noise.
	winrmRunRaw func(t *testing.T, ps string) string
}

// loadUTMConfig reads credentials and host addresses. It skips the test if
// the UTM boxes are unreachable or credentials are unavailable.
func loadUTMConfig(t *testing.T) utmConfig {
	t.Helper()

	cfg := utmConfig{
		linuxHost:   envOrDefault("AI_UTM_LINUX_HOST", "192.168.64.4"),
		windowsHost: envOrDefault("AI_UTM_WINDOWS_HOST", "192.168.64.3"),
	}

	username := os.Getenv("AI_UTM_USERNAME")
	password := os.Getenv("AI_UTM_PASSWORD")

	if username == "" || password == "" {
		envFile := filepath.Join(os.Getenv("HOME"),
			"workspace/convergent-system-co/aiConstitution/env")
		if f, err := os.Open(envFile); err == nil {
			defer f.Close()
			sc := bufio.NewScanner(f)
			for sc.Scan() {
				k, v, ok := strings.Cut(sc.Text(), "=")
				if !ok {
					continue
				}
				switch k {
				case "Username":
					if username == "" {
						username = v
					}
				case "Password":
					if password == "" {
						password = v
					}
				}
			}
		}
	}

	if username == "" || password == "" {
		t.Skip("UTM credentials unavailable; set AI_UTM_USERNAME/AI_UTM_PASSWORD or provide env file")
	}
	cfg.username = username

	// Verify reachability before spending time building.
	if !tcpReachable(cfg.linuxHost, "22", 5*time.Second) {
		t.Skipf("Linux VM %s:22 unreachable", cfg.linuxHost)
	}
	if !tcpReachable(cfg.windowsHost, "5985", 5*time.Second) {
		t.Skipf("Windows VM %s:5985 unreachable", cfg.windowsHost)
	}

	// Capture password in closure so it never leaks into struct fields or logs.
	host, user, pass := cfg.windowsHost, username, password
	cfg.winrmRun = func(t *testing.T, ps string) string {
		t.Helper()
		return winrmExec(t, host, user, pass, ps)
	}
	cfg.winrmRunRaw = cfg.winrmRun

	return cfg
}

// envOrDefault returns the env var value or the fallback if unset.
func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// tcpReachable returns true if host:port accepts a TCP connection within timeout.
func tcpReachable(host, port string, timeout time.Duration) bool {
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, port), timeout)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// sshRun runs a command on the Linux VM via SSH key auth and returns combined output.
// Fails the test on non-zero exit unless the caller checks the error separately.
func sshRun(t *testing.T, host, command string) string {
	t.Helper()
	out, err := exec.Command("ssh",
		"-o", "StrictHostKeyChecking=no",
		"-o", "ConnectTimeout=15",
		"-o", "BatchMode=yes",
		host,
		command,
	).CombinedOutput()
	if err != nil {
		t.Fatalf("ssh %s: %v\noutput:\n%s", host, err, out)
	}
	return strings.TrimSpace(string(out))
}

// winrmExec runs a PowerShell script on the Windows VM via WinRM.
// Password is passed via Python subprocess — never logged.
func winrmExec(t *testing.T, host, username, password, ps string) string {
	t.Helper()
	// Build the Python inline script. The password never appears in test output
	// because we pass it through a dedicated env var to the subprocess.
	// Redirect Python's own stderr to devnull so urllib3 deprecation warnings
	// don't contaminate the WinRM stdout that tests assert on.
	pyScript := `
import winrm, warnings, os, sys
warnings.filterwarnings("ignore")
import urllib3; urllib3.disable_warnings()
sys.stderr = open(os.devnull, "w")
s = winrm.Session(os.environ["_WINRM_HOST"],
                  auth=(os.environ["_WINRM_USER"], os.environ["_WINRM_PASS"]),
                  transport="ntlm", read_timeout_sec=120, operation_timeout_sec=90)
r = s.run_ps(os.environ["_WINRM_CMD"])
out = r.std_out.decode("utf-8", errors="replace")
err = r.std_err.decode("utf-8", errors="replace")
sys.stdout.write(out)
if r.status_code != 0:
    open(2, "w", closefd=False).write("WinRM exit " + str(r.status_code) + "\n" + err)
    sys.exit(r.status_code)
`
	cmd := exec.Command("python3", "-W", "ignore::Warning", "-c", pyScript)
	cmd.Env = append(os.Environ(),
		"_WINRM_HOST="+host,
		"_WINRM_USER="+username,
		"_WINRM_PASS="+password,
		"_WINRM_CMD="+ps,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		// Strip any credential echoes from error before logging.
		safe := strings.ReplaceAll(string(out), password, "[REDACTED:password]")
		t.Fatalf("winrm on %s: %v\noutput:\n%s", host, err, safe)
	}
	return strings.TrimSpace(string(out))
}

// buildBinary cross-compiles the ai binary for the target GOOS/GOARCH.
func buildBinary(t *testing.T, goos, goarch string) string {
	t.Helper()
	ext := ""
	if goos == "windows" {
		ext = ".exe"
	}
	dest := filepath.Join(t.TempDir(), "ai-"+goos+"-"+goarch+ext)
	repoRoot := repoRoot(t)
	cmd := exec.Command("go", "build", "-o", dest, "./src/cmd/ai")
	cmd.Dir = repoRoot
	cmd.Env = append(os.Environ(), "GOOS="+goos, "GOARCH="+goarch, "CGO_ENABLED=0")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build %s/%s: %v\n%s", goos, goarch, err, out)
	}
	return dest
}

// repoRoot returns the repository root by walking up from this file's package.
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.work")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find repo root (go.work)")
		}
		dir = parent
	}
}

// scpTo copies a local file to host:remotePath via scp.
func scpTo(t *testing.T, local, host, remote string) {
	t.Helper()
	out, err := exec.Command("scp",
		"-o", "StrictHostKeyChecking=no",
		"-o", "ConnectTimeout=15",
		local, host+":"+remote,
	).CombinedOutput()
	if err != nil {
		t.Fatalf("scp %s → %s:%s: %v\n%s", local, host, remote, err, out)
	}
}

// serveFile starts a temporary HTTP server that serves a single file at /file.
// Returns the server URL and a cleanup function.
func serveFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("serveFile read %s: %v", path, err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/file", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write(data)
	})
	srv := &http.Server{Handler: mux}
	ln, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatalf("serveFile listen: %v", err)
	}
	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(func() { _ = srv.Shutdown(context.Background()) })
	// Use the macOS host IP reachable from UTM VMs (192.168.64.1 is the vmnet host).
	port := ln.Addr().(*net.TCPAddr).Port
	return fmt.Sprintf("http://192.168.64.1:%d/file", port)
}

// uploadBase64 uploads a file to Windows via WinRM by encoding it as base64
// and writing it via PowerShell. Used when scp is unavailable.
func uploadBase64(t *testing.T, localPath, remoteDir, fileName string, winrm func(*testing.T, string) string) string {
	t.Helper()
	data, err := os.ReadFile(localPath)
	if err != nil {
		t.Fatalf("uploadBase64 read %s: %v", localPath, err)
	}
	encoded := base64.StdEncoding.EncodeToString(data)
	remotePath := remoteDir + `\` + fileName
	// Write in one PowerShell command (binary is ~8 MB; WinRM limit is ~500 KB per call).
	// Chunk at 400 KB of base64 (~300 KB binary).
	const chunkSize = 400 * 1024
	winrm(t, fmt.Sprintf(`New-Item -ItemType Directory -Force -Path '%s' | Out-Null`, remoteDir))
	for i := 0; i < len(encoded); i += chunkSize {
		end := i + chunkSize
		if end > len(encoded) {
			end = len(encoded)
		}
		chunk := encoded[i:end]
		if i == 0 {
			winrm(t, fmt.Sprintf(`[IO.File]::WriteAllText(%q, %q)`, remotePath+".b64", chunk))
		} else {
			winrm(t, fmt.Sprintf(`[IO.File]::AppendAllText(%q, %q)`, remotePath+".b64", chunk))
		}
	}
	winrm(t, fmt.Sprintf(
		`$b=[Convert]::FromBase64String([IO.File]::ReadAllText(%q)); [IO.File]::WriteAllBytes(%q, $b); Remove-Item %q`,
		remotePath+".b64", remotePath, remotePath+".b64",
	))
	return remotePath
}

// ─── Linux workflow ──────────────────────────────────────────────────────────

func TestUTM_Linux_FullWorkflow(t *testing.T) {
	cfg := loadUTMConfig(t)
	host := cfg.linuxHost

	// Log remote clock before any tests; skewed clocks cause TLS cert failures
	// when fetching from ai-atoms.com. The binary handles this gracefully with
	// a warning, so we continue rather than skip.
	remoteDate, _ := exec.Command("ssh",
		"-o", "StrictHostKeyChecking=no", "-o", "ConnectTimeout=5", host, "date").CombinedOutput()
	t.Logf("Linux VM clock: %s", strings.TrimSpace(string(remoteDate)))

	t.Log("Building linux/arm64 binary…")
	binLocal := buildBinary(t, "linux", "arm64")

	remoteBin := "/tmp/ai-e2e"
	remoteRoot := "/tmp/ai-e2e-root"

	t.Log("Copying binary to Linux VM…")
	scpTo(t, binLocal, host, remoteBin)
	sshRun(t, host, "chmod +x "+remoteBin)

	// Helper: run ai command with AI_ROOT isolated.
	ai := func(t *testing.T, args string) string {
		t.Helper()
		return sshRun(t, host,
			fmt.Sprintf("AI_ROOT=%s AICONST_SEEDS='Q01=UTM E2E Test' %s %s 2>&1",
				remoteRoot, remoteBin, args))
	}

	t.Cleanup(func() {
		_ = exec.Command("ssh",
			"-o", "StrictHostKeyChecking=no",
			host,
			"rm -rf "+remoteRoot+" "+remoteBin,
		).Run()
	})

	t.Run("setup", func(t *testing.T) {
		out := ai(t, "setup --non-interactive --no-hooks")
		if !strings.Contains(out, "Constitution") {
			t.Errorf("setup output missing 'Constitution':\n%s", out)
		}
		sshRun(t, host, "test -f "+remoteRoot+"/Constitution.md")
		t.Logf("setup output:\n%s", out)
	})

	t.Run("compress", func(t *testing.T) {
		out := ai(t, "compress")
		if !strings.Contains(out, "compact") && !strings.Contains(out, "Constitution.compact") {
			t.Errorf("compress output unexpected:\n%s", out)
		}
		sshRun(t, host, "test -f "+remoteRoot+"/Constitution.compact.md")
		t.Logf("compress output:\n%s", out)
	})

	t.Run("doctor", func(t *testing.T) {
		out := ai(t, "doctor")
		t.Logf("doctor output:\n%s", out)
		if strings.Contains(strings.ToLower(out), "fatal") {
			t.Errorf("doctor reported fatal issue:\n%s", out)
		}
	})

	t.Run("hooks_install_command_wrappers", func(t *testing.T) {
		// Use a dedicated root so this subtest is independent of the setup
		// subtest having already installed wrappers. Idempotency is fine; we
		// verify file presence, not the "N extracted" count.
		freshRoot := remoteRoot + "-wrappers-fresh"
		freshAI := func(args string) string {
			return sshRun(t, host,
				fmt.Sprintf("AI_ROOT=%s AICONST_SEEDS='Q01=UTM E2E Test' %s %s 2>&1",
					freshRoot, remoteBin, args))
		}
		t.Cleanup(func() {
			_ = exec.Command("ssh",
				"-o", "StrictHostKeyChecking=no",
				host, "rm -rf "+freshRoot).Run()
		})
		// Run setup first to create the AI_ROOT dir structure, then install wrappers.
		freshAI("setup --non-interactive --no-hooks")
		out := freshAI("hooks install command-wrappers")
		t.Logf("hooks install output:\n%s", out)
		// Verify POSIX wrappers (never .cmd or .ps1 on Linux).
		sshRun(t, host, "test -f "+freshRoot+"/bin/git")
		sshRun(t, host, "test -f "+freshRoot+"/bin/gh")
		sshRun(t, host, "test ! -f "+freshRoot+"/bin/git.cmd")
	})

	t.Run("version", func(t *testing.T) {
		out := ai(t, "version")
		if !strings.HasPrefix(out, "ai v") {
			t.Errorf("version output malformed: %q", out)
		}
		if strings.Contains(out, "Code.md") {
			t.Errorf("version output contains stale Code.md line: %q", out)
		}
		t.Logf("version: %s", out)
	})

	t.Run("compress_check_coverage", func(t *testing.T) {
		out := ai(t, "compress --check-coverage")
		t.Logf("compress --check-coverage output:\n%s", out)
		// Should not report missing rule IDs.
		if strings.Contains(strings.ToLower(out), "missing") {
			t.Errorf("coverage check found missing rules:\n%s", out)
		}
	})
}

// ─── Windows workflow ────────────────────────────────────────────────────────

func TestUTM_Windows_FullWorkflow(t *testing.T) {
	cfg := loadUTMConfig(t)
	winrm := cfg.winrmRun

	t.Log("Building windows/arm64 binary…")
	binLocal := buildBinary(t, "windows", "arm64")

	remoteDir := `C:\Users\` + cfg.username + `\AppData\Local\Temp\ai-e2e`
	remoteRoot := remoteDir + `\root`
	remoteBin := remoteDir + `\ai.exe`

	// Serve binary from macOS over HTTP; Windows pulls via Invoke-WebRequest.
	// 192.168.64.1 is the UTM vmnet host address reachable from all VMs.
	t.Log("Serving windows/arm64 binary via HTTP for Windows to download…")
	fileURL := serveFile(t, binLocal)

	winrm(t, fmt.Sprintf(`New-Item -ItemType Directory -Force -Path '%s' | Out-Null`, remoteDir))
	winrm(t, fmt.Sprintf(
		`Invoke-WebRequest -Uri %q -OutFile %q -UseBasicParsing`,
		fileURL, remoteBin,
	))

	// Helper: run ai command on Windows with isolated AI_ROOT.
	// Use PowerShell single-quoted strings for paths so Go's %q double-escaping
	// of backslashes doesn't produce literal '\\' in the env var value.
	ai := func(t *testing.T, args string) string {
		t.Helper()
		ps := fmt.Sprintf(
			`$env:AI_ROOT='%s'; $env:AICONST_SEEDS='Q01=UTM E2E Test'; & '%s' %s 2>&1`,
			remoteRoot, remoteBin, args,
		)
		return winrm(t, ps)
	}

	t.Cleanup(func() {
		winrm(t, fmt.Sprintf(`Remove-Item -Recurse -Force -ErrorAction SilentlyContinue %q`, remoteDir))
	})

	t.Run("setup", func(t *testing.T) {
		out := ai(t, "setup --non-interactive --no-hooks")
		if !strings.Contains(out, "Constitution") {
			t.Errorf("setup output missing 'Constitution':\n%s", out)
		}
		exists := winrm(t, fmt.Sprintf(`(Test-Path '%s').ToString()`, remoteRoot+`\Constitution.md`))
		if !strings.Contains(exists, "True") {
			t.Errorf("Constitution.md not created on Windows; Test-Path returned: %q", exists)
		}
		t.Logf("setup output:\n%s", out)
	})

	t.Run("compress", func(t *testing.T) {
		out := ai(t, "compress")
		t.Logf("compress output:\n%s", out)
		exists := winrm(t, fmt.Sprintf(`Test-Path '%s'`, remoteRoot+`\Constitution.compact.md`))
		if !strings.Contains(exists, "True") {
			t.Errorf("Constitution.compact.md not created; compact output:\n%s\nexists: %s", out, exists)
		}
	})

	t.Run("doctor", func(t *testing.T) {
		out := ai(t, "doctor")
		t.Logf("doctor output:\n%s", out)
		if strings.Contains(strings.ToLower(out), "fatal") {
			t.Errorf("doctor reported fatal issue:\n%s", out)
		}
	})

	t.Run("hooks_install_command_wrappers", func(t *testing.T) {
		out := ai(t, "hooks install command-wrappers")
		t.Logf("hooks install output:\n%s", out)
		// Windows must produce .cmd and .ps1 shims, not bare "git".
		for _, name := range []string{"git.cmd", "git.ps1", "gh.cmd", "gh.ps1"} {
			exists := winrm(t, fmt.Sprintf(`Test-Path '%s'`, remoteRoot+`\bin\`+name))
			if !strings.Contains(exists, "True") {
				t.Errorf("expected Windows shim %s not found", name)
			}
		}
		noBare := winrm(t, fmt.Sprintf(`Test-Path '%s'`, remoteRoot+`\bin\git`))
		if strings.Contains(noBare, "True") {
			t.Error("bare 'git' shim should not exist on Windows")
		}
	})

	t.Run("doctor_fix_python_stubs", func(t *testing.T) {
		// doctor --fix on Windows should detect and optionally remove Python Store stubs.
		// Even if no stubs are present, the check must complete without error.
		out := ai(t, "doctor --fix")
		t.Logf("doctor --fix output:\n%s", out)
		if strings.Contains(strings.ToLower(out), "panic") {
			t.Errorf("doctor --fix panicked:\n%s", out)
		}
	})

	t.Run("version", func(t *testing.T) {
		out := ai(t, "version")
		if !strings.HasPrefix(out, "ai v") {
			t.Errorf("version output malformed: %q", out)
		}
		if strings.Contains(out, "Code.md") {
			t.Errorf("version output contains stale Code.md line: %q", out)
		}
		t.Logf("version: %s", out)
	})
}
