package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/convergent-systems-co/aiConstitution/src/internal/config"
)

func TestLoadReturnsDefaultsWhenFileAbsent(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("AICONST_CONFIG_DIR", tmp)
	got, err := config.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	want := config.Defaults()
	if got.SchemaVersion != want.SchemaVersion {
		t.Errorf("SchemaVersion = %q, want %q", got.SchemaVersion, want.SchemaVersion)
	}
}

func TestLoadParsesToml(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("AICONST_CONFIG_DIR", tmp)

	tomlContent := `schemaVersion = "0.3"

[review]
cadenceDays = 14
includeMemory = false
`
	if err := os.WriteFile(filepath.Join(tmp, "settings.toml"), []byte(tomlContent), 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := config.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got.SchemaVersion != "0.3" {
		t.Errorf("SchemaVersion = %q, want %q", got.SchemaVersion, "0.3")
	}
	if got.Review.CadenceDays != 14 {
		t.Errorf("Review.CadenceDays = %d, want %d", got.Review.CadenceDays, 14)
	}
	if got.Review.IncludeMemory != false {
		t.Error("Review.IncludeMemory = true, want false")
	}
}

func TestLoadEnvVarOverride(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("AICONST_CONFIG_DIR", tmp)
	t.Setenv("AICONST_REVIEW_CADENCE_DAYS", "60")

	got, err := config.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got.Review.CadenceDays != 60 {
		t.Errorf("Review.CadenceDays = %d, want 60 (from env var)", got.Review.CadenceDays)
	}
}

func TestSaveRoundTrips(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("AICONST_CONFIG_DIR", tmp)

	want := config.Defaults()
	want.Review.CadenceDays = 45
	want.SchemaVersion = "0.3"

	if err := config.Save(want); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	got, err := config.Load()
	if err != nil {
		t.Fatalf("Load() after Save() error = %v", err)
	}
	if got.Review.CadenceDays != 45 {
		t.Errorf("Review.CadenceDays = %d, want 45", got.Review.CadenceDays)
	}
}
