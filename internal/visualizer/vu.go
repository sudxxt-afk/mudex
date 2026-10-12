package visualizer

import (
	"math"
	"strings"
)

// clamp01 clamps v to [0,1].
func clamp01(v float64) float64 {
	if math.IsNaN(v) {
		return 0
	}
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

// parseHexColor parses "#RRGGBB" (leading '#' optional).
// On any malformed input it falls back to white (255,255,255).
func parseHexColor(s string) (int, int, int) {
	t := strings.TrimSpace(s)
	t = strings.TrimPrefix(t, "#")
	if len(t) != 6 {
		return 255, 255, 255
	}
	hexVal := func(c byte) (int, bool) {
		switch {
		case c >= '0' && c <= '9':
			return int(c - '0'), true
		case c >= 'a' && c <= 'f':
			return int(c-'a') + 10, true
		case c >= 'A' && c <= 'F':
			return int(c-'A') + 10, true
		}
		return 0, false
	}
	vals := make([]int, 6)
	for i := 0; i < 6; i++ {
		v, ok := hexVal(t[i])
		if !ok {
			return 255, 255, 255
		}
		vals[i] = v
	}
	r := vals[0]*16 + vals[1]
	g := vals[2]*16 + vals[3]
	b := vals[4]*16 + vals[5]
	return r, g, b
}

// RenderVU draws ONE horizontal VU meter: scale + needle + peak mark.
// level, peak in [0,1] (peak may be < level — then no mark). width is the
// scale width in cells (8..60).
// fgHex, dimHex, peakHex are "#RRGGBB" colors (parsed internally,
// broken strings fall back to white).
// Returns 3 strings: scale with ticks, meter line, labels "-20 -10 -5 0 +3".
func RenderVU(level, peak float64, width int, fgHex, dimHex, peakHex string) []string {
	level = clamp01(level)
	peak = clamp01(peak)
	if width < 8 {
		width = 8
	}
	if width > 60 {
		width = 60
	}

	fr, fg, fb := parseHexColor(fgHex)
	dr, dg, db := parseHexColor(dimHex)
	pr, pg, pb := parseHexColor(peakHex)
	fgCode := ansiRGB(fr, fg, fb)
	dimCode := ansiRGB(dr, dg, db)
	peakCode := ansiRGB(pr, pg, pb)
	reset := "\x1b[0m"

	// Scale line: ticks every ~1/6 of width.
	step := width / 6
	if step < 1 {
		step = 1
	}
	var sb0 strings.Builder
	for i := 0; i < width; i++ {
		if i%step == 0 {
			sb0.WriteString(dimCode + "│" + reset)
		} else {
			sb0.WriteString(dimCode + "·" + reset)
		}
	}

	// Meter line.
	needlePos := int(level * float64(width))
	if needlePos >= width {
		needlePos = width - 1
	}
	if needlePos < 0 {
		needlePos = 0
	}
	peakPos := int(peak * float64(width))
	if peakPos >= width {
		peakPos = width - 1
	}
	if peakPos < 0 {
		peakPos = 0
	}
	showPeak := peak > level+0.02

	var sb1 strings.Builder
	for i := 0; i < width; i++ {
		switch {
		case i == needlePos:
			sb1.WriteString(fgCode + "▶" + reset)
		case showPeak && i == peakPos:
			sb1.WriteString(peakCode + "┃" + reset)
		case i < needlePos:
			sb1.WriteString(fgCode + "─" + reset)
		default:
			sb1.WriteString(dimCode + "─" + reset)
		}
	}

	return []string{sb0.String(), sb1.String(), "-20 -10 -5 0 +3"}
}

// RenderWave draws an oscilloscope of mono samples [-1,1]: width columns,
// height rows, dim center axis, bright wave. Returns height strings.
func RenderWave(samples []float64, width, height int, fgHex, dimHex string) []string {
	if width < 8 {
		width = 8
	}
	if width > 60 {
		width = 60
	}
	if height < 2 {
		height = 2
	}
	if height > 12 {
		height = 12
	}

	fr, fg, fb := parseHexColor(fgHex)
	dr, dg, db := parseHexColor(dimHex)
	fgCode := ansiRGB(fr, fg, fb)
	dimCode := ansiRGB(dr, dg, db)
	reset := "\x1b[0m"

	axisRow := height / 2

	// Resample: one value per column.
	colVal := make([]float64, width)
	hasData := len(samples) > 0
	if hasData {
		for c := 0; c < width; c++ {
			idx := int(float64(c) * float64(len(samples)) / float64(width))
			if idx >= len(samples) {
				idx = len(samples) - 1
			}
			v := samples[idx]
			if math.IsNaN(v) {
				v = 0
			}
			if v < -1 {
				v = -1
			}
			if v > 1 {
				v = 1
			}
			colVal[c] = v
		}
	}
	waveRow := make([]int, width)
	for c := range waveRow {
		waveRow[c] = -1
	}
	if hasData {
		for c, v := range colVal {
			r := int(math.Round((1 - (v+1)/2) * float64(height-1)))
			if r < 0 {
				r = 0
			}
			if r >= height {
				r = height - 1
			}
			waveRow[c] = r
		}
	}

	rows := make([]string, height)
	for r := 0; r < height; r++ {
		var sb strings.Builder
		for c := 0; c < width; c++ {
			if hasData && waveRow[c] == r {
				sb.WriteString(fgCode + "●" + reset)
			} else if r == axisRow {
				sb.WriteString(dimCode + "─" + reset)
			} else {
				sb.WriteString(" ")
			}
		}
		rows[r] = sb.String()
	}
	return rows
}
