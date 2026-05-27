package embed

import (
	"testing"
)

// TestHookPermissions verifies that executableForName returns 0o755 for Python
// hook files and 0o644 for all other file types.  Python hooks need +x so the
// shell can invoke them directly without an explicit interpreter prefix.
func TestHookPermissions(t *testing.T) {
	cases := []struct {
		name string
		want uint32
	}{
		{"audit.py", 0o755},
		{"branch-guard.py", 0o755},
		{"worktree-guard.py", 0o755},
		{"patterns.json", 0o644},
		{"config.toml", 0o644},
		{"README.md", 0o644},
		{"script.sh", 0o644},
	}
	for _, tc := range cases {
		got := executableForName(tc.name)
		if uint32(got) != tc.want {
			t.Errorf("executableForName(%q) = 0o%o, want 0o%o", tc.name, got, tc.want)
		}
	}
}
