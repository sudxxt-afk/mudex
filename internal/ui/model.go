// Package ui is the Bubble Tea interface: library list, cover, visualizer, progress.
package ui

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"tui-player/internal/cover"
	"tui-player/internal/fonts"
	"tui-player/internal/library"
	"tui-player/internal/player"
	"tui-player/internal/visualizer"
)

type tickMsg time.Time

// durMsg carries a late duration probe result (ffprobe fallback).
type durMsg struct {
	path string
	dur  time.Duration
}

func probeDurCmd(path string) tea.Cmd {
	return func() tea.Msg {
		if d := library.ProbeDuration(path); d > 0 {
			return durMsg{path, d}
		}
		return nil
	}
}

// fontsMsg carries the background Nerd Font check/install result.
type fontsMsg struct{ text string }

func fontsCmd() tea.Msg {
	if msg := fonts.Ensure(); msg != "" {
		return fontsMsg{msg}
	}
	return nil
}

func tickCmd(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(t time.Time) tea.Msg { return tickMsg(t) })
}

// Model is the TUI state.
type Model struct {
	dir        string
	tracks     []library.Track
	entries    []entry // dirs first, then audio files (filtered)
	cursor     int     // position in entries
	playing    int     // index into tracks of playingDir, -1 = none
	playingDir string
	nowPlaying library.Track
	hasPlaying bool
	player     *player.Player
	viz        *visualizer.State
	levels     []float64
	coverLines []string
	coverSrc   string
	bar        progress.Model
	search     textinput.Model
	searching  bool
	shuffle    bool
	repeat     bool
	vizMode    string // auto | bars | vu | wave | off
	coverMode  string // auto | sextant | half
	menuOpen   bool
	menuCursor int
	theme      Theme
	trueColor  bool
	width      int
	height     int
	status     string
	errCount   int
}

// entry is one browser row: a subdirectory or an audio file.
type entry struct {
	isDir bool
	name  string
	path  string
	track int // index into tracks when !isDir, else -1
}

// New builds a model browsing dir.
func New(dir string) Model {
	ti := textinput.New()
	ti.Placeholder = "filter…"
	ti.CharLimit = 64
	cfg := LoadConfig()
	if cfg.Viz == "" {
		cfg.Viz = "auto"
	}
	if cfg.Cover == "" {
		cfg.Cover = "auto"
	}
	th := ThemeByID(cfg.Theme)
	cover.SetRenderMode(cfg.Cover)
	bar := progress.New(progress.WithSolidFill(th.Accent), progress.WithWidth(40))
	abs, err := filepath.Abs(dir)
	if err != nil {
		abs = dir
	}
	m := Model{
		dir:       abs,
		playing:   -1,
		player:    player.New(),
		viz:       visualizer.New(),
		levels:    make([]float64, visualizer.Bars),
		bar:       bar,
		search:    ti,
		theme:     th,
		shuffle:   cfg.Shuffle,
		repeat:    cfg.Repeat,
		vizMode:   cfg.Viz,
		coverMode: cfg.Cover,
		trueColor: cover.TrueColor(),
		width:     100,
		height:    30,
		status:    "",
	}
	m.browse(m.dir)
	return m
}

// browse lists dir: subdirectories first, then audio files.
func (m *Model) browse(dir string) {
	m.dir = dir
	m.cursor = 0
	m.tracks = nil
	ents, err := os.ReadDir(dir)
	if err != nil {
		m.status = "ERR: " + err.Error()
		m.entries = nil
		return
	}
	_ = ents
	tr, err := library.ScanDir(dir, false)
	if err != nil {
		m.status = "ERR: " + err.Error()
		m.entries = nil
		return
	}
	m.tracks = tr
	m.applyFilter()
	ndirs := 0
	for _, e := range m.entries {
		if e.isDir {
			ndirs++
		}
	}
	m.status = fmt.Sprintf("%s — %d dirs, %d tracks", shortPath(dir), ndirs, len(tr))
}

