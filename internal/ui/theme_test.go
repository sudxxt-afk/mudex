package ui

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAllThemes(t *testing.T) {
	themes := AllThemes()
	if len(themes) != 6 {
		t.Fatalf("expected 6 themes, got %d", len(themes))
	}
	want := []string{"neon", "lamp", "minimal", "phosphor", "paper", "midnight"}
	seen := map[string]bool{}
	for i, th := range themes {
		if th.ID != want[i] {
			t.Fatalf("theme %d: expected id %q, got %q", i, want[i], th.ID)
		}
		if seen[th.ID] {
			t.Fatalf("duplicate theme id %q", th.ID)
		}
		seen[th.ID] = true
	}
}

func TestThemeByIDUnknown(t *testing.T) {
	th := ThemeByID("nope")
	if th.ID != "neon" {
		t.Fatalf("expected neon for unknown id, got %q", th.ID)
	}
}

func TestSaveLoadRoundtrip(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	// os.UserHomeDir respects $HOME on linux; ensure XDG_CONFIG_HOME does not interfere is N/A here.
	th := ThemeByID("paper")
	if err := th.Save(); err != nil {
		t.Fatalf("Save failed: %v", err)
	}
	got := LoadTheme()
	if got.ID != "paper" {
		t.Fatalf("expected paper after roundtrip, got %q", got.ID)
	}
}

func TestLoadBrokenJSON(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	p := filepath.Join(dir, ".config", "mudex", "config.json")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := LoadTheme()
	if got.ID != "neon" {
		t.Fatalf("expected neon for broken JSON, got %q", got.ID)
	}
}
