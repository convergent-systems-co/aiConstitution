package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func runMemoryCmd(t *testing.T, args ...string) (string, error) {
	t.Helper()
	root := NewRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs(append([]string{"memory"}, args...))
	err := root.Execute()
	return buf.String(), err
}

func helperMemoryAIRoot(t *testing.T) string {
	t.Helper()
	aiRoot := t.TempDir()
	t.Setenv("AI_ROOT", aiRoot)
	_ = os.MkdirAll(filepath.Join(aiRoot, "memory", "archived"), 0o755)
	return aiRoot
}

func writeMemoryMD(t *testing.T, aiRoot, content string) {
	t.Helper()
	_ = os.WriteFile(filepath.Join(aiRoot, "memory", "MEMORY.md"), []byte(content), 0o644)
}

// ---- memory list (#171) ----

func TestMemoryList_ParsesMEMORYMD(t *testing.T) {
	aiRoot := helperMemoryAIRoot(t)
	writeMemoryMD(t, aiRoot, "# Memory Index\n\n## Feedback\n- [foo](feedback_foo.md) — a foo\n- [bar](feedback_bar.md) — a bar\n")
	// Create actual memory files with frontmatter
	fooContent := "---\nname: foo\ndescription: a foo\nmetadata:\n  type: feedback\n---\n\n# Body\n"
	barContent := "---\nname: bar\ndescription: a bar\nmetadata:\n  type: feedback\n---\n\n# Body\n"
	_ = os.WriteFile(filepath.Join(aiRoot, "memory", "feedback_foo.md"), []byte(fooContent), 0o644)
	_ = os.WriteFile(filepath.Join(aiRoot, "memory", "feedback_bar.md"), []byte(barContent), 0o644)

	out, err := runMemoryCmd(t, "list")
	if err != nil && strings.Contains(err.Error(), "not yet implemented") {
		t.Fatalf("memory list returned stub error: %v", err)
	}
	if !strings.Contains(out, "foo") {
		t.Errorf("expected 'foo' in output\n%s", out)
	}
	if !strings.Contains(out, "bar") {
		t.Errorf("expected 'bar' in output\n%s", out)
	}
}

func TestMemoryList_TypeFlag(t *testing.T) {
	aiRoot := helperMemoryAIRoot(t)
	writeMemoryMD(t, aiRoot, "# Memory Index\n\n## Feedback\n- [fb](feedback_fb.md) — feedback one\n## Reference\n- [ref](reference_ref.md) — reference one\n")
	_ = os.WriteFile(filepath.Join(aiRoot, "memory", "feedback_fb.md"),
		[]byte("---\nname: fb\ndescription: feedback one\nmetadata:\n  type: feedback\n---\n"), 0o644)
	_ = os.WriteFile(filepath.Join(aiRoot, "memory", "reference_ref.md"),
		[]byte("---\nname: ref\ndescription: reference one\nmetadata:\n  type: reference\n---\n"), 0o644)

	out, err := runMemoryCmd(t, "list", "--type", "feedback")
	if err != nil && strings.Contains(err.Error(), "not yet implemented") {
		t.Fatalf("memory list --type returned stub error: %v", err)
	}
	if !strings.Contains(out, "fb") {
		t.Errorf("expected 'fb' in filtered output\n%s", out)
	}
	if strings.Contains(out, "ref") {
		t.Errorf("expected 'ref' to be filtered out\n%s", out)
	}
}

func TestMemoryList_NoStub(t *testing.T) {
	aiRoot := helperMemoryAIRoot(t)
	writeMemoryMD(t, aiRoot, "# Memory Index\n")

	_, err := runMemoryCmd(t, "list")
	if err != nil && strings.Contains(err.Error(), "not yet implemented") {
		t.Fatalf("memory list returned stub error: %v", err)
	}
}

// ---- memory show (#171) ----

func TestMemoryShow_OutputsFileContent(t *testing.T) {
	aiRoot := helperMemoryAIRoot(t)
	content := "---\nname: myslug\ndescription: my description\nmetadata:\n  type: reference\n---\n\n## Rule\nSome rule.\n"
	_ = os.WriteFile(filepath.Join(aiRoot, "memory", "reference_myslug.md"), []byte(content), 0o644)
	writeMemoryMD(t, aiRoot, "# Memory Index\n\n## Reference\n- [myslug](reference_myslug.md) — my description\n")

	out, err := runMemoryCmd(t, "show", "myslug")
	if err != nil && strings.Contains(err.Error(), "not yet implemented") {
		t.Fatalf("memory show returned stub error: %v", err)
	}
	if !strings.Contains(out, "Some rule.") {
		t.Errorf("expected file content in output\n%s", out)
	}
}

