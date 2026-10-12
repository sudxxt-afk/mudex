// Package visualizer computes neon spectrum bars from PCM using FFT.
package visualizer

import (
	"math"
	"math/cmplx"

	"github.com/madelynnblue/go-dsp/fft"
)

const (
	// FFTSize is the analysis window.
	FFTSize = 1024
	// Bars is the number of output columns.
	Bars = 24
)

// State holds smoothing between frames.
type State struct {
	levels []float64
	peaks  []float64
	wave   []float64
}

// New returns a visualizer state.
func New() *State {
	return &State{
		levels: make([]float64, Bars),
		peaks:  make([]float64, Bars),
	}
}

// Wave resamples mono pcm to width columns with linear interpolation and
// temporal smoothing (lerp toward target), so the oscilloscope glides
// instead of jumping. State is kept across calls; width changes reset it.
func (s *State) Wave(pcm []float64, width int) []float64 {
	if width < 8 {
		width = 8
	}
	if width > 80 {
		width = 80
	}
	target := resampleWave(pcm, width)
	if len(s.wave) != width {
		s.wave = append(s.wave[:0], target...)
		return append([]float64(nil), target...)
	}
	out := make([]float64, width)
	for i := range target {
		// fast attack, gentle release per column
		a := 0.55
		if (target[i] >= 0) != (s.wave[i] >= 0) || abs(target[i]) < abs(s.wave[i]) {
			a = 0.30
		}
		out[i] = s.wave[i] + (target[i]-s.wave[i])*a
	}
	s.wave = append(s.wave[:0], out...)
	return out
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

// resampleWave maps arbitrary-length pcm to width columns by averaging each
// bucket (anti-alias) — no more flicker from point sampling.
func resampleWave(pcm []float64, width int) []float64 {
	out := make([]float64, width)
	if len(pcm) == 0 {
		return out
	}
	per := float64(len(pcm)) / float64(width)
	for i := 0; i < width; i++ {
		lo := int(float64(i) * per)
		hi := int(float64(i+1) * per)
		if hi <= lo {
			hi = lo + 1
		}
		if hi > len(pcm) {
			hi = len(pcm)
		}
		sum := 0.0
		for _, v := range pcm[lo:hi] {
			sum += v
		}
		out[i] = sum / float64(hi-lo)
	}
	return out
}

// Compute takes mono PCM samples in [-1,1], returns Bars levels in [0,1].
// Short inputs are zero-padded; long inputs use the last FFTSize samples.
func (s *State) Compute(pcm []float64) []float64 {
	buf := make([]float64, FFTSize)
	if len(pcm) >= FFTSize {
		copy(buf, pcm[len(pcm)-FFTSize:])
	} else {
		copy(buf[FFTSize-len(pcm):], pcm)
	}
	// Hann window to reduce leakage.
	for i := range buf {
		buf[i] *= 0.5 * (1 - math.Cos(2*math.Pi*float64(i)/float64(FFTSize-1)))
	}
	spec := fft.FFTReal(buf)
	// Log-scale bucketing: 24 bars over first half spectrum.
	half := FFTSize / 2
	out := make([]float64, Bars)
	for b := 0; b < Bars; b++ {
		lo := int(math.Pow(float64(half), float64(b)/float64(Bars)))
		hi := int(math.Pow(float64(half), float64(b+1)/float64(Bars)))
		if hi <= lo {
			hi = lo + 1
		}
		if lo < 1 {
			lo = 1
		}
		if hi > half {
			hi = half
		}
		sum := 0.0
		for k := lo; k < hi; k++ {
			sum += cmplx.Abs(spec[k]) / float64(hi-lo)
		}
		// log compression
		mag := math.Log1p(sum*8) / math.Log1p(8*4)
		if mag < 0 {
			mag = 0
		}
		if mag > 1 {
			mag = 1
		}
		// smoothing: fast attack, slow release (tuned for ~30fps ticks)
		if mag > s.levels[b] {
			s.levels[b] = s.levels[b]*0.35 + mag*0.65
		} else {
			s.levels[b] = s.levels[b]*0.90 + mag*0.10
		}
		if s.levels[b] > s.peaks[b] {
			s.peaks[b] = s.levels[b]
		} else {
			s.peaks[b] -= 0.008
			if s.peaks[b] < s.levels[b] {
				s.peaks[b] = s.levels[b]
			}
			if s.peaks[b] < 0 {
				s.peaks[b] = 0
			}
		}
		out[b] = s.levels[b]
	}
	return out
}

// Peaks returns current peak-hold values (same length as Bars).
func (s *State) Peaks() []float64 {
	cp := make([]float64, Bars)
	copy(cp, s.peaks)
	return cp
}

// RenderBars renders levels as neon vertical bars of given height rows.
// Returns height strings, each with Bars columns of block chars.
func RenderBars(levels []float64, height int) []string {
	if height < 2 {
		height = 2
	}
	if height > 12 {
		height = 12
	}
	return renderBarsGradient(levels, height, [3]int{20, 255, 255}, [3]int{255, 80, 255})
}

// RenderBarsThemed is RenderBars with a theme gradient from lowHex (bottom)
// to highHex (top). Bad hex falls back to white.
func RenderBarsThemed(levels []float64, height int, lowHex, highHex string) []string {
	lr, lg, lb := parseHexColor(lowHex)
	hr, hg, hb := parseHexColor(highHex)
	return renderBarsGradient(levels, height, [3]int{lr, lg, lb}, [3]int{hr, hg, hb})
}

// edgeChars renders fractional cell fill: index 0 = empty … 8 = full.
var edgeChars = []string{" ", "▁", "▂", "▃", "▄", "▅", "▆", "▇", "█"}

func renderBarsGradient(levels []float64, height int, low, high [3]int) []string {
	rows := make([]string, height)
	for r := 0; r < height; r++ {
		line := ""
		frac := float64(r) / float64(height-1)
		red := low[0] + int(frac*float64(high[0]-low[0]))
		green := low[1] + int(frac*float64(high[1]-low[1]))
		blue := low[2] + int(frac*float64(high[2]-low[2]))
		prefix := ansiRGB(red, green, blue)
		for _, lv := range levels {
			if lv < 0 {
				lv = 0
			}
			if lv > 1 {
				lv = 1
			}
			f := lv * float64(height)
			full := int(f)
			edge := int((f - float64(full)) * 8)
			b := height - 1 - r // cell index from bottom
			ch := " "
			switch {
			case b < full:
				ch = "█"
			case b == full && edge > 0:
				ch = edgeChars[edge]
			}
			if ch == " " {
				line += " "
			} else {
				line += prefix + ch + "\x1b[0m"
			}
			line += " "
		}
		rows[r] = line
	}
	return rows
}

func ansiRGB(r, g, b int) string {
	return "\x1b[38;2;" + itoa(r) + ";" + itoa(g) + ";" + itoa(b) + "m"
}

func itoa(n int) string {
	if n < 0 {
		n = 0
	}
	if n > 255 {
		n = 255
	}
	var buf [3]byte
	i := 3
	if n == 0 {
		return "0"
	}
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
