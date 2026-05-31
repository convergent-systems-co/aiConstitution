package cmd_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/convergent-systems-co/aiConstitution/src/cmd/ai/cmd"
)

// ---------------------------------------------------------------------------
// purgeMalformedHookEntries — covers every malformed shape we have observed
// in the wild (see ~/.ai/audit/violations/2026-05-28T21-48-49Z-…).
// ---------------------------------------------------------------------------

func TestPurgeMalformed_RemovesFlatAbsolutePath(t *testing.T) {
	settings := map[string]any{
		"hooks": map[string]any{
			"PreToolUse": []any{
				// Pre-v1.3 absolute-path flat entry (Bug A + earlier absolute paths).
				map[string]any{"type": "PreToolUse", "command": "python3 /home/user/.ai/hooks/audit.py"},
				// Canonical group with a portable command.
				map[string]any{
					"hooks": []any{
						map[string]any{"type": "command", "command": "ai hooks run audit-logger"},
					},
				},
			},
		},
	}
	cmd.PurgeMalformedHookEntriesForTest(settings)

	entries := settings["hooks"].(map[string]any)["PreToolUse"].([]any)
	if len(entries) != 1 {
		t.Fatalf("expected 1 surviving entry, got %d: %v", len(entries), entries)
	}
	g := entries[0].(map[string]any)
	if _, ok := g["hooks"].([]any); !ok {
		t.Fatalf("surviving entry must be canonical group form; got %v", g)
	}
}

func TestPurgeMalformed_RemovesFlatPortableEntry(t *testing.T) {
	// Bug A: addClaudeEntry wrote flat {type: <eventName>, command: "ai hooks run …"}.
	// These must be dropped — the canonical writer re-emits them as proper groups.
	settings := map[string]any{
		"hooks": map[string]any{
			"PreToolUse": []any{
				map[string]any{"type": "PreToolUse", "command": "ai hooks run audit-logger"},
				map[string]any{"type": "PreToolUse", "command": "ai hooks run branch-guard"},
			},
		},
	}
	cmd.PurgeMalformedHookEntriesForTest(settings)

	entries := settings["hooks"].(map[string]any)["PreToolUse"].([]any)
	if len(entries) != 0 {
		t.Fatalf("expected all flat entries dropped; got %d: %v", len(entries), entries)
	}
}

func TestPurgeMalformed_RemovesNullHookStubs(t *testing.T) {
	// Bug B: updateSettingsJSON round-tripped flat entries through []hookGroup,
	// which serialized them back as {"hooks": null}. These must be dropped.
	settings := map[string]any{
		"hooks": map[string]any{
			"PreToolUse": []any{
				map[string]any{"hooks": nil},
				map[string]any{"hooks": []any{}},
				map[string]any{
					"hooks": []any{
						map[string]any{"type": "command", "command": "ai hooks run audit-logger"},
					},
				},
			},
		},
	}
	cmd.PurgeMalformedHookEntriesForTest(settings)

	entries := settings["hooks"].(map[string]any)["PreToolUse"].([]any)
	if len(entries) != 1 {
		t.Fatalf("expected only the canonical group to survive; got %d: %v", len(entries), entries)
	}
}

func TestPurgeMalformed_DropsBadNestedType(t *testing.T) {
	// A group with a nested entry whose type is not "command" is malformed —
	// the nested entry must be dropped. If no valid commands remain, the
	// whole group goes too.
	settings := map[string]any{
		"hooks": map[string]any{
			"PreToolUse": []any{
				map[string]any{
					"matcher": "Bash",
					"hooks": []any{
						map[string]any{"type": "PreToolUse", "command": "ai hooks run branch-guard"},
						map[string]any{"type": "command", "command": "ai hooks run audit-logger"},
					},
				},
			},
		},
	}
	cmd.PurgeMalformedHookEntriesForTest(settings)

	entries := settings["hooks"].(map[string]any)["PreToolUse"].([]any)
	if len(entries) != 1 {
		t.Fatalf("expected 1 group to survive; got %d: %v", len(entries), entries)
	}
	g := entries[0].(map[string]any)
	if g["matcher"] != "Bash" {
		t.Errorf("matcher lost: %v", g)
	}
	inner := g["hooks"].([]any)
	if len(inner) != 1 {
		t.Fatalf("expected only the canonical nested entry; got %d: %v", len(inner), inner)
	}
	h := inner[0].(map[string]any)
	if h["type"] != "command" || h["command"] != "ai hooks run audit-logger" {
		t.Errorf("nested entry wrong: %v", h)
	}
}