// shortPath compresses a path for the header: ~ for home, … for deep tails.
func shortPath(p string) string {
	if home, err := os.UserHomeDir(); err == nil && p == home {
		return "~"
	}
	if home, err := os.UserHomeDir(); err == nil && strings.HasPrefix(p, home+string(os.PathSeparator)) {
		p = "~" + strings.TrimPrefix(p, home)
	}
	parts := strings.Split(p, string(os.PathSeparator))
	if len(parts) > 4 {
		return "…" + string(os.PathSeparator) + filepath.Join(parts[len(parts)-2:]...)
	}
	return p
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd {
	return tea.Batch(tickCmd(33*time.Millisecond), fontsCmd)
}

// entryAt maps cursor -> browser entry.
func (m *Model) entryAt(i int) *entry {
	if i < 0 || i >= len(m.entries) {
		return nil
	}
	return &m.entries[i]
}

func (m Model) currentTrack() *library.Track {
	if !m.hasPlaying {
		return nil
	}
	tr := m.nowPlaying
	return &tr
}

func (m *Model) applyFilter() {
	q := strings.ToLower(strings.TrimSpace(m.search.Value()))
	m.entries = m.entries[:0]
	if ents, err := os.ReadDir(m.dir); err == nil {
		for _, e := range ents {
			if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
				continue
			}
			if q == "" || strings.Contains(strings.ToLower(e.Name()), q) {
				m.entries = append(m.entries, entry{isDir: true, name: e.Name(), path: filepath.Join(m.dir, e.Name()), track: -1})
			}
		}
	}
	for i := range m.tracks {
		name := m.tracks[i].DisplayTitle()
		if q == "" || strings.Contains(strings.ToLower(name), q) ||
			strings.Contains(strings.ToLower(m.tracks[i].Artist), q) {
			m.entries = append(m.entries, entry{isDir: false, name: name, path: m.tracks[i].Path, track: i})
		}
	}
	if m.cursor >= len(m.entries) {
		m.cursor = len(m.entries) - 1
	}
	if m.cursor < 0 && len(m.entries) > 0 {
		m.cursor = 0
	}
}

// loadAndPlay loads tracks[ti] and starts it instantly. When the duration
// was unknown at scan time it returns a background probe Cmd that fills the
// progress bar late instead of blocking startup.
func (m *Model) loadAndPlay(ti int) tea.Cmd {
	if ti < 0 || ti >= len(m.tracks) {
		return nil
	}
	tr := &m.tracks[ti]
	m.player.Close()
	if err := m.player.Load(tr.Path, m.player.Frames(tr.Duration)); err != nil {
		m.status = "ERR: " + err.Error()
		m.errCount++
		return nil
	}
	if err := m.player.Play(); err != nil {
		m.status = "ERR audio: " + err.Error()
		m.errCount++
		return nil
	}
	m.playing = ti
	m.playingDir = m.dir
	m.nowPlaying = *tr
	m.hasPlaying = true
	m.status = "▶ " + tr.DisplayTitle() + tr.DisplayArtist()
	m.refreshCover()
	if tr.Duration <= 0 {
		return probeDurCmd(tr.Path)
	}
	return nil
}

func (m *Model) refreshCover() {
	tr := m.currentTrack()
	w := (m.width*55/100 - 6) / 1
	if w < 16 {
		w = 16
	}
	if w > 44 {
		w = 44
	}
	if tr == nil {
		m.coverLines = cover.Neonize(cover.FallbackArt("no music", w), 0)
		m.coverSrc = "none"
		return
	}
	data, src := cover.LoadCover(tr.Path)
	maxRows := m.height - 14
	if maxRows < 6 {
		maxRows = 6
	}
	if lines := cover.RenderBestGraded(data, w, maxRows, m.trueColor, m.theme.Grade); len(lines) > 0 {
		if len(lines) > maxRows {
			lines = lines[:maxRows]
		}
		m.coverLines = lines
		if cover.SextantsOK() {
			m.coverSrc = src + " (sextant)"
		} else {
			m.coverSrc = src + " (half-block)"
		}
		return
	}
	m.coverLines = cover.Neonize(cover.FallbackArt(tr.Name, w), int64(len(tr.Name)))
	m.coverSrc = "ascii"
}

