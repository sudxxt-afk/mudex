package ui

import (
	"testing"

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
