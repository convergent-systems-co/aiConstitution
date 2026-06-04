package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestMain isolates the test process from the developer's real environment in
// two ways, then runs the suite:
//
//  1. PATH: removes the real ~/.ai/bin governance shim. Several tests shell out
//     to git via exec.Command("git", ...); if ~/.ai/bin is on PATH those calls
//     resolve to the `ai wrap git` shim and run the full hook pipeline instead
//     of the real binary, which can block the suite.
//
//  2. HOME: redirects HOME (and USERPROFILE on Windows) to a throwaway dir, so
//     any test that resolves ~/.claude or ~/.ai via os.UserHomeDir without its
//     own override cannot read — or mutate — the developer's real config. This
//     is a safety net for tests that forget per-test isolation; e.g. a
//     `constitution restore` test once rewired the real ~/.claude/settings.json.
//
// Both are cross-platform: PATH splits on os.PathListSeparator (":" POSIX,
// ";" Windows); home is overridden via the variable each platform's
// os.UserHomeDir consults.
func TestMain(m *testing.M) {
	// Read the real home BEFORE overriding it, so we strip the real shim dir.
	if home, err := os.UserHomeDir(); err == nil {
		shim := filepath.Join(home, ".ai", "bin")
		_ = os.Setenv("PATH", removePathEntry(os.Getenv("PATH"), shim))
	}

	tmpHome, err := os.MkdirTemp("", "aiconst-testhome-")
	if err != nil {
		os.Exit(m.Run())
	}
	_ = os.Setenv("HOME", tmpHome)        // POSIX home
	_ = os.Setenv("USERPROFILE", tmpHome) // Windows home

	code := m.Run()
	_ = os.RemoveAll(tmpHome)
	os.Exit(code)
}

// removePathEntry returns a PATH-style string (os.PathListSeparator-joined)
// with every element equal to target removed. Comparison is on filepath.Clean,
// so trailing-slash and "." noise still match. Cross-platform: the separator is
// ":" on POSIX and ";" on Windows. Other entries (including empty ones) are
// preserved in order.
func removePathEntry(list, target string) string {
	if list == "" {
		return list
	}
	sep := string(os.PathListSeparator)
	want := filepath.Clean(target)
	parts := strings.Split(list, sep)
	kept := make([]string, 0, len(parts))
	for _, p := range parts {
		if filepath.Clean(p) == want {
			continue
		}
		kept = append(kept, p)
	}
	return strings.Join(kept, sep)
}

func TestRemovePathEntry(t *testing.T) {
	sep := string(os.PathListSeparator)
	join := func(p ...string) string { return strings.Join(p, sep) }
	shim := filepath.Join("/home", "u", ".ai", "bin")

	cases := []struct {
		name, list, target, want string
	}{
		{"removes middle", join("/usr/bin", shim, "/bin"), shim, join("/usr/bin", "/bin")},
		{"removes only entry", shim, shim, ""},
		{"absent is no-op", join("/usr/bin", "/bin"), shim, join("/usr/bin", "/bin")},
		{"matches despite trailing slash", join("/usr/bin", shim+"/"), shim, "/usr/bin"},
		{"empty list", "", shim, ""},
	}
	for _, c := range cases {
		if got := removePathEntry(c.list, c.target); got != c.want {
			t.Errorf("%s: removePathEntry(%q, %q) = %q, want %q", c.name, c.list, c.target, got, c.want)
		}
	}
}
