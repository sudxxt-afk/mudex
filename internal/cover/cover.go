// Package cover renders images to terminal cells (half-block) and
// provides neon ASCII fallback when no cover art exists.
// Works on every Linux terminal: only UTF-8 blocks + ANSI colors, no Kitty/Sixel.
package cover

import (
	"bytes"
	"fmt"
	"hash/fnv"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"
	"strings"

	_ "golang.org/x/image/webp"

	id3v2 "github.com/bogem/id3v2/v2"
	"golang.org/x/image/draw"
)

// TrueColor reports whether the terminal likely supports 24-bit color.
func TrueColor() bool {
	for _, k := range []string{"COLORTERM"} {
		v := os.Getenv(k)
		if v == "truecolor" || v == "24bit" {
			return true
		}
	}
	return false
}

// FindFileCover looks for cover.jpg/png/webp, folder.jpg, front.jpg, etc. next to trackPath.
func FindFileCover(trackPath string) string {
	dir := filepath.Dir(trackPath)
	candidates := []string{
		"cover.jpg", "cover.jpeg", "cover.png", "cover.webp",
		"folder.jpg", "folder.png", "front.jpg", "front.png",
		"artwork.jpg", "album.jpg",
	}
	for _, c := range candidates {
		p := filepath.Join(dir, c)
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p
		}
	}
	return ""
}

// ExtractEmbedded tries to read embedded art: ID3v2 APIC for mp3, sidecar already handled elsewhere.
// Returns raw image bytes or nil.
func ExtractEmbedded(trackPath string) []byte {
	ext := strings.ToLower(filepath.Ext(trackPath))
	if ext != ".mp3" {
		return nil // FLAC/OGG embedded parsing: best-effort via sidecar; keep pure-Go small
	}
	f, err := os.Open(trackPath)
	if err != nil {
		return nil
	}
	defer f.Close()
	tag, err := id3v2.ParseReader(f, id3v2.Options{Parse: true})
	if err != nil || tag == nil {
		return nil
	}
	defer tag.Close()
	frames := tag.GetFrames(tag.CommonID("Attached picture"))
	for _, fr := range frames {
		if pic, ok := fr.(id3v2.PictureFrame); ok {
			if len(pic.Picture) > 0 {
				return pic.Picture
			}
		}
	}
	return nil
}

// LoadCover returns image bytes for track: embedded first, then sidecar file.
func LoadCover(trackPath string) ([]byte, string) {
	if b := ExtractEmbedded(trackPath); len(b) > 0 {
		return b, "embedded"
	}
	if p := FindFileCover(trackPath); p != "" {
		if b, err := os.ReadFile(p); err == nil && len(b) > 0 {
			return b, p
		}
	}
	return nil, ""
}

// RenderHalfBlock scales img to width cells (each cell = 2 vertical pixels)
// and returns lines using '▀' with fg=top, bg=bottom. Applies Bayer 4x4 dithering
// when dither is true. If trueColor is false, colors are quantized to 256-color palette.
func RenderHalfBlock(img image.Image, width int, trueColor bool, dither bool) []string {
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
	// Cell aspect: terminal cell ~ 1:2 (w:h). To look square, pixel height = width*2 * (srcH/srcW).
	heightPx := int(float64(width*2) * float64(srcH) / float64(srcW))
	if heightPx < 4 {
		heightPx = 4
	}
	if heightPx > 160 {
		heightPx = 160
	}
	dst := image.NewRGBA(image.Rect(0, 0, width, heightPx))
	draw.CatmullRom.Scale(dst, dst.Bounds(), img, b, draw.Over, nil)

	lines := make([]string, 0, (heightPx+1)/2)
	var sb strings.Builder
	bayer := [4][4]int{{0, 8, 2, 10}, {12, 4, 14, 6}, {3, 11, 1, 9}, {15, 7, 13, 5}}
	for y := 0; y < heightPx; y += 2 {
		sb.Reset()
		for x := 0; x < width; x++ {
			top := dst.RGBAAt(x, y)
			var botR, botG, botB, botA uint32
			if y+1 < heightPx {
				bot := dst.RGBAAt(x, y+1)
				botR, botG, botB, botA = uint32(bot.R), uint32(bot.G), uint32(bot.B), uint32(bot.A)
			} else {
				botR, botG, botB, botA = 0, 0, 0, 255
			}
			tr, tg, tb := uint32(top.R), uint32(top.G), uint32(top.B)
			if dither {
				n := bayer[y%4][x%4] - 8 // -8..7
				tr = clamp(int(tr) + n*2)
				tg = clamp(int(tg) + n*2)
				tb = clamp(int(tb) + n*2)
				botR = clamp(int(botR) + n*2)
				botG = clamp(int(botG) + n*2)
				botB = clamp(int(botB) + n*2)
			}
			_ = botA
			if trueColor {
				fmt.Fprintf(&sb, "\x1b[38;2;%d;%d;%dm\x1b[48;2;%d;%d;%dm▀", tr, tg, tb, botR, botG, botB)
			} else {
				fmt.Fprintf(&sb, "\x1b[38;5;%dm\x1b[48;5;%dm▀", rgbTo256(int(tr), int(tg), int(tb)), rgbTo256(int(botR), int(botG), int(botB)))
			}
		}
		sb.WriteString("\x1b[0m")
		lines = append(lines, sb.String())
	}
	return lines
}

