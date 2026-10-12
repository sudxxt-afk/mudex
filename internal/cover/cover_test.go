package cover

import (
	"image"
	"image/color"
	"strings"
	"testing"
)

func solid(w, h int, c color.RGBA) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetRGBA(x, y, c)
		}
	}
	return img
}

func TestRenderHalfBlockDims(t *testing.T) {
	img := solid(100, 100, color.RGBA{255, 0, 128, 255})
	lines := RenderHalfBlock(img, 20, true, false)
	if len(lines) != 20 { // heightPx=40 -> 20 rows
		t.Fatalf("want 20 rows got %d", len(lines))
	}
	for _, l := range lines {
		if !strings.Contains(l, "▀") {
			t.Fatal("missing block char")
		}
	}
}

func TestRenderHalfBlock256(t *testing.T) {
	img := solid(10, 10, color.RGBA{10, 20, 30, 255})
	lines := RenderHalfBlock(img, 10, false, true)
	if len(lines) == 0 || !strings.Contains(lines[0], "[38;5;") {
		t.Fatalf("want 256-color escape, got %q", lines)
	}
}

func TestFallbackDeterministic(t *testing.T) {
	a := FallbackArt("song-a", 20)
	b := FallbackArt("song-a", 20)
	if strings.Join(a, "\n") != strings.Join(b, "\n") {
		t.Fatal("fallback must be deterministic")
	}
	if len(Neonize(a, 1)) != len(a) {
		t.Fatal("neonize length mismatch")
	}
}

func TestRgbTo256Gray(t *testing.T) {
	if rgbTo256(0, 0, 0) != 16 {
		t.Fatal("black map wrong")
	}
	if rgbTo256(255, 0, 0) < 16 {
		t.Fatal("red map wrong")
	}
}