func TestMemoryShow_UnknownSlug_ReturnsError(t *testing.T) {
	aiRoot := helperMemoryAIRoot(t)
	writeMemoryMD(t, aiRoot, "# Memory Index\n")

	_, err := runMemoryCmd(t, "show", "nonexistent-slug")
	if err == nil {
		t.Error("expected error for unknown slug, got nil")
	}
	if err != nil && strings.Contains(err.Error(), "not yet implemented") {
		t.Fatalf("memory show returned stub error: %v", err)
	}
}

// ---- memory archive (#171) ----

func TestMemoryArchive_MovesFile(t *testing.T) {
	aiRoot := helperMemoryAIRoot(t)
	content := "---\nname: archiveme\ndescription: archive this\nmetadata:\n  type: feedback\n---\n"
	_ = os.WriteFile(filepath.Join(aiRoot, "memory", "feedback_archiveme.md"), []byte(content), 0o644)
	writeMemoryMD(t, aiRoot, "# Memory Index\n\n## Feedback\n- [archiveme](feedback_archiveme.md) — archive this\n")

	out, err := runMemoryCmd(t, "archive", "archiveme")
	if err != nil && strings.Contains(err.Error(), "not yet implemented") {
		t.Fatalf("memory archive returned stub error: %v", err)
	}
	_ = out

	// Original file should be gone
	origPath := filepath.Join(aiRoot, "memory", "feedback_archiveme.md")
	if _, statErr := os.Stat(origPath); !os.IsNotExist(statErr) {
		t.Errorf("expected original file %s to be removed", origPath)
	}

	// Archived file should exist
	archivedPath := filepath.Join(aiRoot, "memory", "archived", "archiveme.md")
	if _, statErr := os.Stat(archivedPath); os.IsNotExist(statErr) {
		t.Errorf("expected archived file at %s", archivedPath)
	}
}

func TestMemoryArchive_RemovesMEMORYMDLine(t *testing.T) {
	aiRoot := helperMemoryAIRoot(t)
	content := "---\nname: removeentry\ndescription: remove from index\nmetadata:\n  type: feedback\n---\n"
	_ = os.WriteFile(filepath.Join(aiRoot, "memory", "feedback_removeentry.md"), []byte(content), 0o644)
	writeMemoryMD(t, aiRoot, "# Memory Index\n\n## Feedback\n- [removeentry](feedback_removeentry.md) — remove from index\n- [keep](feedback_keep.md) — keep this\n")

	out, err := runMemoryCmd(t, "archive", "removeentry")
	if err != nil && strings.Contains(err.Error(), "not yet implemented") {
		t.Fatalf("memory archive returned stub error: %v", err)
	}
	_ = out

	memContent, _ := os.ReadFile(filepath.Join(aiRoot, "memory", "MEMORY.md"))
	if strings.Contains(string(memContent), "removeentry") {
		t.Errorf("expected 'removeentry' line to be removed from MEMORY.md\n%s", string(memContent))
	}
	if !strings.Contains(string(memContent), "keep") {
		t.Errorf("expected 'keep' line to remain in MEMORY.md\n%s", string(memContent))
	}
}

// ---- memory codify (#170) ----

func TestMemoryCodify_WritesMemoryFile(t *testing.T) {
	aiRoot := helperMemoryAIRoot(t)
	writeMemoryMD(t, aiRoot, "# Memory Index\n")

	// Create a fake violation file
	violationContent := "# Violation — 2026-05-24T10:00:00Z\n\n" +
		"- **File / Rule violated:** Common.md §U14 — Independent verification\n" +
		"- **What happened:** Accepted stale handoff.\n" +
		"- **How noticed:** Self-detected\n" +
		"- **Remediation:** Verified live state.\n" +
		"- **Proposed amendment (if any):** Add verification step.\n"
	violationPath := filepath.Join(t.TempDir(), "test-violation.md")
	_ = os.WriteFile(violationPath, []byte(violationContent), 0o644)

	out, err := runMemoryCmd(t, "codify", violationPath,
		"--type", "feedback",
		"--slug", "handoff-verification",
		"--description", "Verify handoff claims before acting",
	)
	if err != nil && strings.Contains(err.Error(), "not yet implemented") {
		t.Fatalf("memory codify returned stub error: %v", err)
	}
	_ = out

	memFile := filepath.Join(aiRoot, "memory", "feedback_handoff-verification.md")
	if _, statErr := os.Stat(memFile); os.IsNotExist(statErr) {
		t.Errorf("expected memory file at %s", memFile)
	}
}

