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
