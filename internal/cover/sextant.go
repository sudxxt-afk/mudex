package cover

import (
	"fmt"
	"image"
	"os"
	"os/exec"
	"strings"
	"sync"
)

// Sextant rendering: 2x3 pixels per cell (U+1FB00 block) with Floyd-Steinberg
// dithering. Falls back to half-blocks when the terminal/font can't do it.

// sextantGlyphs maps a 6-bit cell mask (bit d-1 = sextant d, layout 1 2 / 3 4 / 5 6)
// to its codepoint. Masks 0 (space), 21 (left half), 42 (right half) and
// 63 (full) have no dedicated sextant and are handled separately.
var sextantGlyphs = map[int]rune{
	1: 0x1FB00, 2: 0x1FB01, 3: 0x1FB02, 4: 0x1FB03, 5: 0x1FB04, 6: 0x1FB05,
	7: 0x1FB06, 8: 0x1FB07, 9: 0x1FB08, 10: 0x1FB09, 11: 0x1FB0A, 12: 0x1FB0B,
	13: 0x1FB0C, 14: 0x1FB0D, 15: 0x1FB0E, 16: 0x1FB0F, 17: 0x1FB10, 18: 0x1FB11,
	19: 0x1FB12, 20: 0x1FB13, 22: 0x1FB14, 23: 0x1FB15, 24: 0x1FB16, 25: 0x1FB17,
	26: 0x1FB18, 27: 0x1FB19, 28: 0x1FB1A, 29: 0x1FB1B, 30: 0x1FB1C, 31: 0x1FB1D,
	32: 0x1FB1E, 33: 0x1FB1F, 34: 0x1FB20, 35: 0x1FB21, 36: 0x1FB22, 37: 0x1FB23,
	38: 0x1FB24, 39: 0x1FB25, 40: 0x1FB26, 41: 0x1FB27, 43: 0x1FB28, 44: 0x1FB29,
	45: 0x1FB2A, 46: 0x1FB2B, 47: 0x1FB2C, 48: 0x1FB2D, 49: 0x1FB2E, 50: 0x1FB2F,
	51: 0x1FB30, 52: 0x1FB31, 53: 0x1FB32, 54: 0x1FB33, 55: 0x1FB34, 56: 0x1FB35,
	57: 0x1FB36, 58: 0x1FB37, 59: 0x1FB38, 60: 0x1FB39, 61: 0x1FB3A, 62: 0x1FB3B,
}

var sextantProbe = sync.OnceValue(func() bool {
	if v := strings.ToLower(os.Getenv("MUDEX_COVER")); v != "" {
		return v == "sextant" || v == "auto"
	}
	term := strings.ToLower(os.Getenv("TERM"))
	if term == "" || term == "dumb" || term == "linux" {
		return false
	}
	// fontconfig knows whether any installed font covers the sextant block;
	// most modern terminals fall back to it automatically.
	out, err := exec.Command("fc-list", ":charset=1FB00", "family").Output()
	if err != nil {
		return false
	}
	return len(strings.TrimSpace(string(out))) > 0
})

// SextantsOK reports whether sextant glyphs will likely render.
func SextantsOK() bool { return sextantProbe() }

// RenderSextants scales img to width cells (2x3 px each, maxRows rows cap)
// and returns ANSI-colored lines. Uses Floyd-Steinberg dithering.
func RenderSextants(img image.Image, width, maxRows int, trueColor bool) []string {
	if width < 8 {
		width = 8
	}
	if width > 80 {
		width = 80
	}
	b := img.Bounds()
	srcW, srcH := b.Dx(), b.Dy()
	if srcW == 0 || srcH == 0 {
		return nil
	}
	// terminal cell ~ 1:2 (w:h); rows from display aspect
	rows := width * srcH / (2 * srcW)
	if rows < 2 {
		rows = 2
	}
	if maxRows > 0 && rows > maxRows {
		rows = maxRows
	}
	pxW, pxH := width*2, rows*3
	fb := scaleFloat(img, pxW, pxH)
	floydSteinberg(fb, pxW, pxH)

	lines := make([]string, 0, rows)
	var sb strings.Builder
	for r := 0; r < rows; r++ {
		sb.Reset()
		for c := 0; c < width; c++ {
			// gather 2x3 pixels, threshold at cell mean luminance
			var lum [6]float64
			var rr, gg, bb [6]float64
			sum := 0.0
			for dy := 0; dy < 3; dy++ {
				for dx := 0; dx < 2; dx++ {
					i := (r*3+dy)*pxW + (c*2 + dx)
					r1, g1, b1 := fb[i*3], fb[i*3+1], fb[i*3+2]
					k := dy*2 + dx
					rr[k], gg[k], bb[k] = r1, g1, b1
					lum[k] = 0.2126*r1 + 0.7152*g1 + 0.0722*b1
					sum += lum[k]
				}
			}
			avg := sum / 6
			mask := 0
			for k := 0; k < 6; k++ {
				if lum[k] > avg {
					mask |= 1 << uint(k)
				}
			}
			fg, bg := splitColors(rr[:], gg[:], bb[:], mask)
			ch := sextantChar(mask)
			if trueColor {
				fmt.Fprintf(&sb, "\x1b[38;2;%d;%d;%dm\x1b[48;2;%d;%d;%dm%s",
					fg[0], fg[1], fg[2], bg[0], bg[1], bg[2], string(ch))
			} else {
				fmt.Fprintf(&sb, "\x1b[38;5;%dm\x1b[48;5;%dm%s",
					rgbTo256(fg[0], fg[1], fg[2]), rgbTo256(bg[0], bg[1], bg[2]), string(ch))
			}
		}
		sb.WriteString("\x1b[0m")
		lines = append(lines, sb.String())
	}
	return lines
}

