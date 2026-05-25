package constitution_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/convergent-systems-co/aiConstitution/src/internal/constitution"
)

func TestLoadUnified_ReadsConstitutionMD(t *testing.T) {
	root := t.TempDir()
	_ = os.WriteFile(filepath.Join(root, "Constitution.md"),
		[]byte("# Unified\n\n## §1 Governance\nGov content here."), 0o600)

	unified, err := constitution.LoadUnified(root)
	if err != nil {
		t.Fatalf("LoadUnified() error: %v", err)
	}
	if !strings.Contains(unified.Content, "§1 Governance") {
		t.Error("unified content missing expected section")
	}
	if unified.Local != "" {
		t.Error("Local should be empty when Constitution.local.md absent")
	}
}

func TestLoadUnified_MissingFile_ReturnsError(t *testing.T) {
	root := t.TempDir()
	_, err := constitution.LoadUnified(root)
	if err == nil {
		t.Error("expected error when Constitution.md missing")
	}
}

func TestLoadUnified_IncludesLocalOverride(t *testing.T) {
	root := t.TempDir()
	_ = os.WriteFile(filepath.Join(root, "Constitution.md"), []byte("# Main"), 0o600)
	_ = os.WriteFile(filepath.Join(root, "Constitution.local.md"), []byte("# Local override"), 0o600)

	unified, err := constitution.LoadUnified(root)
	if err != nil {
		t.Fatalf("LoadUnified() error: %v", err)
	}
	if unified.Local == "" {
		t.Error("Local should be populated when Constitution.local.md present")
	}
}

func TestFileStatusV2_DetectsUnifiedLayout(t *testing.T) {
	root := t.TempDir()
	_ = os.WriteFile(filepath.Join(root, "Constitution.md"), []byte("# Unified"), 0o600)

	status := constitution.FileStatusV2(root)
	if !status["Constitution.md"] {
		t.Error("Constitution.md should be present")
	}
	if !status["v2"] {
		t.Error("v2 should be true when only Constitution.md present (no Common.md)")
	}
}

func TestFileStatusV2_DetectsLegacyLayout(t *testing.T) {
	root := t.TempDir()
	for _, f := range []string{"Constitution.md", "Common.md", "Code.md", "Writing.md"} {
		_ = os.WriteFile(filepath.Join(root, f), []byte("# "+f), 0o600)
	}
	status := constitution.FileStatusV2(root)
	if status["v2"] {
		t.Error("v2 should be false when Common.md is present (legacy layout)")
	}
}
