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
