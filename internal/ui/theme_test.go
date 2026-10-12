package ui

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
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

func hexLuminance(t *testing.T, hex string) float64 {
	t.Helper()
	var r, g, b int
	if _, err := fmt.Sscanf(strings.ToLower(hex), "#%02x%02x%02x", &r, &g, &b); err != nil {
		t.Fatalf("bad hex %q", hex)
	}
	l := func(v int) float64 {
		f := float64(v) / 255
		if f <= 0.03928 {
			return f / 12.92
		}
		return math.Pow((f+0.055)/1.055, 2.4)
	}
	return 0.2126*l(r) + 0.7152*l(g) + 0.0722*l(b)
}

func contrast(a, b float64) float64 {
	if a < b {
		a, b = b, a
	}
	return (a + 0.05) / (b + 0.05)
}

// TestThemeContrast guards palettes against eye-searing combos:
// body text must read well on the theme background.
func TestThemeContrast(t *testing.T) {
	for _, th := range AllThemes() {
		if th.AppBG == "" {
			continue // transparent: terminal default unknown
		}
		bg := hexLuminance(t, th.AppBG)
		if c := contrast(hexLuminance(t, th.Text), bg); c < 4.5 {
			t.Errorf("%s: Text/BG contrast %.2f < 4.5", th.ID, c)
		}
		if c := contrast(hexLuminance(t, th.Title), bg); c < 3.0 {
			t.Errorf("%s: Title/BG contrast %.2f < 3.0", th.ID, c)
		}
		if c := contrast(hexLuminance(t, th.Dim), bg); c < 1.8 {
			t.Errorf("%s: Dim/BG contrast %.2f < 1.8", th.ID, c)
		}
	}
}
