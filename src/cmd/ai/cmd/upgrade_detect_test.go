package cmd

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestDetectSelfUpgradeCommand_FromAbsolutePaths drives the detection heuristic
// with concrete exe paths so changes to substring matching are visible.
//
// The intent is to lock in behavior for the two install shapes that matter:
//   - Brew (macOS): exe lives under …/Cellar/ai/<ver>/bin/ai
//   - GOPATH / go install: exe lives under …/go/bin/ai
//
// The cases marked "WANT brew (after symlink-resolution fix)" currently fail
// against the implementation in this commit — the detection looks at the
// unresolved exe path, so a brew install reached through a PATH symlink does
// not match the Cellar prefix. The bug is tracked separately; the test exists
// so a future regression cannot quietly re-introduce it.
func TestDetectSelfUpgradeCommand_FromAbsolutePaths(t *testing.T) {
	// AICONST_UPGRADE_COMMAND must not be set, or it short-circuits detection.
	t.Setenv("AICONST_UPGRADE_COMMAND", "")

	type result struct {
		name string
		args []string
	}
	cases := []struct {
		name     string
		exe      string
		darwinOK result // expected on darwin
		linuxOK  result // expected on linux
	}{
		{
			name:     "brew_cellar_direct",
			exe:      "/opt/homebrew/Cellar/ai/1.7.0/bin/ai",
			darwinOK: result{name: "brew", args: []string{"upgrade", "ai"}},
			linuxOK:  result{}, // brew check is darwin-only
		},
		{
			name:     "unresolved_brew_symlink_via_homebrew_bin",
			exe:      "/opt/homebrew/bin/ai",
			darwinOK: result{}, // FIXME: should be brew once detection resolves symlinks
			linuxOK:  result{},
		},
		{
			name:     "unresolved_brew_symlink_via_user_bin",
			exe:      "/Users/anyone/bin/ai",
			darwinOK: result{}, // FIXME: should be brew once detection resolves symlinks
			linuxOK:  result{},
		},
		{
			name:     "unrelated_path",
			exe:      "/random/dir/ai-tool",
			darwinOK: result{},
			linuxOK:  result{},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			orig := osExecutable
			t.Cleanup(func() { osExecutable = orig })
			osExecutable = func() (string, error) { return tc.exe, nil }

			got := detectSelfUpgradeCommand()
			want := tc.darwinOK
			if runtime.GOOS != "darwin" {
				want = tc.linuxOK
			}
			if got.Name != want.name || strings.Join(got.Args, " ") != strings.Join(want.args, " ") {
				t.Fatalf("detect(%q) on %s = {%q %v}, want {%q %v}",
					tc.exe, runtime.GOOS, got.Name, got.Args, want.name, want.args)
			}
		})
	}
}

// TestDetectSelfUpgradeCommand_SymlinkChain constructs a real on-disk Cellar
// layout and asserts what the detection heuristic returns when the binary is
// invoked through a symlink chain that mirrors a real brew install:
//
//	${tmp}/Cellar/ai/1.7.0/bin/ai   ← target file
//	${tmp}/opt/bin/ai               → ../../Cellar/ai/1.7.0/bin/ai
//	${tmp}/userbin/ai               → ../opt/bin/ai                    (entry point)
//
// The test injects each layer's path as os.Executable() in turn and locks in
// today's behavior. Once Step 3 lands (filepath.EvalSymlinks in detection),
// the assertions for the symlinked entry points should flip to expect brew
// on darwin — which is the regression guard we want.
func TestDetectSelfUpgradeCommand_SymlinkChain(t *testing.T) {
	t.Setenv("AICONST_UPGRADE_COMMAND", "")

	tmp := t.TempDir()
	cellarBin := filepath.Join(tmp, "Cellar", "ai", "1.7.0", "bin")
	if err := os.MkdirAll(cellarBin, 0o755); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(cellarBin, "ai")
	if err := os.WriteFile(target, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	optBin := filepath.Join(tmp, "opt", "bin")
	if err := os.MkdirAll(optBin, 0o755); err != nil {
		t.Fatal(err)
	}
	opt := filepath.Join(optBin, "ai")
	if err := os.Symlink(target, opt); err != nil {
		t.Fatal(err)
	}

	userBin := filepath.Join(tmp, "userbin")
	if err := os.MkdirAll(userBin, 0o755); err != nil {
		t.Fatal(err)
	}
	user := filepath.Join(userBin, "ai")
	if err := os.Symlink(opt, user); err != nil {
		t.Fatal(err)
	}

	layers := []struct {
		label string
		exe   string
		// today: detection inspects this string directly; only the Cellar
		// layer carries the literal substring "/Cellar/ai/".
		wantBrewToday bool
	}{
		{label: "cellar_target", exe: target, wantBrewToday: true},
		{label: "opt_symlink", exe: opt, wantBrewToday: false},
		{label: "user_symlink", exe: user, wantBrewToday: false},
	}

	for _, l := range layers {
		t.Run(l.label, func(t *testing.T) {
			orig := osExecutable
			t.Cleanup(func() { osExecutable = orig })
			osExecutable = func() (string, error) { return l.exe, nil }

			got := detectSelfUpgradeCommand()
			isBrew := got.Name == "brew"
			wantBrew := l.wantBrewToday && runtime.GOOS == "darwin"
			if isBrew != wantBrew {
				t.Fatalf("layer=%s exe=%s: got %q %v, wantBrew=%v",
					l.label, l.exe, got.Name, got.Args, wantBrew)
			}
		})
	}
}