// sextantChar maps a cell mask to its glyph, using block-element substitutes
// for the three unencoded combinations.
func sextantChar(mask int) rune {
	switch mask {
	case 0:
		return ' '
	case 63:
		return '█' // U+2588
	case 21: // left column 1,3,5
		return '▌' // U+258C
	case 42: // right column 2,4,6
		return '▐' // U+2590
	default:
		if r, ok := sextantGlyphs[mask]; ok {
			return r
		}
		return ' '
	}
}

// splitColors returns fg (mean of set pixels) and bg (mean of unset).
func splitColors(rr, gg, bb []float64, mask int) ([3]int, [3]int) {
	var sr, sg, sb, br, bg, bb2 float64
	var nf, nb float64
	for k := 0; k < 6; k++ {
		if mask&(1<<uint(k)) != 0 {
			sr, sg, sb, nf = sr+rr[k], sg+gg[k], sb+bb[k], nf+1
		} else {
			br, bg, bb2, nb = br+rr[k], bg+gg[k], bb2+bb[k], nb+1
		}
	}
	fgc := [3]int{128, 128, 128}
	bgc := [3]int{0, 0, 0}
	if nf > 0 {
		fgc = [3]int{int(sr / nf), int(sg / nf), int(sb / nf)}
	}
	if nb > 0 {
		bgc = [3]int{int(br / nb), int(bg / nb), int(bb2 / nb)}
	} else {
		bgc = fgc
	}
	if nf == 0 {
		fgc = bgc
	}
	return fgc, bgc
}

// scaleFloat scales img to pxW*pxH with bilinear filtering into float RGB 0..255.
func scaleFloat(img image.Image, pxW, pxH int) []float64 {
	b := img.Bounds()
	srcW, srcH := float64(b.Dx()), float64(b.Dy())
	fb := make([]float64, pxW*pxH*3)
	for y := 0; y < pxH; y++ {
		sy := (float64(y)+0.5)*srcH/float64(pxH) - 0.5
		y0 := int(sy)
		fy := sy - float64(y0)
		if y0 < 0 {
			y0, fy = 0, 0
		}
		if y0 >= b.Dy()-1 {
			y0, fy = b.Dy()-1, 0
		}
		for x := 0; x < pxW; x++ {
			sx := (float64(x)+0.5)*srcW/float64(pxW) - 0.5
			x0 := int(sx)
			fx := sx - float64(x0)
			if x0 < 0 {
				x0, fx = 0, 0
			}
			if x0 >= b.Dx()-1 {
				x0, fx = b.Dx()-1, 0
			}
			var c [2][2][3]float64
			for j := 0; j < 2; j++ {
				for i := 0; i < 2; i++ {
					xx, yy := x0+i, y0+j
					if xx >= b.Dx() {
						xx = b.Dx() - 1
					}
					if yy >= b.Dy() {
						yy = b.Dy() - 1
					}
					r, g, bl, _ := img.At(b.Min.X+xx, b.Min.Y+yy).RGBA()
					c[j][i][0], c[j][i][1], c[j][i][2] = float64(r>>8), float64(g>>8), float64(bl>>8)
				}
			}
			o := (y*pxW + x) * 3
			for ch := 0; ch < 3; ch++ {
				top := c[0][0][ch]*(1-fx) + c[0][1][ch]*fx
				bot := c[1][0][ch]*(1-fx) + c[1][1][ch]*fx
				fb[o+ch] = top*(1-fy) + bot*fy
			}
		}
	}
	return fb
}

// floydSteinberg diffuses quantization error (to 8-bit) across the buffer.
func floydSteinberg(fb []float64, w, h int) {
	at := func(x, y, ch int) float64 { return fb[(y*w+x)*3+ch] }
	set := func(x, y, ch int, v float64) { fb[(y*w+x)*3+ch] = v }
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			for ch := 0; ch < 3; ch++ {
				old := at(x, y, ch)
				nw := float64(int(old + 0.5))
				if nw < 0 {
					nw = 0
				}
				if nw > 255 {
					nw = 255
				}
				set(x, y, ch, nw)
				err := old - nw
				if x+1 < w {
					set(x+1, y, ch, at(x+1, y, ch)+err*7/16)
				}
				if y+1 < h {
					if x > 0 {
						set(x-1, y+1, ch, at(x-1, y+1, ch)+err*3/16)
					}
					set(x, y+1, ch, at(x, y+1, ch)+err*5/16)
					if x+1 < w {
						set(x+1, y+1, ch, at(x+1, y+1, ch)+err*1/16)
					}
				}
			}
		}
	}
}