func TestPurgeMalformed_PreservesCanonicalGroups(t *testing.T) {
	// Canonical {matcher, hooks:[{type:"command", command}]} groups must survive
	// untouched and idempotently.
	settings := map[string]any{
		"hooks": map[string]any{
			"PreToolUse": []any{
				map[string]any{
					"hooks": []any{
						map[string]any{"type": "command", "command": "ai hooks run audit-logger"},
					},
				},
				map[string]any{
					"matcher": "Bash",
					"hooks": []any{
						map[string]any{"type": "command", "command": "ai hooks run branch-guard"},
					},
				},
			},
		},
	}
	cmd.PurgeMalformedHookEntriesForTest(settings)
	// Run twice to confirm idempotence.
	cmd.PurgeMalformedHookEntriesForTest(settings)

	entries := settings["hooks"].(map[string]any)["PreToolUse"].([]any)
	if len(entries) != 2 {
		t.Fatalf("expected both canonical groups preserved; got %d: %v", len(entries), entries)
	}
}

func TestPurgeMalformed_DropsRetiredCommands(t *testing.T) {
	// Retired wiring (catalog renamed audit→audit-logger; audit-command is a
	// wrapper post-hook, never a Claude event hook; checkpoint-tick is now
	// opt-in because it writes HANDOFF.md into working trees) must be scrubbed
	// even when it appears inside an otherwise-canonical group.
	settings := map[string]any{
		"hooks": map[string]any{
			"PreToolUse": []any{
				map[string]any{
					"hooks": []any{
						map[string]any{"type": "command", "command": "ai hooks run audit"},         // retired
						map[string]any{"type": "command", "command": "ai hooks run audit-command"}, // retired
						map[string]any{"type": "command", "command": "ai hooks run audit-logger"},  // keep
					},
				},
			},
			"Stop": []any{
				map[string]any{
					"hooks": []any{
						map[string]any{"type": "command", "command": "ai hooks run audit"}, // retired
						map[string]any{"type": "command", "command": "ai hooks run checkpoint-tick"},
					},
				},
			},
		},
	}
	cmd.PurgeMalformedHookEntriesForTest(settings)

	pre := settings["hooks"].(map[string]any)["PreToolUse"].([]any)
	if len(pre) != 1 {
		t.Fatalf("PreToolUse: expected 1 surviving group, got %d: %v", len(pre), pre)
	}
	inner := pre[0].(map[string]any)["hooks"].([]any)
	if len(inner) != 1 {
		t.Fatalf("PreToolUse[0].hooks: expected 1 surviving entry, got %d: %v", len(inner), inner)
	}
	if inner[0].(map[string]any)["command"] != "ai hooks run audit-logger" {
		t.Errorf("PreToolUse[0].hooks[0]: wrong survivor: %v", inner[0])
	}

	// Stop group had only retired entries — the whole group must be dropped.
	stop := settings["hooks"].(map[string]any)["Stop"].([]any)
	if len(stop) != 0 {
		t.Errorf("Stop: expected 0 groups (only retired entries), got %d: %v", len(stop), stop)
	}
}

func TestPurgeMalformed_EmptySettings(t *testing.T) {
	settings := map[string]any{}
	cmd.PurgeMalformedHookEntriesForTest(settings)
	if _, ok := settings["hooks"]; ok {
		t.Error("purge should not add a hooks key when none existed")
	}
}

// ---------------------------------------------------------------------------
// updateSettingsJSON — regression test against the exact corruption observed
// in the wild (see settings.json.broken).
// ---------------------------------------------------------------------------

