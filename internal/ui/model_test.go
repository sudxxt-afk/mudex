package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"tui-player/internal/cover"
	"tui-player/internal/library"
)

func TestModelNavAndView(t *testing.T) {
	tracks := []library.Track{
		{Path: "/m/a.mp3", Name: "alpha", Ext: ".mp3"},
		{Path: "/m/b.flac", Name: "beta", Ext: ".flac"},
		{Path: "/m/c.wav", Name: "gamma", Ext: ".wav"},
	}
	m := New("/m", tracks)
	if len(m.filtered) != 3 {
		t.Fatal("filtered init wrong")
	}
	// simulate down key via Update with key msg? Use View smoke test instead.
	out := m.View()
	if out == "" {
		t.Fatal("empty view")
	}
	m.cursor = 1
	m.playing = 0
	m.refreshCover()
	out = m.View()
	if len(out) < 100 {
		t.Fatalf("view too short: %d", len(out))
	}
	// filter
	m.search.SetValue("alp")
	m.applyFilter()
	if len(m.filtered) != 1 || m.tracks[m.filtered[0]].Name != "alpha" {
		t.Fatalf("filter wrong: %+v", m.filtered)
	}
}

func TestAllThemesRender(t *testing.T) {
	tracks := []library.Track{
		{Path: "/m/a.mp3", Name: "alpha", Ext: ".mp3", Artist: "Someone"},
	}
	for _, th := range AllThemes() {
		m := New("/m", tracks)
		m.theme = th
		m.playing = 0
		m.refreshCover()
		out := m.View()
		if len(out) < 100 {
			t.Fatalf("theme %s view too short", th.ID)
		}
		for _, row := range m.vizRows(40) {
			if row == "" {
				t.Fatalf("theme %s produced empty viz row", th.ID)
			}
		}
	}
}

func TestCycleThemeWraps(t *testing.T) {
	t.Setenv("HOME", t.TempDir()) // cycleTheme persists; keep off real config
	m := New("/m", nil)
	all := AllThemes()
	m.theme = all[0]
	m.cycleTheme(-1)
	if m.theme.ID != all[len(all)-1].ID {
		t.Fatalf("backward wrap wrong: %s", m.theme.ID)
	}
	m.cycleTheme(1)
	if m.theme.ID != all[0].ID {
		t.Fatalf("forward wrap wrong: %s", m.theme.ID)
	}
}

func TestMenuToggleAndNavigate(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	m := New("/m", []library.Track{{Path: "/m/a.mp3", Name: "a", Ext: ".mp3"}})
	if m.menuOpen {
		t.Fatal("menu must start closed")
	}
	um, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlP})
	m = um.(Model)
	if !m.menuOpen {
		t.Fatal("ctrl+p must open menu")
	}
	um, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m = um.(Model)
	if m.menuCursor != 1 {
		t.Fatalf("cursor want 1 got %d", m.menuCursor)
	}
	um, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = um.(Model)
	if m.menuOpen {
		t.Fatal("esc must close menu")
	}
	out := m.View()
	if out == "" {
		t.Fatal("empty view")
	}
	m.menuOpen = true
	if out := m.View(); !strings.Contains(out, "Settings") {
		t.Fatal("menu view must contain Settings")
	}
}

func TestMenuStepPersists(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	m := New("/m", nil)
	m.menuCursor = 1 // shuffle
	m.menuStep(1)
	if !m.shuffle {
		t.Fatal("shuffle must toggle on")
	}
	if got := LoadConfig(); !got.Shuffle {
		t.Fatal("shuffle must persist")
	}
	m.menuCursor = 3 // visualizer
	m.menuStep(1)
	if m.vizMode != "bars" {
		t.Fatalf("viz want bars got %s", m.vizMode)
	}
	m.menuCursor = 0 // theme
	first := m.theme.ID
	m.menuStep(1)
	if m.theme.ID == first {
		t.Fatal("theme must change")
	}
	if got := LoadConfig(); got.Theme != m.theme.ID || got.Viz != "bars" {
		t.Fatalf("config not persisted: %+v", got)
	}
}

func TestMenuCoverMode(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	defer cover.SetRenderMode("auto")
	m := New("/m", nil)
	m.menuCursor = 4
	m.menuStep(1)
	if m.coverMode != "sextant" {
		t.Fatalf("cover want sextant got %s", m.coverMode)
	}
	m.menuStep(1)
	if m.coverMode != "half" {
		t.Fatalf("cover want half got %s", m.coverMode)
	}
}
