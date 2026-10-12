package visualizer

import (
	"strings"
	"testing"
	"unicode/utf8"
)

// stripANSI removes "\x1b[...m" sequences for rune counting.
func stripANSI(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); {
		if s[i] == 0x1b && i+1 < len(s) && s[i+1] == '[' {
			j := i + 2
			for j < len(s) && s[j] != 'm' {
				j++
			}
			if j < len(s) {
				j++
			}
			i = j
			continue
		}
		b.WriteByte(s[i])
		i++
	}
	return b.String()
}

func TestRenderVUDims(t *testing.T) {
	for _, w := range []int{2, 8, 20, 60, 200} {
		rows := RenderVU(0.5, 0.7, w, "#00ff00", "#555555", "#ff0000")
		if len(rows) != 3 {
			t.Fatalf("width=%d: want 3 rows got %d", w, len(rows))
		}
	}
}

func TestRenderVUNeedlePos(t *testing.T) {
	width := 20
	level := 0.5
	rows := RenderVU(level, 0, width, "#00ff00", "#555555", "#ff0000")
	plain := stripANSI(rows[1])
	want := int(level * float64(width))
	got := -1
	for i, r := range plain {
		if r == '▶' {
			got = i
			break
		}
	}
	_ = got
	// position in runes
	runes := []rune(plain)
	pos := -1
	for i, r := range runes {
		if r == '▶' {
			pos = i
			break
		}
	}
	if pos < 0 {
		t.Fatalf("needle ▶ not found in %q", plain)
	}
	if pos < want-1 || pos > want+1 {
		t.Fatalf("needle pos=%d want ~%d (plain=%q)", pos, want, plain)
	}
}

func TestRenderVUPeakPresent(t *testing.T) {
	rows := RenderVU(0.3, 0.8, 20, "#00ff00", "#555555", "#ff0000")
	if !strings.Contains(rows[1], "┃") && !strings.Contains(rows[1], "▌") {
		t.Fatalf("peak mark missing in %q", stripANSI(rows[1]))
	}
}

func TestRenderVUPeakAbsent(t *testing.T) {
	rows := RenderVU(0.5, 0.5, 20, "#00ff00", "#555555", "#ff0000")
	if strings.Contains(rows[1], "┃") || strings.Contains(rows[1], "▌") {
		t.Fatalf("peak mark should be absent when peak<=level: %q", stripANSI(rows[1]))
	}
	rows = RenderVU(0.7, 0.3, 20, "#00ff00", "#555555", "#ff0000")
	if strings.Contains(rows[1], "┃") || strings.Contains(rows[1], "▌") {
		t.Fatalf("peak mark should be absent when peak<level: %q", stripANSI(rows[1]))
	}
}

func TestRenderVUClamp(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("RenderVU panicked on clamp: %v", r)
		}
	}()
	width := 20
	rows := RenderVU(2.0, 5.0, width, "#00ff00", "#555555", "#ff0000")
	plain := stripANSI(rows[1])
	runes := []rune(plain)
	pos := -1
	for i, r := range runes {
		if r == '▶' {
			pos = i
			break
		}
	}
	if pos < 0 {
		t.Fatalf("needle missing after clamp")
	}
	if pos != width-1 {
		t.Fatalf("clamped needle pos=%d want %d", pos, width-1)
	}
	if utf8.RuneCountInString(plain) != width {
		t.Fatalf("meter visible width=%d want %d", utf8.RuneCountInString(plain), width)
	}
}

func TestRenderWaveDims(t *testing.T) {
	s := []float64{0, 0.5, -0.5, 1, -1, 0}
	rows := RenderWave(s, 16, 6, "#00ff00", "#555555")
	if len(rows) != 6 {
		t.Fatalf("want 6 rows got %d", len(rows))
	}
}

func TestRenderWaveEmpty(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("RenderWave panicked on empty: %v", r)
		}
	}()
	rows := RenderWave(nil, 16, 6, "#00ff00", "#555555")
	if len(rows) != 6 {
		t.Fatalf("want 6 rows got %d", len(rows))
	}
	found := false
	for _, r := range rows {
		if strings.Contains(r, "─") || strings.Contains(r, "·") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("axis symbol missing in empty wave")
	}
}

func TestRenderWaveBadHex(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("panicked on bad hex: %v", r)
		}
	}()
	RenderVU(0.5, 0.8, 20, "zzz", "", "not-a-color")
	RenderWave([]float64{0.1, -0.2}, 16, 6, "!!!", "xx")
}

func TestDotsDimsAndSymmetry(t *testing.T) {
	lv := []float64{0.1, 0.5, 0.9, 0.3, 0.7, 0.2, 0.6, 0.4}
	rows := RenderDots(lv, 6, "#FFFFFF", "#888888")
	if len(rows) != 6 {
		t.Fatalf("want 6 rows got %d", len(rows))
	}
	for _, row := range rows {
		cells := splitCells(row)
		if len(cells) != len(lv) {
			t.Fatalf("want %d cells got %d (%q)", len(lv), len(cells), row)
		}
		for i := 0; i < len(cells)/2; i++ {
			if cells[i] != cells[len(cells)-1-i] {
				t.Fatalf("row not symmetric: %q", row)
			}
		}
	}
	joined := strings.Join(rows, "\n")
	if !strings.Contains(joined, "●") {
		t.Fatal("want filled dots")
	}
}

func TestDotsClamp(t *testing.T) {
	rows := RenderDots([]float64{2.0, -1.0}, 4, "nope", "bad")
	if len(rows) != 4 {
		t.Fatalf("want 4 rows got %d", len(rows))
	}
}

// splitCells strips ANSI escapes and chunks a row into 2-rune cells
// (glyph + separator space).
func splitCells(s string) []string {
	var plain []rune
	inEsc := false
	for _, r := range s {
		if inEsc {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
				inEsc = false
			}
			continue
		}
		if r == 0x1b {
			inEsc = true
			continue
		}
		plain = append(plain, r)
	}
	var cells []string
	for i := 0; i+1 < len(plain); i += 2 {
		cells = append(cells, string(plain[i]))
	}
	return cells
}