// TestUpdateSettingsJSON_RecoversFromCorruptedFile drives the canonical writer
// against a settings.json containing every bad shape we saw on disk and
// asserts the result is a single clean set of canonical groups per event.
func TestUpdateSettingsJSON_RecoversFromCorruptedFile(t *testing.T) {
	tmp := t.TempDir()
	settingsPath := filepath.Join(tmp, "settings.json")

	// Synthesize the exact PreToolUse pathology: 1 valid all-tools group,
	// several {"hooks": null} stubs, 1 valid Bash group, more nulls, then
	// the flat {type: "PreToolUse", command} entries that addClaudeEntry wrote.
	corrupted := map[string]any{
		"model": "sonnet[1m]",
		"hooks": map[string]any{
			"PreToolUse": []any{
				map[string]any{
					"hooks": []any{
						map[string]any{"type": "command", "command": "ai hooks run audit-logger"},
					},
				},
				map[string]any{"hooks": nil},
				map[string]any{"hooks": nil},
				map[string]any{
					"matcher": "Bash",
					"hooks": []any{
						map[string]any{"type": "command", "command": "ai hooks run branch-guard"},
					},
				},
				map[string]any{"hooks": nil},
				map[string]any{"hooks": nil},
				map[string]any{"command": "ai hooks run audit-logger", "type": "PreToolUse"},
				map[string]any{"command": "ai hooks run branch-guard", "type": "PreToolUse"},
			},
		},
	}
	data, err := json.MarshalIndent(corrupted, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(settingsPath, data, 0o644); err != nil {
		t.Fatal(err)
	}

	if err := cmd.UpdateSettingsJSONForTest(settingsPath, ""); err != nil {
		t.Fatalf("updateSettingsJSON: %v", err)
	}

	out, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("result not valid JSON: %v\n%s", err, out)
	}

	// Unrelated keys must survive.
	if got["model"] != "sonnet[1m]" {
		t.Errorf("model key was clobbered: %v", got["model"])
	}

	hooks, ok := got["hooks"].(map[string]any)
	if !ok {
		t.Fatalf("hooks key missing or wrong type: %v", got["hooks"])
	}
	entries, ok := hooks["PreToolUse"].([]any)
	if !ok {
		t.Fatalf("PreToolUse missing or wrong type: %v", hooks["PreToolUse"])
	}

	// Expect exactly two groups: one matcher="" (all-tools), one matcher="Bash".
	// No nulls, no flat entries.
	if len(entries) != 2 {
		t.Fatalf("expected 2 canonical groups, got %d:\n%s", len(entries), out)
	}
	seenAllTools, seenBash := false, false
	for _, e := range entries {
		g, ok := e.(map[string]any)
		if !ok {
			t.Fatalf("entry not an object: %v", e)
		}
		if _, ok := g["hooks"].([]any); !ok {
			t.Fatalf("entry has no hooks array (malformed survivor): %v", g)
		}
		if _, hasType := g["type"]; hasType {
			t.Errorf("entry retains stray top-level type field: %v", g)
		}
		if _, hasCmd := g["command"]; hasCmd {
			t.Errorf("entry retains stray top-level command field: %v", g)
		}
		matcher, _ := g["matcher"].(string)
		if matcher == "Bash" {
			seenBash = true
		} else {
			seenAllTools = true
		}
	}
	if !seenAllTools {
		t.Errorf("expected all-tools group (matcher=\"\"): %s", out)
	}
	if !seenBash {
		t.Errorf("expected Bash matcher group: %s", out)
	}
}

// TestUpdateSettingsJSON_Idempotent verifies running the writer twice produces
// byte-identical output — the failure mode in the wild was that re-runs grew
// the file with {"hooks": null} stubs.
func TestUpdateSettingsJSON_Idempotent(t *testing.T) {
	tmp := t.TempDir()
	settingsPath := filepath.Join(tmp, "settings.json")

	if err := cmd.UpdateSettingsJSONForTest(settingsPath, ""); err != nil {
		t.Fatalf("first run: %v", err)
	}
	first, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.UpdateSettingsJSONForTest(settingsPath, ""); err != nil {
		t.Fatalf("second run: %v", err)
	}
	second, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) {
		t.Errorf("not idempotent:\nfirst:\n%s\nsecond:\n%s", first, second)
	}
}

// ---------------------------------------------------------------------------
// Schema validity — assert the written settings.json conforms to the Claude
// Code hook schema, independent of the canonical wiring content. This catches
// any future writer change that re-introduces the corruption shapes.
//
// Schema reference (Claude Code settings.json hooks section):
//
//	hooks: {
//	  <EventName>: [
//	    {
//	      matcher?: string,            // optional; absent or "" = all tools
//	      hooks: [                     // REQUIRED, non-empty array
//	        { type: "command",         // REQUIRED literal "command"
//	          command: string          // REQUIRED non-empty
//	        }, ...
//	      ]
//	    }, ...
//	  ]
//	}
//
// Allowed event names mirror Claude Code's documented hook events.
// ---------------------------------------------------------------------------