// saveConfig persists theme + toggles + modes.
func (m *Model) saveConfig() {
	_ = Config{
		Theme:   m.theme.ID,
		Shuffle: m.shuffle,
		Repeat:  m.repeat,
		Viz:     m.vizMode,
		Cover:   m.coverMode,
	}.Save()
}

// cycleTheme switches to the next/previous theme, persists it and refreshes
// theme-dependent views (cover grading, progress bar).
func (m *Model) cycleTheme(step int) {
	all := AllThemes()
	cur := 0
	for i, th := range all {
		if th.ID == m.theme.ID {
			cur = i
			break
		}
	}
	m.theme = all[(cur+step+len(all))%len(all)]
	m.saveConfig()
	m.bar = progress.New(progress.WithSolidFill(m.theme.Accent), progress.WithWidth(min(m.width-20, 70)))
	m.refreshCover()
	m.status = "theme: " + m.theme.ID
}

func (m *Model) nextIndex(step int) int {
	if len(m.tracks) == 0 {
		return -1
	}
	// find pos of playing track in current dir
	pos := -1
	if m.hasPlaying && m.playingDir == m.dir {
		for i := range m.tracks {
			if m.tracks[i].Path == m.nowPlaying.Path {
				pos = i
				break
			}
		}
	}
	if pos < 0 {
		if m.shuffle && step > 0 {
			return rand.Intn(len(m.tracks))
		}
		return 0
	}
	if m.shuffle && step > 0 {
		return rand.Intn(len(m.tracks))
	}
	return (pos + step + len(m.tracks)) % len(m.tracks)
}

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.bar.Width = min(m.width-20, 70)
		m.refreshCover()
		return m, nil
	case fontsMsg:
		if msg.text != "" {
			m.status = msg.text
		}
		return m, nil
	case durMsg:
		if m.hasPlaying && msg.path == m.nowPlaying.Path && msg.dur > 0 {
			m.nowPlaying.Duration = msg.dur
			m.player.SetLength(m.player.Frames(msg.dur))
			for i := range m.tracks {
				if m.tracks[i].Path == msg.path {
					m.tracks[i].Duration = msg.dur
				}
			}
		}
		return m, nil
	case tickMsg:
		if m.player != nil && m.playing >= 0 {
			if m.player.Finished() {
				if m.repeat {
					// restart in place: works even when browsed away
					m.player.SeekBy(-1 << 30)
					return m, nil
				}
				if nx := m.nextIndex(1); nx >= 0 {
					return m, m.loadAndPlay(nx)
				}
			} else if m.vizMode != "off" {
				if m.player.IsPlaying() {
					m.levels = m.viz.Compute(m.player.SpectrumPCM(1024))
				} else {
					// decay
					m.levels = m.viz.Compute(make([]float64, 64))
				}
			}
		}
		return m, tickCmd(33 * time.Millisecond)
	case tea.KeyMsg:
		if msg.String() == "ctrl+p" {
			m.menuOpen = !m.menuOpen
			m.menuCursor = 0
			return m, nil
		}
		if m.menuOpen {
			switch msg.String() {
			case "esc":
				m.menuOpen = false
				return m, nil
			case "up", "k":
				if m.menuCursor > 0 {
					m.menuCursor--
				}
			case "down", "j":
				if m.menuCursor < len(m.menuRows())-1 {
					m.menuCursor++
				}
			case "left", "h":
				m.menuStep(-1)
			case "right", "l", "enter", " ":
				m.menuStep(1)
			}
			return m, nil
		}
		if m.searching {
			switch msg.String() {
			case "enter", "esc":
				m.searching = false
				m.search.Blur()
				m.applyFilter()
				return m, nil
			}
			var cmd tea.Cmd
			m.search, cmd = m.search.Update(msg)
			m.applyFilter()
			return m, cmd
		}
		switch msg.String() {
		case "q", "ctrl+c":
			m.player.Close()
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.entries)-1 {
				m.cursor++
			}
		case "enter", "right", "l":
			if e := m.entryAt(m.cursor); e != nil {
				if e.isDir {
					m.browse(e.path)
				} else {
					return m, m.loadAndPlay(e.track)
				}
			}
		case " ":
			if e := m.entryAt(m.cursor); e != nil && !e.isDir {
				if m.hasPlaying && m.nowPlaying.Path == e.path {
					if err := m.player.Toggle(); err != nil {
						m.status = "ERR: " + err.Error()
					} else if m.player.IsPlaying() {
						m.status = "▶ " + m.nowPlaying.DisplayTitle() + m.nowPlaying.DisplayArtist()
					} else {
						m.status = "⏸ " + m.nowPlaying.DisplayTitle() + m.nowPlaying.DisplayArtist()
					}
				} else {
					return m, m.loadAndPlay(e.track)
				}
			} else if m.hasPlaying {
				if err := m.player.Toggle(); err != nil {
					m.status = "ERR: " + err.Error()
				} else if m.player.IsPlaying() {
					m.status = "▶ " + m.nowPlaying.DisplayTitle() + m.nowPlaying.DisplayArtist()
				} else {
					m.status = "⏸ " + m.nowPlaying.DisplayTitle() + m.nowPlaying.DisplayArtist()
				}
			}
		case "n":
			if nx := m.nextIndex(1); nx >= 0 {
				return m, m.loadAndPlay(nx)
			}
		case "p":
			if nx := m.nextIndex(-1); nx >= 0 {
				return m, m.loadAndPlay(nx)
			}
		case "left", "h", "backspace":
			parent := filepath.Dir(m.dir)
			if parent != m.dir {
				m.browse(parent)
			}
		case ",", "<":
			m.player.SeekBy(-5 * m.player.SampleRate())
		case ".", ">":
			m.player.SeekBy(5 * m.player.SampleRate())
		case "+", "=":
			m.player.VolumeAdd(1)
		case "-", "_":
			m.player.VolumeAdd(-1)
		case "s":
			m.shuffle = !m.shuffle
			m.saveConfig()
		case "r":
			m.repeat = !m.repeat
			m.saveConfig()
		case "v":
			m.cycleViz(1)
		case "/":
			m.searching = true
			m.search.Focus()
			return m, textinput.Blink
		case "t":
			m.cycleTheme(1)
		case "T":
			m.cycleTheme(-1)
		case "R":
			m.browse(m.dir)
		}
		return m, nil
	}
	return m, nil
}