func TestMemoryCodify_UpdatesMEMORYMD(t *testing.T) {
	aiRoot := helperMemoryAIRoot(t)
	writeMemoryMD(t, aiRoot, "# Memory Index\n")

	violationContent := "# Violation — 2026-05-24T10:00:00Z\n\n" +
		"- **File / Rule violated:** Code.md §11.8\n" +
		"- **What happened:** Pipeline step skipped.\n" +
		"- **Proposed amendment (if any):** Enforce pipeline.\n"
	violationPath := filepath.Join(t.TempDir(), "violation.md")
	_ = os.WriteFile(violationPath, []byte(violationContent), 0o644)

	_, err := runMemoryCmd(t, "codify", violationPath,
		"--type", "feedback",
		"--slug", "pipeline-enforcement",
		"--description", "Never skip pipeline steps",
	)
	if err != nil && strings.Contains(err.Error(), "not yet implemented") {
		t.Fatalf("memory codify returned stub error: %v", err)
	}

	memMD, _ := os.ReadFile(filepath.Join(aiRoot, "memory", "MEMORY.md"))
	if !strings.Contains(string(memMD), "pipeline-enforcement") {
		t.Errorf("expected MEMORY.md to contain pointer to new memory\n%s", string(memMD))
	}
}

func TestMemoryCodify_MemoryFileHasFrontmatter(t *testing.T) {
	aiRoot := helperMemoryAIRoot(t)
	writeMemoryMD(t, aiRoot, "# Memory Index\n")

	violationContent := "# Violation — 2026-05-24T10:00:00Z\n\n" +
		"- **File / Rule violated:** Common.md §U15\n" +
		"- **What happened:** Loop exceeded 5 cycles.\n" +
		"- **Proposed amendment (if any):** none\n"
	violationPath := filepath.Join(t.TempDir(), "violation.md")
	_ = os.WriteFile(violationPath, []byte(violationContent), 0o644)

	_, err := runMemoryCmd(t, "codify", violationPath,
		"--type", "reference",
		"--slug", "loop-cap",
		"--description", "Loop cap enforcement",
	)
	if err != nil && strings.Contains(err.Error(), "not yet implemented") {
		t.Fatalf("memory codify returned stub error: %v", err)
	}

	memFile := filepath.Join(aiRoot, "memory", "reference_loop-cap.md")
	data, readErr := os.ReadFile(memFile)
	if readErr != nil {
		t.Fatalf("reading memory file: %v", readErr)
	}
	body := string(data)
	if !strings.Contains(body, "---") {
		t.Errorf("expected YAML frontmatter delimiter in memory file\n%s", body)
	}
	if !strings.Contains(body, "name: loop-cap") {
		t.Errorf("expected name in frontmatter\n%s", body)
	}
	if !strings.Contains(body, "type: reference") {
		t.Errorf("expected type in frontmatter\n%s", body)
	}
}

func TestMemoryCodify_MissingViolationFile_ReturnsError(t *testing.T) {
	aiRoot := helperMemoryAIRoot(t)
	writeMemoryMD(t, aiRoot, "# Memory Index\n")

	_, err := runMemoryCmd(t, "codify", "/nonexistent/path/to/violation.md",
		"--type", "feedback",
		"--slug", "test",
		"--description", "test",
	)
	if err == nil {
		t.Error("expected error for nonexistent violation file, got nil")
	}
	if err != nil && strings.Contains(err.Error(), "not yet implemented") {
		t.Fatalf("memory codify returned stub error: %v", err)
	}
}

// ---- memory retire (#384) ----