var allowedEventNames = map[string]bool{
	"PreToolUse":       true,
	"PostToolUse":      true,
	"UserPromptSubmit": true,
	"SessionStart":     true,
	"SessionEnd":       true,
	"SubagentStop":     true,
	"Stop":             true,
	"PreCompact":       true,
	"Notification":     true,
}

// validateSettingsJSONShape walks a parsed settings.json and reports every
// schema violation it finds. Returns nil only when the file is fully valid.
func validateSettingsJSONShape(t *testing.T, settings map[string]any) {
	t.Helper()
	hooksRaw, ok := settings["hooks"]
	if !ok {
		return // no hooks section is valid — empty config.
	}
	hooks, ok := hooksRaw.(map[string]any)
	if !ok {
		t.Errorf("settings.hooks must be an object, got %T: %v", hooksRaw, hooksRaw)
		return
	}
	for event, val := range hooks {
		if !allowedEventNames[event] {
			t.Errorf("hooks.%s: unknown event name (allowed: %v)", event, keys(allowedEventNames))
		}
		entries, ok := val.([]any)
		if !ok {
			t.Errorf("hooks.%s must be an array, got %T", event, val)
			continue
		}
		for i, entry := range entries {
			validateHookGroup(t, event, i, entry)
		}
	}
}

func validateHookGroup(t *testing.T, event string, idx int, entry any) {
	t.Helper()
	g, ok := entry.(map[string]any)
	if !ok {
		t.Errorf("hooks.%s[%d] must be an object, got %T: %v", event, idx, entry, entry)
		return
	}
	for k := range g {
		if k != "matcher" && k != "hooks" {
			t.Errorf("hooks.%s[%d] has unknown field %q (allowed: matcher, hooks)", event, idx, k)
		}
	}
	if m, present := g["matcher"]; present {
		if _, ok := m.(string); !ok {
			t.Errorf("hooks.%s[%d].matcher must be a string, got %T", event, idx, m)
		}
	}
	raw, ok := g["hooks"]
	if !ok {
		t.Errorf("hooks.%s[%d].hooks is required", event, idx)
		return
	}
	inner, ok := raw.([]any)
	if !ok {
		t.Errorf("hooks.%s[%d].hooks must be an array, got %T (value: %v)", event, idx, raw, raw)
		return
	}
	if len(inner) == 0 {
		t.Errorf("hooks.%s[%d].hooks must be non-empty", event, idx)
		return
	}
	for j, h := range inner {
		validateHookEntry(t, event, idx, j, h)
	}
}

func validateHookEntry(t *testing.T, event string, gi, hi int, h any) {
	t.Helper()
	m, ok := h.(map[string]any)
	if !ok {
		t.Errorf("hooks.%s[%d].hooks[%d] must be an object, got %T", event, gi, hi, h)
		return
	}
	for k := range m {
		if k != "type" && k != "command" {
			t.Errorf("hooks.%s[%d].hooks[%d] has unknown field %q (allowed: type, command)", event, gi, hi, k)
		}
	}
	typ, _ := m["type"].(string)
	if typ != "command" {
		t.Errorf("hooks.%s[%d].hooks[%d].type must be \"command\", got %q", event, gi, hi, typ)
	}
	cmd, _ := m["command"].(string)
	if strings.TrimSpace(cmd) == "" {
		t.Errorf("hooks.%s[%d].hooks[%d].command must be non-empty", event, gi, hi)
	}
}

func keys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func readAndValidate(t *testing.T, settingsPath string) {
	t.Helper()
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("read settings.json: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("settings.json is not valid JSON: %v\n%s", err, data)
	}
	validateSettingsJSONShape(t, got)
}

// TestSettingsJSON_ValidOnFreshWrite verifies the canonical writer produces a
// schema-valid file when starting from nothing.
func TestSettingsJSON_ValidOnFreshWrite(t *testing.T) {
	tmp := t.TempDir()
	settingsPath := filepath.Join(tmp, "settings.json")

	if err := cmd.UpdateSettingsJSONForTest(settingsPath, ""); err != nil {
		t.Fatalf("write: %v", err)
	}
	readAndValidate(t, settingsPath)
}

