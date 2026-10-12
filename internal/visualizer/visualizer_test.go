package visualizer

import (
	"math"
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