func TestMemoryRetire_HappyPath(t *testing.T) {
	aiRoot := helperMemoryAIRoot(t)
	memDir := filepath.Join(aiRoot, "memory")
	_ = os.MkdirAll(memDir, 0o755)

	content := "---\nname: retireable\ndescription: retire this\nmetadata:\n  type: feedback\n---\n"
	_ = os.WriteFile(filepath.Join(memDir, "retireable.md"), []byte(content), 0o644)
	writeMemoryMD(t, aiRoot, "# Memory Index\n\n## Feedback\n- [retireable](retireable.md) — retire this\n- [keeper](keeper.md) — keep this\n")

	out, err := runMemoryCmd(t, "retire", "retireable")
	if err != nil {
		t.Fatalf("memory retire returned error: %v\nout: %s", err, out)
	}
	if strings.Contains(out, "not yet implemented") {
		t.Fatalf("memory retire returned stub output: %s", out)
	}

	// Original file should be gone
	origPath := filepath.Join(memDir, "retireable.md")
	if _, statErr := os.Stat(origPath); !os.IsNotExist(statErr) {
		t.Errorf("expected original file %s to be removed", origPath)
	}

	// Retired file should exist in retired/ dir with timestamp prefix
	retiredDir := filepath.Join(memDir, "retired")
	entries, err := os.ReadDir(retiredDir)
	if err != nil {
		t.Fatalf("retired dir not found: %v", err)
	}
	found := false
	for _, e := range entries {
		if strings.Contains(e.Name(), "retireable") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected a retired file containing 'retireable' in %s", retiredDir)
	}

	// Output should include the move information
	if !strings.Contains(out, "Retired") {
		t.Errorf("expected 'Retired' in output, got: %s", out)
	}
}

func TestMemoryRetire_MissingFile_ReturnsError(t *testing.T) {
	aiRoot := helperMemoryAIRoot(t)
	writeMemoryMD(t, aiRoot, "# Memory Index\n")

	_, err := runMemoryCmd(t, "retire", "nonexistent-memory")
	if err == nil {
		t.Error("expected error for nonexistent memory file, got nil")
	}
	if err != nil && strings.Contains(err.Error(), "not yet implemented") {
		t.Fatalf("memory retire returned stub error: %v", err)
	}
}

func TestMemoryRetire_RemovesMEMORYMDLine(t *testing.T) {
	aiRoot := helperMemoryAIRoot(t)
	memDir := filepath.Join(aiRoot, "memory")
	_ = os.MkdirAll(memDir, 0o755)

	content := "---\nname: bye\ndescription: goodbye\nmetadata:\n  type: feedback\n---\n"
	_ = os.WriteFile(filepath.Join(memDir, "bye.md"), []byte(content), 0o644)
	writeMemoryMD(t, aiRoot, "# Memory Index\n\n## Feedback\n- [bye](bye.md) — goodbye\n- [stay](stay.md) — stays\n")

	_, err := runMemoryCmd(t, "retire", "bye")
	if err != nil {
		t.Fatalf("memory retire failed: %v", err)
	}

	memContent, _ := os.ReadFile(filepath.Join(memDir, "MEMORY.md"))
	if strings.Contains(string(memContent), "bye.md") {
		t.Errorf("expected 'bye.md' line removed from MEMORY.md\n%s", string(memContent))
	}
	if !strings.Contains(string(memContent), "stay") {
		t.Errorf("expected 'stay' line to remain in MEMORY.md\n%s", string(memContent))
	}
}

func TestMemoryRetire_CreatesRetiredDir(t *testing.T) {
	aiRoot := t.TempDir()
	t.Setenv("AI_ROOT", aiRoot)
	memDir := filepath.Join(aiRoot, "memory")
	_ = os.MkdirAll(memDir, 0o755)
	// Do NOT pre-create retired/ dir

	content := "---\nname: createdir\ndescription: test\nmetadata:\n  type: feedback\n---\n"
	_ = os.WriteFile(filepath.Join(memDir, "createdir.md"), []byte(content), 0o644)
	// No MEMORY.md — that's ok, pruning is best-effort

	_, err := runMemoryCmd(t, "retire", "createdir")
	if err != nil {
		t.Fatalf("memory retire failed: %v", err)
	}

	retiredDir := filepath.Join(memDir, "retired")
	if _, statErr := os.Stat(retiredDir); os.IsNotExist(statErr) {
		t.Errorf("expected retired dir to be created at %s", retiredDir)
	}
}
