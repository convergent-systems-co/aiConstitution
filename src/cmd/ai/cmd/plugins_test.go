package cmd_test

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/convergent-systems-co/aiConstitution/src/cmd/ai/cmd"
)

// makePluginTarGz creates a .tar.gz archive in destDir containing the
// given manifest.yaml content at <pluginName>/manifest.yaml.
// Returns the path to the created archive.
func makePluginTarGz(t *testing.T, destDir, pluginName, manifestContent string) string {
	t.Helper()
	archivePath := filepath.Join(destDir, pluginName+".tar.gz")
	f, err := os.Create(archivePath)
	if err != nil {
		t.Fatalf("create archive: %v", err)
	}
	defer f.Close()

	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)

	entryPath := pluginName + "/manifest.yaml"
	body := []byte(manifestContent)
	hdr := &tar.Header{
		Name: entryPath,
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
	if err := gz.Close(); err != nil {
		t.Fatalf("gzip close: %v", err)
	}
	return archivePath
}

// TestPluginsInstall_UnpacksTarGzToPluginsDir verifies that
// `ai plugins install <path>` unpacks the archive to ~/.ai/plugins/<name>/.
func TestPluginsInstall_UnpacksTarGzToPluginsDir(t *testing.T) {
	workDir := t.TempDir()
	pluginsDir := filepath.Join(workDir, "plugins")
	configDir := filepath.Join(workDir, "config")
	t.Setenv("AI_ROOT", workDir)
	t.Setenv("AICONST_CONFIG_DIR", configDir)

	manifest := `name: test-plugin
version: "1.0.0"
description: "A test plugin"
source: ""
`
	archivePath := makePluginTarGz(t, workDir, "test-plugin", manifest)

	root := cmd.NewRootCmd()
	root.SetArgs([]string{"plugins", "install", archivePath})
	out := &strings.Builder{}
	root.SetOut(out)

	if err := root.Execute(); err != nil {
		t.Fatalf("plugins install returned error: %v", err)
	}

	manifestPath := filepath.Join(pluginsDir, "test-plugin", "manifest.yaml")
	if _, err := os.Stat(manifestPath); err != nil {
		t.Errorf("manifest not found at %s: %v", manifestPath, err)
	}

	output := out.String()
	if !strings.Contains(output, "Installed test-plugin") {
		t.Errorf("expected 'Installed test-plugin' in output, got: %q", output)
	}
	if !strings.Contains(output, "1.0.0") {
		t.Errorf("expected version '1.0.0' in output, got: %q", output)
	}
}

// TestPluginsInstall_ErrorsWhenNoManifest verifies that install fails if
// the tar.gz does not contain a manifest.yaml at the expected path.
func TestPluginsInstall_ErrorsWhenNoManifest(t *testing.T) {
	workDir := t.TempDir()
	t.Setenv("AI_ROOT", workDir)
	t.Setenv("AICONST_CONFIG_DIR", filepath.Join(workDir, "config"))

	// Archive with no manifest.yaml
	archivePath := filepath.Join(workDir, "bad-plugin.tar.gz")
	f, _ := os.Create(archivePath)
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	_ = tw.Close()
	_ = gz.Close()
	_ = f.Close()

	root := cmd.NewRootCmd()
	root.SetArgs([]string{"plugins", "install", archivePath})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error when manifest.yaml is missing, got nil")
	}
}

// TestPluginsInstall_Force verifies that --force reinstalls an existing plugin.
func TestPluginsInstall_Force(t *testing.T) {
	workDir := t.TempDir()
	t.Setenv("AI_ROOT", workDir)
	t.Setenv("AICONST_CONFIG_DIR", filepath.Join(workDir, "config"))

	manifest := `name: force-plugin
version: "1.0.0"
description: "force test"
`
	archivePath := makePluginTarGz(t, workDir, "force-plugin", manifest)

	// First install
	root := cmd.NewRootCmd()
	root.SetArgs([]string{"plugins", "install", archivePath})
	if err := root.Execute(); err != nil {
		t.Fatalf("first install: %v", err)
	}

	// Second install without --force should error
	root2 := cmd.NewRootCmd()
	root2.SetArgs([]string{"plugins", "install", archivePath})
	if err := root2.Execute(); err == nil {
		t.Fatal("expected error on reinstall without --force")
	}

	// Third install with --force should succeed
	root3 := cmd.NewRootCmd()
	root3.SetArgs([]string{"plugins", "install", "--force", archivePath})
	out := &strings.Builder{}
	root3.SetOut(out)
	if err := root3.Execute(); err != nil {
		t.Fatalf("--force reinstall: %v", err)
	}
}

