package ui

import (
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"tui-player/internal/cover"
)

// mkMusicDir builds: root/{a.mp3, b.flac, sub/c.wav}. Files are empty;
// listing/probing tolerates that (duration unknown).
func mkMusicDir(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, f := range []string{"a.mp3", "b.flac"} {
		if err := os.WriteFile(filepath.Join(root, f), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	sub := filepath.Join(root, "sub")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "c.wav"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func writeToneWAV(t *testing.T, path string) {
	t.Helper()
	sr, n := 44100, 4410 // 0.1s stereo 16-bit
	data := make([]byte, 44+n*4)
	copy(data[0:], []byte("RIFF"))
	binary.LittleEndian.PutUint32(data[4:], uint32(36+n*4))
	copy(data[8:], []byte("WAVEfmt "))
	binary.LittleEndian.PutUint32(data[16:], 16)
	binary.LittleEndian.PutUint16(data[20:], 1)
	binary.LittleEndian.PutUint16(data[22:], 2)
	binary.LittleEndian.PutUint32(data[24:], uint32(sr))
	binary.LittleEndian.PutUint32(data[28:], uint32(sr*4))
	binary.LittleEndian.PutUint16(data[32:], 4)
	binary.LittleEndian.PutUint16(data[34:], 16)
	copy(data[36:], []byte("data"))
	binary.LittleEndian.PutUint32(data[40:], uint32(n*4))
	for i := 0; i < n; i++ {
		v := int16(math.Sin(2*math.Pi*440*float64(i)/float64(sr)) * 8000)
		binary.LittleEndian.PutUint16(data[44+i*4:], uint16(v))
		binary.LittleEndian.PutUint16(data[44+i*4+2:], uint16(v))
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestBrowseListsDirsFirst(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	m := New(mkMusicDir(t))
	if len(m.entries) != 3 {
		t.Fatalf("want 3 entries, got %+v", m.entries)
	}
	if !m.entries[0].isDir || m.entries[0].name != "sub" {
		t.Fatalf("dirs first: %+v", m.entries)
	}
	if m.entries[1].isDir || m.entries[2].isDir {
		t.Fatalf("files after dirs: %+v", m.entries)
	}
}

func TestEnterDirAndBack(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	root := mkMusicDir(t)
	m := New(root)
	um, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter}) // cursor 0 = sub
	m = um.(Model)
	if m.dir != filepath.Join(root, "sub") {
		t.Fatalf("want sub dir, got %s", m.dir)
	}
	if len(m.entries) != 1 || m.entries[0].name != "c" {
		t.Fatalf("sub content wrong: %+v", m.entries)
	}
	um, _ = m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	m = um.(Model)
	if m.dir != root {
		t.Fatalf("want back to root, got %s", m.dir)
	}
}

func TestPlayFileSurvivesBrowse(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	root := mkMusicDir(t)
	writeToneWAV(t, filepath.Join(root, "tone.wav"))
	m := New(root)
	// find tone.wav entry
	idx := -1
	for i, e := range m.entries {
		if !e.isDir && filepath.Base(e.path) == "tone.wav" {
			idx = i
		}
	}
	if idx < 0 {
		t.Fatalf("tone.wav not listed: %+v", m.entries)
	}
	m.cursor = idx
	m.loadAndPlay(m.entries[idx].track)
	if !m.hasPlaying || m.nowPlaying.Name != "tone" {
		t.Fatalf("not playing: %+v", m.nowPlaying)
	}
	// browse away: right panel + status must keep the track
	m.browse(filepath.Join(root, "sub"))
	if !m.hasPlaying || m.nowPlaying.Name != "tone" {
		t.Fatal("now playing lost after browse")
	}
	if out := m.View(); !strings.Contains(out, "tone") {
		t.Fatal("right panel must keep playing track")
	}
}

func TestBadFileDoesNotStart(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	m := New(mkMusicDir(t))
	m.cursor = 1 // a.mp3 (garbage content)
	m.loadAndPlay(m.entries[1].track)
	if m.hasPlaying {
		t.Fatal("garbage file must not start playing")
	}
	if !strings.HasPrefix(m.status, "ERR") {
		t.Fatalf("want ERR status, got %q", m.status)
	}
}

func TestFilterMatchesDirs(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	m := New(mkMusicDir(t))
	m.search.SetValue("sub")
	m.applyFilter()
	if len(m.entries) != 1 || !m.entries[0].isDir {
		t.Fatalf("filter must match dir: %+v", m.entries)
	}
}

func TestShortPath(t *testing.T) {
	if shortPath("/a/b/c") != "/a/b/c" {
		t.Fatal("short path must stay short")
	}
	home, _ := os.UserHomeDir()
	if got := shortPath(home); got != "~" {
		t.Fatalf("home must be ~, got %s", got)
	}
	if got := shortPath(filepath.Join(home, "a", "b", "c", "d", "e")); !strings.HasPrefix(got, "~/") && !strings.HasPrefix(got, "…") {
		t.Fatalf("deep home path must compress, got %s", got)
	}
}

func TestAllThemesRender(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	for _, th := range AllThemes() {
		m := New(mkMusicDir(t))
		m.theme = th
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
	m := New(t.TempDir())
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
	m := New(mkMusicDir(t))
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
	m := New(t.TempDir())
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
	m := New(t.TempDir())
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

func TestDurMsgFillsLength(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	root := mkMusicDir(t)
	writeToneWAV(t, filepath.Join(root, "tone.wav"))
	m := New(root)
	idx := -1
	for i, e := range m.entries {
		if !e.isDir && filepath.Base(e.path) == "tone.wav" {
			idx = i
		}
	}
	if idx < 0 {
		t.Fatal("tone.wav missing")
	}
	m.cursor = idx
	m.loadAndPlay(m.entries[idx].track)
	if !m.hasPlaying {
		t.Fatal("must be playing")
	}
	um, _ := m.Update(durMsg{path: m.nowPlaying.Path, dur: 10 * time.Second})
	m = um.(Model)
	if m.player.Duration() != 10*44100 {
		t.Fatalf("player length want %d got %d", 10*44100, m.player.Duration())
	}
	found := false
	for _, tr := range m.tracks {
		if tr.Path == m.nowPlaying.Path && tr.Duration == 10*1000000000 {
			found = true
		}
	}
	if !found {
		t.Fatal("track duration not updated")
	}
}
