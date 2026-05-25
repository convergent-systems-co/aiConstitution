package cmd_test

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// makeTarGz creates a .tar.gz archive at dest containing one file
// "constitution.md" with the given content.
func makeTarGz(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	dest := filepath.Join(dir, "snapshot.tar.gz")
	f, err := os.Create(dest)
	if err != nil {
		t.Fatalf("create tar.gz: %v", err)
	}
	defer f.Close()
	gw := gzip.NewWriter(f)
	tw := tar.NewWriter(gw)
	body := []byte(content)
	hdr := &tar.Header{
		Name: "constitution.md",
		Mode: 0o644,
		Size: int64(len(body)),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatalf("tar header: %v", err)
	}
	if _, err := tw.Write(body); err != nil {
		t.Fatalf("tar write: %v", err)
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("tar close: %v", err)
	}
	if err := gw.Close(); err != nil {
		t.Fatalf("gzip close: %v", err)
	}
	return dest
}

// runRestore runs the cobra root command with `restore <args...>` and
// captures stdout/stderr.
func runRestore(t *testing.T, aiRoot string, args ...string) (string, error) {
	t.Helper()
	t.Setenv("AI_ROOT", aiRoot)
	return runRootCmd(t, append([]string{"restore"}, args...)...)
}

// Test_restore_extracts_targz verifies that `ai restore <path>` extracts
// the tar.gz into AIRoot.
func Test_restore_extracts_targz(t *testing.T) {
	snapshot := makeTarGz(t, "# test constitution")
	aiRoot := t.TempDir()
	// Remove aiRoot so there is nothing to back up.
	if err := os.RemoveAll(aiRoot); err != nil {
		t.Fatalf("remove aiRoot: %v", err)
	}

	stdout, err := runRestore(t, aiRoot, snapshot)
	if err != nil {
		t.Fatalf("restore failed: %v", err)
	}

	extracted := filepath.Join(aiRoot, "constitution.md")
	if _, statErr := os.Stat(extracted); statErr != nil {
		t.Fatalf("expected %s to exist after restore, got: %v", extracted, statErr)
	}

	if !strings.Contains(stdout, fmt.Sprintf("Restored from %s", snapshot)) {
		t.Errorf("expected stdout to contain 'Restored from %s'; got: %q", snapshot, stdout)
	}
}

// Test_restore_backs_up_existing verifies that `ai restore` snapshots an
// existing AIRoot to a sibling .ai-backup-<UTC>/ before overwriting.
func Test_restore_backs_up_existing(t *testing.T) {
	snapshot := makeTarGz(t, "# new constitution")
	aiRoot := t.TempDir()
	// Plant a sentinel file in the existing AIRoot.
	sentinel := filepath.Join(aiRoot, "existing-file.md")
	if err := os.WriteFile(sentinel, []byte("old"), 0o644); err != nil {
		t.Fatalf("write sentinel: %v", err)
	}

	stdout, err := runRestore(t, aiRoot, snapshot)
	if err != nil {
		t.Fatalf("restore failed: %v", err)
	}

	// stdout must mention a backup path
	if !strings.Contains(stdout, "backed up to") {
		t.Errorf("expected stdout to contain 'backed up to'; got: %q", stdout)
	}

	// Find the backup dir — it must contain the sentinel.
	parent := filepath.Dir(aiRoot)
	entries, err := os.ReadDir(parent)
	if err != nil {
		t.Fatalf("read parent: %v", err)
	}
	var backupDir string
	for _, e := range entries {
		if strings.Contains(e.Name(), "ai-backup-") && e.IsDir() {
			backupDir = filepath.Join(parent, e.Name())
		}
	}
	if backupDir == "" {
		t.Fatal("expected a .ai-backup-* directory to exist after restore")
	}
	backupSentinel := filepath.Join(backupDir, "existing-file.md")
	if _, statErr := os.Stat(backupSentinel); statErr != nil {
		t.Errorf("sentinel should have been backed up to %s: %v", backupSentinel, statErr)
	}
}

// Test_restore_from_url_local uses a file:// URL (pointing at a real temp
// .tar.gz) to exercise the --from-url download path without network.
func Test_restore_from_url_local(t *testing.T) {
	snapshot := makeTarGz(t, "# from url")
	aiRoot := t.TempDir()
	if err := os.RemoveAll(aiRoot); err != nil {
		t.Fatalf("remove aiRoot: %v", err)
	}

	// Construct a file:// URL.
	fileURL := "file://" + snapshot

	stdout, err := runRestore(t, aiRoot, "--from-url", fileURL)
	if err != nil {
		t.Fatalf("restore --from-url failed: %v", err)
	}

	extracted := filepath.Join(aiRoot, "constitution.md")
	if _, statErr := os.Stat(extracted); statErr != nil {
		t.Fatalf("expected %s to exist after --from-url restore: %v", extracted, statErr)
	}

	if !strings.Contains(stdout, "Restored from") {
		t.Errorf("expected 'Restored from' in stdout; got: %q", stdout)
	}
}

// Test_restore_error_missing_file verifies an error when the path doesn't exist.
func Test_restore_error_missing_file(t *testing.T) {
	aiRoot := t.TempDir()
	_, err := runRestore(t, aiRoot, "/nonexistent/snapshot.tar.gz")
	if err == nil {
		t.Fatal("expected an error for missing snapshot, got nil")
	}
}

// Test_restore_rejects_path_traversal verifies that a tar.gz containing a
// path-traversal entry (../../etc/evil) is rejected before any extraction.
func Test_restore_rejects_path_traversal(t *testing.T) {
	dir := t.TempDir()
	malicious := filepath.Join(dir, "malicious.tar.gz")
	f, err := os.Create(malicious)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	gw := gzip.NewWriter(f)
	tw := tar.NewWriter(gw)
	body := []byte("owned")
	_ = tw.WriteHeader(&tar.Header{Name: "../../etc/evil", Mode: 0o644, Size: int64(len(body))})
	_, _ = tw.Write(body)
	tw.Close()
	gw.Close()
	f.Close()

	aiRoot := t.TempDir()
	_, err = runRestore(t, aiRoot, malicious)
	if err == nil {
		t.Fatal("expected error for path-traversal entry, got nil")
	}
	if !strings.Contains(err.Error(), "path-traversal") && !strings.Contains(err.Error(), "refusing") {
		t.Errorf("expected path-traversal error; got: %q", err)
	}
}

// Test_restore_error_bad_extension verifies an error for unsupported extensions.
func Test_restore_error_bad_extension(t *testing.T) {
	dir := t.TempDir()
	badFile := filepath.Join(dir, "snapshot.zip")
	if err := os.WriteFile(badFile, []byte("data"), 0o644); err != nil {
		t.Fatalf("create bad file: %v", err)
	}
	aiRoot := t.TempDir()
	_, err := runRestore(t, aiRoot, badFile)
	if err == nil {
		t.Fatal("expected an error for bad extension, got nil")
	}
	if !strings.Contains(err.Error(), "extension") && !strings.Contains(err.Error(), ".tar.gz") {
		t.Errorf("error message should mention extension or .tar.gz; got: %q", err)
	}
}