// TestSettingsJSON_ValidAfterRepairingCorruption verifies the writer recovers
// from every malformed shape we have seen in the wild and emits a schema-valid
// result. Drives a representative slice of the on-disk corruption from
// settings.json.broken (the snapshot in ~/.ai/audit/violations/).
func TestSettingsJSON_ValidAfterRepairingCorruption(t *testing.T) {
	tmp := t.TempDir()
	settingsPath := filepath.Join(tmp, "settings.json")

	corrupted := map[string]any{
		"hooks": map[string]any{
			"PreToolUse": []any{
				// Canonical groups that should be preserved.
				map[string]any{
					"hooks": []any{
						map[string]any{"type": "command", "command": "ai hooks run audit-logger"},
					},
				},
				map[string]any{
					"matcher": "Bash",
					"hooks": []any{
						map[string]any{"type": "command", "command": "ai hooks run branch-guard"},
					},
				},
				// Null stubs (Bug B).
				map[string]any{"hooks": nil},
				map[string]any{"hooks": []any{}},
				// Flat wrong-shape entries (Bug A).
				map[string]any{"command": "ai hooks run audit-logger", "type": "PreToolUse"},
				map[string]any{"command": "ai hooks run worktree-guard", "type": "PreToolUse"},
				// Pre-v1.3 absolute-path entry (both flat and embedded in a group).
				map[string]any{"command": "python3 /Users/x/.ai/hooks/audit.py", "type": "PreToolUse"},
				map[string]any{
					"hooks": []any{
						map[string]any{"type": "command", "command": "/Users/x/.ai/hooks/secret-block.py"},
					},
				},
			},
			"Stop": []any{
				// Mixed: one canonical, one null, one flat.
				map[string]any{
					"hooks": []any{
						map[string]any{"type": "command", "command": "ai hooks run audit-logger"},
					},
				},
				map[string]any{"hooks": nil},
				map[string]any{"command": "ai hooks run checkpoint-tick", "type": "Stop"},
			},
		},
	}
	data, err := json.MarshalIndent(corrupted, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(settingsPath, data, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := cmd.UpdateSettingsJSONForTest(settingsPath, ""); err != nil {
		t.Fatalf("repair: %v", err)
	}
	readAndValidate(t, settingsPath)

	out, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(out), "ai hooks run checkpoint-tick") {
		t.Errorf("settings.json should scrub retired checkpoint-tick wiring:\n%s", out)
	}
}

// TestSettingsJSON_RepeatedWritesStayValid verifies the writer remains valid
// under many repeated invocations — the file must not grow or degrade.
func TestSettingsJSON_RepeatedWritesStayValid(t *testing.T) {
	tmp := t.TempDir()
	settingsPath := filepath.Join(tmp, "settings.json")

	for i := 0; i < 10; i++ {
		if err := cmd.UpdateSettingsJSONForTest(settingsPath, ""); err != nil {
			t.Fatalf("iteration %d: %v", i, err)
		}
		readAndValidate(t, settingsPath)
	}
}

// TestSettingsJSON_NoNullHooks proves the writer never emits {"hooks": null}
// stubs even when the input file is dense with them.
func TestSettingsJSON_NoNullHooks(t *testing.T) {
	tmp := t.TempDir()
	settingsPath := filepath.Join(tmp, "settings.json")

	// Seed with 50 null stubs under PreToolUse — the exact failure mode.
	preToolUse := make([]any, 0, 50)
	for i := 0; i < 50; i++ {
		preToolUse = append(preToolUse, map[string]any{"hooks": nil})
	}
	seed := map[string]any{
		"hooks": map[string]any{"PreToolUse": preToolUse},
	}
	data, _ := json.MarshalIndent(seed, "", "  ")
	if err := os.WriteFile(settingsPath, data, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := cmd.UpdateSettingsJSONForTest(settingsPath, ""); err != nil {
		t.Fatalf("repair: %v", err)
	}

	out, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(out), "\"hooks\": null") {
		t.Errorf("settings.json contains \"hooks\": null after repair:\n%s", out)
	}
	readAndValidate(t, settingsPath)
}