// TestPluginsEnable_AddsToPluginsJSON verifies that `ai plugins enable <name>`
// adds the plugin name to plugins.json and errors if the plugin is not installed.
func TestPluginsEnable_AddsToPluginsJSON(t *testing.T) {
	workDir := t.TempDir()
	pluginsDir := filepath.Join(workDir, "plugins")
	configDir := filepath.Join(workDir, "config")
	t.Setenv("AI_ROOT", workDir)
	t.Setenv("AICONST_CONFIG_DIR", configDir)

	// Pre-create the plugin dir to simulate an installed plugin.
	if err := os.MkdirAll(filepath.Join(pluginsDir, "my-plugin"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pluginsDir, "my-plugin", "manifest.yaml"),
		[]byte("name: my-plugin\nversion: \"1.0.0\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	root := cmd.NewRootCmd()
	root.SetArgs([]string{"plugins", "enable", "my-plugin"})
	if err := root.Execute(); err != nil {
		t.Fatalf("plugins enable: %v", err)
	}

	// Verify plugins.json was written and contains the name.
	stateFile := filepath.Join(configDir, "plugins.json")
	data, err := os.ReadFile(stateFile)
	if err != nil {
		t.Fatalf("read plugins.json: %v", err)
	}
	var state struct {
		Enabled []string `json:"enabled"`
	}
	if err := json.Unmarshal(data, &state); err != nil {
		t.Fatalf("unmarshal plugins.json: %v", err)
	}
	found := false
	for _, n := range state.Enabled {
		if n == "my-plugin" {
			found = true
		}
	}
	if !found {
		t.Errorf("my-plugin not found in enabled list: %v", state.Enabled)
	}
}

// TestPluginsEnable_ErrorsWhenNotInstalled verifies that enable returns
// an error if the named plugin directory does not exist.
func TestPluginsEnable_ErrorsWhenNotInstalled(t *testing.T) {
	workDir := t.TempDir()
	t.Setenv("AI_ROOT", workDir)
	t.Setenv("AICONST_CONFIG_DIR", filepath.Join(workDir, "config"))

	root := cmd.NewRootCmd()
	root.SetArgs([]string{"plugins", "enable", "nonexistent-plugin"})
	if err := root.Execute(); err == nil {
		t.Fatal("expected error enabling uninstalled plugin")
	}
}

// TestPluginsDisable_RemovesFromPluginsJSON verifies that `ai plugins disable`
// removes a plugin from the enabled list in plugins.json.
func TestPluginsDisable_RemovesFromPluginsJSON(t *testing.T) {
	workDir := t.TempDir()
	configDir := filepath.Join(workDir, "config")
	t.Setenv("AI_ROOT", workDir)
	t.Setenv("AICONST_CONFIG_DIR", configDir)

	// Pre-write plugins.json with the plugin enabled.
	if err := os.MkdirAll(configDir, 0o750); err != nil {
		t.Fatal(err)
	}
	initial := `{"enabled":["my-plugin","other-plugin"]}`
	if err := os.WriteFile(filepath.Join(configDir, "plugins.json"), []byte(initial), 0o644); err != nil {
		t.Fatal(err)
	}

	root := cmd.NewRootCmd()
	root.SetArgs([]string{"plugins", "disable", "my-plugin"})
	if err := root.Execute(); err != nil {
		t.Fatalf("plugins disable: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(configDir, "plugins.json"))
	var state struct {
		Enabled []string `json:"enabled"`
	}
	if err := json.Unmarshal(data, &state); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	for _, n := range state.Enabled {
		if n == "my-plugin" {
			t.Error("my-plugin should have been removed from enabled list")
		}
	}
	// other-plugin should still be present
	found := false
	for _, n := range state.Enabled {
		if n == "other-plugin" {
			found = true
		}
	}
	if !found {
		t.Error("other-plugin was accidentally removed")
	}
}

// TestPluginsDisable_ErrorsWhenNotEnabled verifies that disable returns
// an error if the named plugin is not in the enabled list.
func TestPluginsDisable_ErrorsWhenNotEnabled(t *testing.T) {
	workDir := t.TempDir()
	configDir := filepath.Join(workDir, "config")
	t.Setenv("AI_ROOT", workDir)
	t.Setenv("AICONST_CONFIG_DIR", configDir)

	if err := os.MkdirAll(configDir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "plugins.json"),
		[]byte(`{"enabled":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}

	root := cmd.NewRootCmd()
	root.SetArgs([]string{"plugins", "disable", "missing-plugin"})
	if err := root.Execute(); err == nil {
		t.Fatal("expected error disabling plugin not in enabled list")
	}
}

// TestPluginsStatus_ShowsInstalledPlugins verifies that `ai plugins status`
// lists all installed plugins with their enabled/disabled marker.
func TestPluginsStatus_ShowsInstalledPlugins(t *testing.T) {
	workDir := t.TempDir()
	pluginsDir := filepath.Join(workDir, "plugins")
	configDir := filepath.Join(workDir, "config")
	t.Setenv("AI_ROOT", workDir)
	t.Setenv("AICONST_CONFIG_DIR", configDir)

	// Install two plugins.
	for _, name := range []string{"alpha-plugin", "beta-plugin"} {
		dir := filepath.Join(pluginsDir, name)
		if err := os.MkdirAll(dir, 0o750); err != nil {
			t.Fatal(err)
		}
		content := "name: " + name + "\nversion: \"2.0.0\"\n"
		if err := os.WriteFile(filepath.Join(dir, "manifest.yaml"), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	// Enable only alpha-plugin.
	if err := os.MkdirAll(configDir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "plugins.json"),
		[]byte(`{"enabled":["alpha-plugin"]}`), 0o644); err != nil {
		t.Fatal(err)
	}

	root := cmd.NewRootCmd()
	root.SetArgs([]string{"plugins", "status"})
	out := &strings.Builder{}
	root.SetOut(out)
	if err := root.Execute(); err != nil {
		t.Fatalf("plugins status: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "alpha-plugin") {
		t.Errorf("expected alpha-plugin in status output, got: %q", output)
	}
	if !strings.Contains(output, "beta-plugin") {
		t.Errorf("expected beta-plugin in status output, got: %q", output)
	}
	// alpha-plugin should show as enabled
	if !strings.Contains(output, "enabled") {
		t.Errorf("expected 'enabled' marker in output, got: %q", output)
	}
}

// TestPluginsStatus_EmptyWhenNoPlugins verifies that `ai plugins status`
// prints the "(no plugins installed)" message when no plugins exist.
func TestPluginsStatus_EmptyWhenNoPlugins(t *testing.T) {
	workDir := t.TempDir()
	t.Setenv("AI_ROOT", workDir)
	t.Setenv("AICONST_CONFIG_DIR", filepath.Join(workDir, "config"))

	root := cmd.NewRootCmd()
	root.SetArgs([]string{"plugins", "status"})
	out := &strings.Builder{}
	root.SetOut(out)
	if err := root.Execute(); err != nil {
		t.Fatalf("plugins status: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "no plugins installed") {
		t.Errorf("expected 'no plugins installed' in output, got: %q", output)
	}
}

// TestPluginsUpdate_ReinstallsFromSource verifies that `ai plugins update`
// re-downloads and installs from the source in manifest.yaml, printing
// the old→new version transition.
func TestPluginsUpdate_ReinstallsFromSource(t *testing.T) {
	workDir := t.TempDir()
	pluginsDir := filepath.Join(workDir, "plugins")
	t.Setenv("AI_ROOT", workDir)
	t.Setenv("AICONST_CONFIG_DIR", filepath.Join(workDir, "config"))

	// Simulate an installed v1.0.0 plugin; source points to a local path.
	newArchive := makePluginTarGz(t, workDir, "update-plugin", `name: update-plugin
version: "2.0.0"
description: "updated plugin"
`)
	oldManifest := `name: update-plugin
version: "1.0.0"
description: "old plugin"
source: "` + newArchive + `"
`
	pluginDir := filepath.Join(pluginsDir, "update-plugin")
	if err := os.MkdirAll(pluginDir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pluginDir, "manifest.yaml"), []byte(oldManifest), 0o644); err != nil {
		t.Fatal(err)
	}

	root := cmd.NewRootCmd()
	root.SetArgs([]string{"plugins", "update", "update-plugin"})
	out := &strings.Builder{}
	root.SetOut(out)
	if err := root.Execute(); err != nil {
		t.Fatalf("plugins update: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "Updated update-plugin") {
		t.Errorf("expected 'Updated update-plugin' in output, got: %q", output)
	}
	if !strings.Contains(output, "1.0.0") {
		t.Errorf("expected old version '1.0.0' in output, got: %q", output)
	}
	if !strings.Contains(output, "2.0.0") {
		t.Errorf("expected new version '2.0.0' in output, got: %q", output)
	}
}

// TestPluginsUpdate_ErrorsWhenNotInstalled verifies that update returns
// an error when the plugin is not installed.
func TestPluginsUpdate_ErrorsWhenNotInstalled(t *testing.T) {
	workDir := t.TempDir()
	t.Setenv("AI_ROOT", workDir)
	t.Setenv("AICONST_CONFIG_DIR", filepath.Join(workDir, "config"))

	root := cmd.NewRootCmd()
	root.SetArgs([]string{"plugins", "update", "ghost-plugin"})
	if err := root.Execute(); err == nil {
		t.Fatal("expected error updating uninstalled plugin")
	}
}

// TestPluginsList_NoPlugins verifies that `ai plugins list` prints
// "(no plugins installed)" and exits 0 when no plugins are installed.
func TestPluginsList_NoPlugins(t *testing.T) {
	workDir := t.TempDir()
	t.Setenv("AI_ROOT", workDir)
	t.Setenv("AICONST_CONFIG_DIR", filepath.Join(workDir, "config"))

	root := cmd.NewRootCmd()
	root.SetArgs([]string{"plugins", "list"})
	out := &strings.Builder{}
	root.SetOut(out)

	if err := root.Execute(); err != nil {
		t.Fatalf("plugins list returned error: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "no plugins installed") {
		t.Errorf("expected 'no plugins installed' in output, got: %q", output)
	}
}

// TestPluginsList_OneEnabled verifies that `ai plugins list` outputs a
// tabwriter table with the correct columns when one plugin is installed
// and enabled. The STATUS column must show "enabled".
func TestPluginsList_OneEnabled(t *testing.T) {
	workDir := t.TempDir()
	pluginsDir := filepath.Join(workDir, "plugins")
	configDir := filepath.Join(workDir, "config")
	t.Setenv("AI_ROOT", workDir)
	t.Setenv("AICONST_CONFIG_DIR", configDir)

	// Install one plugin.
	pluginDir := filepath.Join(pluginsDir, "alpha-plugin")
	if err := os.MkdirAll(pluginDir, 0o750); err != nil {
		t.Fatal(err)
	}
	manifest := "name: alpha-plugin\nversion: \"1.2.3\"\nsource: \"https://example.com/alpha.tar.gz\"\n"
	if err := os.WriteFile(filepath.Join(pluginDir, "manifest.yaml"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}

	// Mark it enabled.
	if err := os.MkdirAll(configDir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "plugins.json"),
		[]byte(`{"enabled":["alpha-plugin"]}`), 0o644); err != nil {
		t.Fatal(err)
	}

	root := cmd.NewRootCmd()
	root.SetArgs([]string{"plugins", "list"})
	out := &strings.Builder{}
	root.SetOut(out)

	if err := root.Execute(); err != nil {
		t.Fatalf("plugins list returned error: %v", err)
	}

	output := out.String()

	// Headers must be present.
	if !strings.Contains(output, "NAME") {
		t.Errorf("expected column header 'NAME' in output, got: %q", output)
	}
	if !strings.Contains(output, "VERSION") {
		t.Errorf("expected column header 'VERSION' in output, got: %q", output)
	}
	if !strings.Contains(output, "STATUS") {
		t.Errorf("expected column header 'STATUS' in output, got: %q", output)
	}
	if !strings.Contains(output, "SOURCE") {
		t.Errorf("expected column header 'SOURCE' in output, got: %q", output)
	}

	// Plugin data must appear.
	if !strings.Contains(output, "alpha-plugin") {
		t.Errorf("expected plugin name 'alpha-plugin' in output, got: %q", output)
	}
	if !strings.Contains(output, "1.2.3") {
		t.Errorf("expected version '1.2.3' in output, got: %q", output)
	}
	if !strings.Contains(output, "enabled") {
		t.Errorf("expected status 'enabled' in output, got: %q", output)
	}
	if !strings.Contains(output, "https://example.com/alpha.tar.gz") {
		t.Errorf("expected source URL in output, got: %q", output)
	}
}

// TestPluginsList_OneDisabled verifies that `ai plugins list` shows
// "disabled" in the STATUS column for a plugin that is installed but
// not in the enabled list.
func TestPluginsList_OneDisabled(t *testing.T) {
	workDir := t.TempDir()
	pluginsDir := filepath.Join(workDir, "plugins")
	configDir := filepath.Join(workDir, "config")
	t.Setenv("AI_ROOT", workDir)
	t.Setenv("AICONST_CONFIG_DIR", configDir)

	// Install one plugin.
	pluginDir := filepath.Join(pluginsDir, "beta-plugin")
	if err := os.MkdirAll(pluginDir, 0o750); err != nil {
		t.Fatal(err)
	}
	manifest := "name: beta-plugin\nversion: \"0.9.0\"\nsource: \"\"\n"
	if err := os.WriteFile(filepath.Join(pluginDir, "manifest.yaml"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}

	// No enabled plugins — empty list.
	if err := os.MkdirAll(configDir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "plugins.json"),
		[]byte(`{"enabled":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}

	root := cmd.NewRootCmd()
	root.SetArgs([]string{"plugins", "list"})
	out := &strings.Builder{}
	root.SetOut(out)

	if err := root.Execute(); err != nil {
		t.Fatalf("plugins list returned error: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "beta-plugin") {
		t.Errorf("expected plugin name 'beta-plugin' in output, got: %q", output)
	}
	if !strings.Contains(output, "0.9.0") {
		t.Errorf("expected version '0.9.0' in output, got: %q", output)
	}
	if !strings.Contains(output, "disabled") {
		t.Errorf("expected status 'disabled' in output, got: %q", output)
	}
}
