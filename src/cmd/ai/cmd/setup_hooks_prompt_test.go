package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/convergent-systems-co/aiConstitution/src/cmd/ai/embed"
)

// testHookInstallRecorder returns a hookInstallFn that records which hooks were installed.
func testHookInstallRecorder(installed *[]string) hookInstallFn {
	return func(name, _ string, _ bool) error {
		*installed = append(*installed, name)
		return nil
	}
}

// testWireRecorder returns a hookWireFn that records calls.
func testWireRecorder(called *bool) hookWireFn {
	return func(_, _ string) error {
		*called = true
		return nil
	}
}

// TestHookSelectionPrompt_InstallsSelected verifies that selecting hook #1 by
// number installs only that hook and calls the wire function.
func TestHookSelectionPrompt_InstallsSelected(t *testing.T) {
	names, err := embed.HookNames()
	if err != nil {
		t.Fatalf("embed.HookNames: %v", err)
	}
	// Collect the rows that runHookSelectionPrompt will show.
	var rows []string
	for _, n := range names {
		if isHookFile(n) || n == "patterns.json" {
			rows = append(rows, n)
		}
	}
	if len(rows) == 0 {
		t.Skip("no embedded hooks available")
	}

	var installed []string
	wireCalled := false
	var out bytes.Buffer
	input := strings.NewReader("1\n")

	err = runHookSelectionPrompt(
		&out, input, true,
		t.TempDir(),
		testHookInstallRecorder(&installed),
		testWireRecorder(&wireCalled),
		t.TempDir(),
	)
	if err != nil {
		t.Fatalf("runHookSelectionPrompt: %v", err)
	}

	if len(installed) != 1 {
		t.Errorf("InstallsSelected: installed %d hooks, want 1; got %v", len(installed), installed)
	}
	if installed[0] != rows[0] {
		t.Errorf("InstallsSelected: want %q, got %q", rows[0], installed[0])
	}
	if !wireCalled {
		t.Error("InstallsSelected: wire function was not called")
	}
}

// TestHookSelectionPrompt_All verifies that "all" installs every installable hook.
func TestHookSelectionPrompt_All(t *testing.T) {
	names, err := embed.HookNames()
	if err != nil {
		t.Fatalf("embed.HookNames: %v", err)
	}
	var rows []string
	for _, n := range names {
		if isHookFile(n) || n == "patterns.json" {
			rows = append(rows, n)
		}
	}
	if len(rows) == 0 {
		t.Skip("no embedded hooks available")
	}

	var installed []string
	wireCalled := false
	var out bytes.Buffer
	input := strings.NewReader("all\n")

	err = runHookSelectionPrompt(
		&out, input, true,
		t.TempDir(),
		testHookInstallRecorder(&installed),
		testWireRecorder(&wireCalled),
		t.TempDir(),
	)
	if err != nil {
		t.Fatalf("runHookSelectionPrompt all: %v", err)
	}

	if len(installed) != len(rows) {
		t.Errorf("All: installed %d hooks, want %d; got %v", len(installed), len(rows), installed)
	}
	if !wireCalled {
		t.Error("All: wire function was not called")
	}
}

// TestHookSelectionPrompt_Skip verifies that pressing Enter (empty input) installs nothing.
func TestHookSelectionPrompt_Skip(t *testing.T) {
	var installed []string
	wireCalled := false
	var out bytes.Buffer
	input := strings.NewReader("\n") // empty — skip

	err := runHookSelectionPrompt(
		&out, input, true,
		t.TempDir(),
		testHookInstallRecorder(&installed),
		testWireRecorder(&wireCalled),
		t.TempDir(),
	)
	if err != nil {
		t.Fatalf("runHookSelectionPrompt skip: %v", err)
	}

	if len(installed) != 0 {
		t.Errorf("Skip: expected 0 installs, got %v", installed)
	}
	if wireCalled {
		t.Error("Skip: wire function should not be called when skipping")
	}
}

// TestHookSelectionPrompt_NonTTY verifies that isTTY=false is a no-op.
func TestHookSelectionPrompt_NonTTY(t *testing.T) {
	var installed []string
	wireCalled := false
	var out bytes.Buffer
	input := strings.NewReader("all\n")

	err := runHookSelectionPrompt(
		&out, input, false, // isTTY=false → non-interactive
		t.TempDir(),
		testHookInstallRecorder(&installed),
		testWireRecorder(&wireCalled),
		t.TempDir(),
	)
	if err != nil {
		t.Fatalf("runHookSelectionPrompt non-TTY: %v", err)
	}

	if len(installed) != 0 {
		t.Errorf("NonTTY: expected 0 installs, got %v", installed)
	}
	if wireCalled {
		t.Error("NonTTY: wire function should not be called in non-TTY mode")
	}
}

// TestHookSelectionPrompt_InvalidSelection verifies that out-of-range numbers
// emit a warning but do not cause an error, and valid selections still install.
func TestHookSelectionPrompt_InvalidSelection(t *testing.T) {
	names, err := embed.HookNames()
	if err != nil {
		t.Fatalf("embed.HookNames: %v", err)
	}
	var rows []string
	for _, n := range names {
		if isHookFile(n) || n == "patterns.json" {
			rows = append(rows, n)
		}
	}
	if len(rows) < 1 {
		t.Skip("need at least 1 embedded hook")
	}

	var installed []string
	wireCalled := false
	var out bytes.Buffer
	// "999" is out of range; "1" is valid.
	input := strings.NewReader("999,1\n")

	err = runHookSelectionPrompt(
		&out, input, true,
		t.TempDir(),
		testHookInstallRecorder(&installed),
		testWireRecorder(&wireCalled),
		t.TempDir(),
	)
	if err != nil {
		t.Fatalf("runHookSelectionPrompt invalid selection: %v", err)
	}

	// Warning should mention the invalid token.
	if !strings.Contains(out.String(), "Warning") && !strings.Contains(out.String(), "not a valid") {
		t.Errorf("InvalidSelection: expected warning in output, got: %q", out.String())
	}

	// Valid selection (item 1) should still install.
	if len(installed) != 1 {
		t.Errorf("InvalidSelection: expected 1 install (item 1), got %v", installed)
	}
	if installed[0] != rows[0] {
		t.Errorf("InvalidSelection: want %q installed, got %q", rows[0], installed[0])
	}
}
