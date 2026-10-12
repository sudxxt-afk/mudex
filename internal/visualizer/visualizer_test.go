package visualizer

import (
	"math"
	"strings"
	"testing"
)

func sine(freq float64, n int, sr float64) []float64 {
	out := make([]float64, n)
	for i := range out {
		out[i] = math.Sin(2 * math.Pi * freq * float64(i) / sr)
	}
	return out
}

func TestSilenceNearZero(t *testing.T) {
	s := New()
	lv := s.Compute(make([]float64, FFTSize))
	for _, v := range lv {
		if v > 0.05 {
			t.Fatalf("silence should be ~0, got %v", lv)
		}
	}
}

func TestToneProducesEnergy(t *testing.T) {
	s := New()
	lv := s.Compute(sine(440, FFTSize, 44100))
	max := 0.0
	for _, v := range lv {
		if v > max {
			max = v
		}
	}
	if max < 0.2 {
		t.Fatalf("tone should produce energy, max=%v", max)
	}
}

func TestRenderBarsDims(t *testing.T) {
	s := New()
	lv := s.Compute(sine(440, FFTSize, 44100))
	rows := RenderBars(lv, 6)
	if len(rows) != 6 {
		t.Fatalf("want 6 rows got %d", len(rows))
	}
}

func TestSmoothingReleases(t *testing.T) {
	s := New()
	s.Compute(sine(440, FFTSize, 44100))
	high := append([]float64(nil), s.levels...)
	s.Compute(make([]float64, FFTSize))
	for i := range high {
		if s.levels[i] > high[i] {
			t.Fatal("levels should decay on silence")
		}
	}
}

func TestBarsFractionalEdge(t *testing.T) {
	// lv=0.45 of height 6 -> fill 2.7: two full cells + one partial (not full, not empty)
	rows := RenderBars([]float64{0.45}, 6)
	partials := "▁▂▃▄▅▆▇"
	found := false
	for _, row := range rows {
		for _, r := range row {
			if strings.ContainsRune(partials, r) {
				found = true
			}
		}
	}
	if !found {
		t.Fatalf("want fractional edge char, rows=%q", rows)
	}
}

func TestWaveConverges(t *testing.T) {
	s := New()
	s.Wave(make([]float64, 512), 32) // prime with silence
	target := sine(440, 512, 44100)
	w1 := s.Wave(target, 32)
	w2 := s.Wave(target, 32)
	d1, d2 := 0.0, 0.0
	// compare against static resample
	ref := resampleWave(target, 32)
	for i := range ref {
		d1 += (w1[i] - ref[i]) * (w1[i] - ref[i])
		d2 += (w2[i] - ref[i]) * (w2[i] - ref[i])
	}
	if !(d2 < d1) {
		t.Fatalf("wave must converge: d1=%v d2=%v", d1, d2)
	}
}

func TestWaveEmptyStable(t *testing.T) {
	s := New()
	w := s.Wave(nil, 16)
	if len(w) != 16 {
		t.Fatalf("want 16 cols got %d", len(w))
	}
	rows := RenderWaveCols(w, 16, 4, "#FFFFFF", "#888888")
	if len(rows) != 4 {
		t.Fatalf("want 4 rows got %d", len(rows))
	}
}