// vizModes lists selectable visualizer modes.
func vizModes() []string { return []string{"auto", "bars", "vu", "wave", "dots", "off"} }

// cycleViz moves vizMode one step, wrapping around.
func (m *Model) cycleViz(step int) {
	modes := vizModes()
	cur := 0
	for i, v := range modes {
		if v == m.vizMode {
			cur = i
			break
		}
	}
	m.vizMode = modes[(cur+step+len(modes))%len(modes)]
	m.saveConfig()
	m.status = "visualizer: " + m.vizMode
}

// effViz resolves the effective visualizer style ("off" hides it).
func (m *Model) effViz() VizStyle {
	switch m.vizMode {
	case "off":
		return -1
	case "bars":
		return VizBars
	case "vu":
		return VizVU
	case "wave":
		return VizWave
	case "dots":
		return VizDots
	default: // auto
		return m.theme.Viz
	}
}

// vizRows renders the visualizer in the current theme style.
func (m Model) vizRows(rightW int) []string {
	w := rightW - 6
	if w < 8 {
		w = 8
	}
	if w > 44 {
		w = 44
	}
	switch m.effViz() {
	case VizVU:
		lv, pk := 0.0, 0.0
		for _, v := range m.levels {
			if v > lv {
				lv = v
			}
		}
		for _, v := range m.viz.Peaks() {
			if v > pk {
				pk = v
			}
		}
		return visualizer.RenderVU(lv, pk, w, m.theme.Accent, m.theme.Dim, m.theme.Peak)
	case VizWave:
		cols := m.viz.Wave(m.player.SpectrumPCM(1024), w)
		return visualizer.RenderWaveCols(cols, w, 5, m.theme.Accent, m.theme.Dim)
	case VizDots:
		return visualizer.RenderDots(m.levels, 6, m.theme.Accent, m.theme.Dim)
	default:
		if len(m.levels) == 0 {
			return nil
		}
		return visualizer.RenderBarsThemed(m.levels, 5, m.theme.Accent2, m.theme.Accent)
	}
}

