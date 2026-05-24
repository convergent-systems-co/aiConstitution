package cmd_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/convergent-systems-co/aiConstitution/src/cmd/ai/cmd"
)

func TestAmendApplyEndToEnd(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("AI_ROOT", dir)
	t.Setenv("AICONST_CONFIG_DIR", dir)
	t.Setenv("EDITOR", "")
	if err := os.WriteFile(filepath.Join(dir, "Common.md"), []byte(stubCommon), 0o600); err != nil {
		t.Fatal(err)
	}

	// 1) Draft.
	root := cmd.NewRootCmd()
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"amend", "draft", "Common.md/U17", "--rationale", "Strengthen U17."})
	if err := root.Execute(); err != nil {
		t.Fatalf("amend draft error: %v\noutput:%s", err, buf)
	}
	plansDir := filepath.Join(dir, "governance", "plans")
	entries, err := os.ReadDir(plansDir)
	if err != nil || len(entries) == 0 {
		t.Fatalf("no draft created")
	}
	draftPath := filepath.Join(plansDir, entries[0].Name())

	// 2) Hand-edit the draft to add a real proposed change.
	body, err := os.ReadFile(draftPath)
	if err != nil {
		t.Fatal(err)
	}
	edited := strings.Replace(string(body),
		"<!-- Replace this comment with the prose to append to the section. -->",
		"This is the appended amendment text.",
		1)
	if err := os.WriteFile(draftPath, []byte(edited), 0o600); err != nil {
		t.Fatal(err)
	}

	// 3) Apply.
	root2 := cmd.NewRootCmd()
	buf2 := &bytes.Buffer{}
	root2.SetOut(buf2)
	root2.SetErr(buf2)
	root2.SetArgs([]string{"amend", "apply", draftPath})
	if err := root2.Execute(); err != nil {
		t.Fatalf("amend apply error: %v\noutput:%s", err, buf2)
	}

	// 4) Verify.
	got, err := os.ReadFile(filepath.Join(dir, "Common.md"))
	if err != nil {
		t.Fatal(err)
	}
	g := string(got)
	if !strings.Contains(g, "This is the appended amendment text.") {
		t.Errorf("proposed change not appended:\n%s", g)
	}
	if !strings.Contains(g, "**Version:** 0.18") {
		t.Errorf("version not bumped:\n%s", g)
	}
	if !strings.Contains(g, "**0.18**") {
		t.Errorf("changelog entry missing:\n%s", g)
	}

	// 5) Audit record was written.
	overridesDir := filepath.Join(dir, "audit", "overrides")
	auditEntries, err := os.ReadDir(overridesDir)
	if err != nil || len(auditEntries) == 0 {
		t.Errorf("no audit/overrides record written in %s", overridesDir)
	}
}
