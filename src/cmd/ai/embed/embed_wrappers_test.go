package embed

import (
	"runtime"
	"strings"
	"testing"
)

func TestWrapperAppliesOnOS_POSIX(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX-only assertion; run on Linux/macOS")
	}
	t.Parallel()

	// Bare git/gh should be included on POSIX.
	for _, name := range []string{"git", "gh"} {
		if !wrapperAppliesOnOS(name) {
			t.Errorf("POSIX: %q should apply (it is the bash shim)", name)
		}
	}
	// .cmd and .ps1 should be excluded on POSIX.
	for _, name := range []string{"git.cmd", "git.ps1", "gh.cmd", "gh.ps1"} {
		if wrapperAppliesOnOS(name) {
			t.Errorf("POSIX: %q should NOT apply", name)
		}
	}
	// notify-me variants should always apply.
	for _, name := range []string{"notify-me", "notify-me.cmd", "notify-me.ps1"} {
		if !wrapperAppliesOnOS(name) {
			t.Errorf("POSIX: %q should always apply", name)
		}
	}
}

func TestWrapperAppliesOnOS_Windows(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows-only assertion")
	}
	t.Parallel()

	// .cmd and .ps1 should be included on Windows.
	for _, name := range []string{"git.cmd", "git.ps1", "gh.cmd", "gh.ps1"} {
		if !wrapperAppliesOnOS(name) {
			t.Errorf("Windows: %q should apply", name)
		}
	}
	// Bare git/gh should be excluded on Windows (bash not available).
	for _, name := range []string{"git", "gh"} {
		if wrapperAppliesOnOS(name) {
			t.Errorf("Windows: bare %q should NOT apply (no bash)", name)
		}
	}
}

func TestExtractWrappers_PlatformFilter(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	written, err := ExtractWrappers(tmp, false)
	if err != nil {
		t.Fatalf("ExtractWrappers: %v", err)
	}

	names := make(map[string]bool, len(written))
	for _, p := range written {
		base := p[strings.LastIndexByte(p, '/')+1:]
		if i := strings.LastIndexByte(base, '\\'); i >= 0 {
			base = base[i+1:]
		}
		names[base] = true
	}

	if runtime.GOOS == "windows" {
		for _, want := range []string{"git.cmd", "git.ps1", "gh.cmd", "gh.ps1"} {
			if !names[want] {
				t.Errorf("Windows: expected %q extracted, got %v", want, names)
			}
		}
		for _, notwant := range []string{"git", "gh"} {
			if names[notwant] {
				t.Errorf("Windows: bare %q should NOT be extracted", notwant)
			}
		}
	} else {
		for _, want := range []string{"git", "gh"} {
			if !names[want] {
				t.Errorf("POSIX: expected %q extracted, got %v", want, names)
			}
		}
		for _, notwant := range []string{"git.cmd", "git.ps1", "gh.cmd", "gh.ps1"} {
			if names[notwant] {
				t.Errorf("POSIX: %q should NOT be extracted on non-Windows", notwant)
			}
		}
	}
}