// menuRows lists settings rows as [label, value].
func (m Model) menuRows() [][2]string {
	return [][2]string{
		{"Theme", m.theme.ID},
		{"Shuffle", onOff(m.shuffle)},
		{"Repeat", onOff(m.repeat)},
		{"Visualizer", m.vizMode},
		{"Cover", m.coverMode},
	}
}

// menuStep changes the current row value by step (-1/+1).
func (m *Model) menuStep(step int) {
	switch m.menuCursor {
	case 0:
		m.cycleTheme(step)
	case 1:
		if step != 0 {
			m.shuffle = !m.shuffle
			m.saveConfig()
			m.status = "shuffle: " + onOff(m.shuffle)
		}
	case 2:
		if step != 0 {
			m.repeat = !m.repeat
			m.saveConfig()
			m.status = "repeat: " + onOff(m.repeat)
		}
	case 3:
		m.cycleViz(step)
	case 4:
		modes := []string{"auto", "sextant", "half"}
		cur := 0
		for i, v := range modes {
			if v == m.coverMode {
				cur = i
				break
			}
		}
		m.coverMode = modes[(cur+step+len(modes))%len(modes)]
		cover.SetRenderMode(m.coverMode)
		m.saveConfig()
		m.refreshCover()
		m.status = "cover: " + m.coverMode
	}
}

// menuView renders the settings overlay box.
func (m Model) menuView() string {
	th := m.theme
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(th.Title))
	selStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(th.Accent)).Background(lipgloss.Color(th.SelBG))
	textStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(th.Text))
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(th.Dim))
	border := lipgloss.RoundedBorder()
	if !th.RoundedBorder {
		border = lipgloss.NormalBorder()
	}
	var b strings.Builder
	b.WriteString(titleStyle.Render("⚙ Settings") + "\n\n")
	rows := m.menuRows()
	for i, r := range rows {
		line := fmt.Sprintf("  %-10s  < %s >", r[0], r[1])
		if i == m.menuCursor {
			line = selStyle.Render("▸ " + fmt.Sprintf("%-10s  < %s >", r[0], r[1]))
		} else {
			line = textStyle.Render(line)
		}
		b.WriteString(line + "\n")
	}
	b.WriteString("\n" + dimStyle.Render("j/k move · h/l change · esc close"))
	box := lipgloss.NewStyle().Border(border).BorderForeground(lipgloss.Color(th.BorderFG)).Padding(1, 3).Render(b.String())
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}

