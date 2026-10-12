package cover

import (
	"image"
	"image/color"
	"strings"
	"testing"
)

func gradImage(w, h int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			v := uint8((x * 255) / (w - 1))
			img.SetRGBA(x, y, color.RGBA{v, uint8((y * 255) / (h - 1)), 128, 255})
		}
	}
	return img
}

func TestSextantDims(t *testing.T) {
	img := gradImage(100, 100)
	lines := RenderSextants(img, 20, 0, true)
	if len(lines) != 10 { // rows = 20*100/(2*100)
		t.Fatalf("want 10 rows got %d", len(lines))
	}
}

func TestSextantMaxRows(t *testing.T) {
	img := gradImage(100, 200)
	lines := RenderSextants(img, 20, 6, true)
	if len(lines) != 6 {
		t.Fatalf("want cap 6 rows got %d", len(lines))
	}
}

// stripANSI removes escape sequences for cell-content assertions.
func stripANSI(s string) string {
	var b strings.Builder
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
		b.WriteRune(r)
	}
	return b.String()
}

func assertUniformCells(t *testing.T, lines []string, width int) {
	t.Helper()
	for _, l := range lines {
		stripped := stripANSI(l)
		if len([]rune(stripped)) != width {
			t.Fatalf("want %d cells, got %q", width, stripped)
		}
		for _, r := range stripped {
			// uniform area: only empty or full cells (same color either way)
			if r != ' ' && r != '█' {
				t.Fatalf("uniform area must not use partial glyphs, got %q", stripped)
			}
		}
	}
}

func TestSextantSolidWhite(t *testing.T) {
	img := solid(50, 50, color.RGBA{255, 255, 255, 255})
	assertUniformCells(t, RenderSextants(img, 10, 0, true), 10)
}

func TestSextantSolidBlack(t *testing.T) {
	img := solid(50, 50, color.RGBA{0, 0, 0, 255})
	assertUniformCells(t, RenderSextants(img, 10, 0, true), 10)
}

func TestSextantGradientUsesSextants(t *testing.T) {
	lines := RenderSextants(gradImage(120, 120), 24, 0, true)
	found := false
	for _, l := range lines {
		for _, r := range l {
			if r >= 0x1FB00 && r <= 0x1FB3B {
				found = true
				break
			}
		}
	}
	if !found {
		t.Fatal("gradient should produce sextant glyphs")
	}
}

func TestSextantCharSubstitutes(t *testing.T) {
	if sextantChar(0) != ' ' || sextantChar(63) != '█' {
		t.Fatal("0/63 substitutes wrong")
	}
	if sextantChar(21) != '▌' || sextantChar(42) != '▐' {
		t.Fatal("21/42 substitutes wrong")
	}
	if sextantChar(1) != 0x1FB00 || sextantChar(62) != 0x1FB3B {
		t.Fatal("table lookup wrong")
	}
	if len(sextantGlyphs) != 60 {
		t.Fatalf("want 60 dedicated sextants, got %d", len(sextantGlyphs))
	}
}

func TestSextant256Color(t *testing.T) {
	lines := RenderSextants(gradImage(40, 40), 10, 0, false)
	if len(lines) == 0 || !strings.Contains(lines[0], "[38;5;") {
		t.Fatalf("want 256-color escapes, got %q", lines)
	}
}