// RenderImageBytes decodes raw bytes and renders, or returns nil on failure.
func RenderImageBytes(data []byte, width int, trueColor bool) []string {
	if len(data) == 0 {
		return nil
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil
	}
	return RenderHalfBlock(img, width, trueColor, true)
}

// RenderBest decodes raw bytes and renders with the best available method:
// sextants (2x3 px/cell) when the font supports them, half-blocks otherwise.
func RenderBest(data []byte, width, maxRows int, trueColor bool) []string {
	if len(data) == 0 {
		return nil
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil
	}
	if SextantsOK() {
		return RenderSextants(img, width, maxRows, trueColor)
	}
	return RenderHalfBlock(img, width, trueColor, true)
}

func clamp(v int) uint32 {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return uint32(v)
}

// rgbTo256 maps 24-bit to xterm 256 palette (16+216+24).
func rgbTo256(r, g, b int) int {
	if r == g && g == b {
		if r < 8 {
			return 16
		}
		if r > 248 {
			return 231
		}
		return 232 + (r-8)*24/247
	}
	return 16 + (r*6/256)*36 + (g*6/256)*6 + (b * 6 / 256)
}

// --- Neon ASCII fallback ---

var asciiArts = [][]string{
	{
		"   ♪    ♫   ",
		"  ┌──────┐  ",
		"  │ (◉) │♪ ",
		"  │ \\_／ │  ",
		"  └──────┘  ",
		" ♫  MUSIC ♪ ",
	},
	{
		"  .-\"\"\"-.  ",
		" / .--. \\ ",
		"| (◕‿◕) |",
		" \\ '--' / ",
		"  '--'--'  ",
		" VINYL VIBE",
	},
	{
		" ▄▀▄▀▄▀▄▀▄ ",
		" █ NEON █ ",
		" █ BEATS █ ",
		" ▀▄▀▄▀▄▀▄▀ ",
		" ♪ ━━━ ♫ ",
	},
	{
		"  /\\_/\\   ",
		" ( o.o )♪ ",
		"  > ^ <  ♫",
		" /|   |\\  ",
		"  NO COVER ",
	},
	{
		" .·:*¨¨*:·. ",
		" ♪ ┌───┐ ♫ ",
		"   │♪♪♪│   ",
		"   └───┘   ",
		" ·:..·:..· ",
	},
}

// FallbackArt picks a deterministic ASCII art from track name hash.
func FallbackArt(trackName string, width int) []string {
	h := fnv.New32a()
	h.Write([]byte(trackName))
	art := asciiArts[h.Sum32()%uint32(len(asciiArts))]
	out := make([]string, len(art))
	for i, line := range art {
		if len(line) > width {
			out[i] = line[:width]
		} else {
			out[i] = line + strings.Repeat(" ", width-len(line))
		}
	}
	return out
}

// Neonize wraps lines in a neon color gradient (cyan->magenta) using ANSI.
func Neonize(lines []string, seed int64) []string {
	colors := []string{"\x1b[38;2;0;255;255m", "\x1b[38;2;80;180;255m", "\x1b[38;2;180;120;255m", "\x1b[38;2;255;80;220m"}
	out := make([]string, len(lines))
	for i, l := range lines {
		c := colors[(i+int(seed))%len(colors)]
		out[i] = c + l + "\x1b[0m"
	}
	return out
}