// View implements tea.Model.
func (m Model) View() string {
	if m.menuOpen {
		return m.menuView()
	}
	if m.width == 0 {
		return "loading…"
	}
	th := m.theme
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(th.Title))
	textStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(th.Text))
	selStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(th.Accent)).Background(lipgloss.Color(th.SelBG))
	playStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(th.Accent2))
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(th.Dim))
	border := lipgloss.RoundedBorder()
	if !th.RoundedBorder {
		border = lipgloss.NormalBorder()
	}
	panel := lipgloss.NewStyle().Border(border).BorderForeground(lipgloss.Color(th.BorderFG)).Padding(0, 1)

	// left: file list
	leftW := m.width * 42 / 100
	if leftW < 28 {
		leftW = 28
	}
	rightW := m.width - leftW - 6
	listH := m.height - 10
	if listH < 8 {
		listH = 8
	}
	var lb strings.Builder
	lb.WriteString(titleStyle.Render("♪ "+shortPath(m.dir)) + "\n")
	lb.WriteString(dimStyle.Render("↵ open · ⌫ up") + "\n")
	if m.searching {
		lb.WriteString(m.search.View() + "\n")
	}
	start := 0
	if m.cursor > listH-5 {
		start = m.cursor - (listH - 5)
	}
	end := start + listH - 3
	if end > len(m.entries) {
		end = len(m.entries)
	}
	for i := start; i < end; i++ {
		e := m.entries[i]
		isPlayingRow := false
		var line string
		if e.isDir {
			line = fmt.Sprintf("📁 %s", truncate(e.name, leftW-8))
		} else {
			tr := m.tracks[e.track]
			icon := fileIcon(tr.Ext)
			line = fmt.Sprintf("%s %s", icon, truncate(tr.DisplayTitle()+tr.DisplayArtist(), leftW-8))
			if m.hasPlaying && tr.Path == m.nowPlaying.Path {
				isPlayingRow = true
				eq := "▶"
				if !m.player.IsPlaying() {
					eq = "⏸"
				}
				line = playStyle.Render(eq + " " + line)
			}
		}
		if !isPlayingRow {
			if i == m.cursor {
				line = selStyle.Render("▸ " + line)
			} else {
				line = "  " + line
			}
		} else if i == m.cursor {
			line = "▸ " + line
		}
		lb.WriteString(line + "\n")
	}
	if len(m.entries) == 0 {
		lb.WriteString(dimStyle.Render("  empty — try another folder") + "\n")
	}
	left := panel.Width(leftW).Height(listH).Render(textStyle.Render(lb.String()))

	// right: cover + meta + viz
	var rb strings.Builder
	if tr := m.currentTrack(); tr != nil {
		state := "⏸ paused"
		if m.player.IsPlaying() {
			state = "▶ playing"
		}
		rb.WriteString(titleStyle.Render(tr.DisplayTitle()) + "\n")
		if tr.Artist != "" || tr.Album != "" {
			rb.WriteString(dimStyle.Render(strings.Trim(tr.Artist+" · "+tr.Album, " ·")) + "\n")
		}
		rb.WriteString(dimStyle.Render(tr.Ext+" · "+state) + "\n")
		rb.WriteString(dimStyle.Render("cover: "+m.coverSrc+" · vol "+fmt.Sprintf("%.0f", m.player.Volume())+"dB") + "\n")
	} else {
		rb.WriteString(titleStyle.Render("— pick a track with Enter —") + "\n\n")
	}
	for _, l := range m.coverLines {
		rb.WriteString(l + "\n")
	}
	if m.effViz() >= 0 {
		rb.WriteString("\n")
		for _, row := range m.vizRows(rightW) {
			rb.WriteString(row + "\n")
		}
	}
	right := panel.Width(rightW).Height(listH).Render(textStyle.Render(rb.String()))

	top := lipgloss.JoinHorizontal(lipgloss.Top, left, " ", right)

	// bottom: progress + status + help
	var pct float64
	var posS, durS string
	if m.playing >= 0 && m.player.Duration() > 0 {
		pct = float64(m.player.Position()) / float64(m.player.Duration())
		if pct < 0 {
			pct = 0
		}
		if pct > 1 {
			pct = 1
		}
		posS = library.FormatDuration(time.Duration(m.player.Position()) * time.Second / time.Duration(m.player.SampleRate()))
		durS = library.FormatDuration(time.Duration(m.player.Duration()) * time.Second / time.Duration(m.player.SampleRate()))
	} else {
		posS, durS = "--:--", "--:--"
	}
	prog := m.bar.ViewAs(pct)
	flags := fmt.Sprintf("theme:%s shuffle:%v repeat:%v viz:%s", m.theme.ID, onOff(m.shuffle), onOff(m.repeat), m.vizMode)
	bottom := fmt.Sprintf("%s %s / %s   %s\n%s\n%s",
		prog, posS, durS, dimStyle.Render(flags),
		dimStyle.Render(m.status),
		dimStyle.Render("j/k move · enter open/play · ⌫ up · space pause · ,/. seek · n/p next/prev · +/- vol · s/r shuffle/repeat · v viz · t/T theme · ctrl+p settings · / filter · R rescan · q quit"),
	)
	_ = os.Getenv("")
	return top + "\n" + bottom
}

func onOff(b bool) string {
	if b {
		return "on"
	}
	return "off"
}

func fileIcon(ext string) string {
	switch ext {
	case ".mp3":
		return "🎵"
	case ".flac":
		return "💎"
	case ".wav":
		return "🌊"
	case ".ogg", ".opus":
		return "🌀"
	case ".m4a", ".aac":
		return "🍏"
	default:
		return "♪"
	}
}

func truncate(s string, n int) string {
	if n <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
