package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// profileTestEnv sets up a temp ConfigDir with a profiles/ subdirectory
// and injects it via AICONST_CONFIG_DIR so paths.ConfigDir() resolves
// to our temp tree. Returns the profiles dir path and a cleanup func.
func profileTestEnv(t *testing.T) (profilesDir string, cleanup func()) {
	t.Helper()
	tmp := t.TempDir()
	profilesDir = filepath.Join(tmp, "profiles")
	if err := os.MkdirAll(profilesDir, 0o750); err != nil {
		t.Fatalf("mkdir profiles: %v", err)
	}
	t.Setenv("AICONST_CONFIG_DIR", tmp)
	return profilesDir, func() {} // t.TempDir auto-cleans; cleanup is a no-op
}

// writeProfileYAML writes a minimal YAML profile file with the given frontmatter
// fields into dir/<name>.yaml.
func writeProfileYAML(t *testing.T, dir, name, description string) {
	t.Helper()
	content := "name: " + name + "\ndescription: \"" + description + "\"\ndomains: []\n"
	if err := os.WriteFile(filepath.Join(dir, name+".yaml"), []byte(content), 0o644); err != nil {
		t.Fatalf("write profile yaml: %v", err)
	}
}

// execProfile runs the profile subcommand with args and captures stdout.
func execProfile(t *testing.T, args ...string) (stdout string, err error) {
	t.Helper()
	var buf bytes.Buffer
	root := NewRootCmd()
	root.SetOut(&buf)
	root.SetErr(&bytes.Buffer{}) // suppress stderr noise
	root.SetArgs(append([]string{"profile"}, args...))
	err = root.Execute()
	return buf.String(), err
}

// ---------- #215 profile list ----------

func TestProfileList_ShowsNameAndDescription(t *testing.T) {
	dir, _ := profileTestEnv(t)
	writeProfileYAML(t, dir, "coding", "Coder persona")
	writeProfileYAML(t, dir, "review", "Reviewer persona")

	out, err := execProfile(t, "list")
	if err != nil {
		t.Fatalf("profile list returned error: %v", err)
	}
	if !strings.Contains(out, "coding") {
		t.Errorf("expected 'coding' in output, got:\n%s", out)
	}
	if !strings.Contains(out, "Coder persona") {
		t.Errorf("expected 'Coder persona' in output, got:\n%s", out)
	}
	if !strings.Contains(out, "review") {
		t.Errorf("expected 'review' in output, got:\n%s", out)
	}
	if !strings.Contains(out, "Reviewer persona") {
		t.Errorf("expected 'Reviewer persona' in output, got:\n%s", out)
	}
}

func TestProfileList_EmptyDir_PrintsNotice(t *testing.T) {
	_, _ = profileTestEnv(t) // creates empty profiles dir

	out, err := execProfile(t, "list")
	if err != nil {
		t.Fatalf("profile list returned error: %v", err)
	}
	if !strings.Contains(out, "(no profiles)") {
		t.Errorf("expected '(no profiles)' for empty dir, got:\n%s", out)
	}
}

func TestProfileList_DirNotExist_PrintsNotice(t *testing.T) {
	tmp := t.TempDir()
	// Set config dir to a temp dir that has no profiles/ subdir.
	t.Setenv("AICONST_CONFIG_DIR", tmp)

	out, err := execProfile(t, "list")
	if err != nil {
		t.Fatalf("profile list returned error: %v", err)
	}
	if !strings.Contains(out, "(no profiles)") {
		t.Errorf("expected '(no profiles)' when dir missing, got:\n%s", out)
	}
}

// ---------- #215 profile show ----------

func TestProfileShow_PrintsFileContent(t *testing.T) {
	dir, _ := profileTestEnv(t)
	writeProfileYAML(t, dir, "myprofile", "My test profile")

	out, err := execProfile(t, "show", "myprofile")
	if err != nil {
		t.Fatalf("profile show returned error: %v", err)
	}
	if !strings.Contains(out, "name: myprofile") {
		t.Errorf("expected file content in output, got:\n%s", out)
	}
	if !strings.Contains(out, "My test profile") {
		t.Errorf("expected description in output, got:\n%s", out)
	}
}

func TestProfileShow_NotFound_ReturnsError(t *testing.T) {
	_, _ = profileTestEnv(t)

	_, err := execProfile(t, "show", "nonexistent")
	if err == nil {
		t.Fatal("expected error for missing profile, got nil")
	}
}

// ---------- #216 profile new ----------

func TestProfileNew_CreatesFileWithFrontmatter(t *testing.T) {
	dir, _ := profileTestEnv(t)

	_, err := execProfile(t, "new", "myprofile")
	if err != nil {
		t.Fatalf("profile new returned error: %v", err)
	}

	expected := filepath.Join(dir, "myprofile.yaml")
	data, readErr := os.ReadFile(expected)
	if readErr != nil {
		t.Fatalf("expected file %s to exist, got: %v", expected, readErr)
	}
	content := string(data)
	if !strings.Contains(content, "name: myprofile") {
		t.Errorf("expected 'name: myprofile' in frontmatter, got:\n%s", content)
	}
	if !strings.Contains(content, `description: ""`) {
		t.Errorf("expected 'description: \"\"' in frontmatter, got:\n%s", content)
	}
	if !strings.Contains(content, "domains: []") {
		t.Errorf("expected 'domains: []' in frontmatter, got:\n%s", content)
	}
}

func TestProfileNew_AlreadyExists_ReturnsError(t *testing.T) {
	dir, _ := profileTestEnv(t)
	writeProfileYAML(t, dir, "existing", "already here")

	_, err := execProfile(t, "new", "existing")
	if err == nil {
		t.Fatal("expected error when profile already exists, got nil")
	}
}

// ---------- #216 profile edit ----------

func TestProfileEdit_EditorUnset_PrintsPath(t *testing.T) {
	dir, _ := profileTestEnv(t)
	writeProfileYAML(t, dir, "myprofile", "desc")
	t.Setenv("EDITOR", "") // ensure EDITOR is not set

	out, err := execProfile(t, "edit", "myprofile")
	if err != nil {
		t.Fatalf("profile edit returned error: %v", err)
	}
	expected := filepath.Join(dir, "myprofile.yaml")
	if !strings.Contains(out, expected) {
		t.Errorf("expected path %q in output, got:\n%s", expected, out)
	}
}

func TestProfileEdit_NotFound_ReturnsError(t *testing.T) {
	_, _ = profileTestEnv(t)
	t.Setenv("EDITOR", "")

	_, err := execProfile(t, "edit", "ghost")
	if err == nil {
		t.Fatal("expected error for missing profile in edit, got nil")
	}
}

// ---------- #216 profile remove ----------

func TestProfileRemove_DeletesFile(t *testing.T) {
	dir, _ := profileTestEnv(t)
	writeProfileYAML(t, dir, "deleteme", "bye")

	_, err := execProfile(t, "remove", "deleteme")
	if err != nil {
		t.Fatalf("profile remove returned error: %v", err)
	}

	path := filepath.Join(dir, "deleteme.yaml")
	if _, statErr := os.Stat(path); statErr == nil {
		t.Errorf("expected file %s to be deleted, but it still exists", path)
	}
}

func TestProfileRemove_NotFound_ReturnsError(t *testing.T) {
	_, _ = profileTestEnv(t)

	_, err := execProfile(t, "remove", "ghost")
	if err == nil {
		t.Fatal("expected error for missing profile in remove, got nil")
	}
}
