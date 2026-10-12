package cover

import (
	"image"
	"image/color"
	"math"
)

// Grade selects a color treatment for cover art.
type Grade int

const (
	GradeNone Grade = iota
	GradeSepia
	GradePhosphor
	GradeMono
)

// ApplyGrade returns img unchanged for GradeNone, otherwise the graded copy.
func ApplyGrade(img image.Image, g Grade) image.Image {
	if img == nil {
		return image.NewRGBA(image.Rect(0, 0, 0, 0))
	}
	switch g {
	case GradeSepia:
		return Sepia(img, false)
	case GradePhosphor:
		return DuotoneGreen(img)
	case GradeMono:
		return Grayscale(img)
	default:
		return img
	}
}

func clamp8(v float64) uint8 {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return uint8(math.Round(v))
}

// lum709 returns Rec.709 luminance in [0,255].
func lum709(r, g, b uint8) float64 {
	return 0.2126*float64(r) + 0.7152*float64(g) + 0.0722*float64(b)
}

// Sepia applies the classic sepia matrix; with vignette it darkens toward
// the corners (multiplier down to ~0.55 in corners, smooth falloff).
func Sepia(img image.Image, vignette bool) image.Image {
	if img == nil {
		return image.NewRGBA(image.Rect(0, 0, 0, 0))
	}
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	if w == 0 || h == 0 {
		return dst
	}
	cx, cy := float64(w-1)/2, float64(h-1)/2
	maxD := math.Hypot(cx, cy)
	if maxD == 0 {
		maxD = 1
	}
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r32, g32, b32, a32 := img.At(x, y).RGBA()
			r, g, bl := float64(uint8(r32>>8)), float64(uint8(g32>>8)), float64(uint8(b32>>8))
			nr := 0.393*r + 0.769*g + 0.189*bl
			ng := 0.349*r + 0.686*g + 0.168*bl
			nb := 0.272*r + 0.534*g + 0.131*bl
			if vignette {
				d := math.Hypot(float64(x-b.Min.X)-cx, float64(y-b.Min.Y)-cy) / maxD // 0..1
				m := 1.0 - 0.45*d*d
				if m < 0.55 {
					m = 0.55
				}
				nr *= m
				ng *= m
				nb *= m
			}
			dst.SetRGBA(x-b.Min.X, y-b.Min.Y, color.RGBA{
				R: clamp8(nr),
				G: clamp8(ng),
				B: clamp8(nb),
				A: uint8(a32 >> 8),
			})
		}
	}
	return dst
}

// DuotoneGreen maps luminance from black to phosphor green (51,255,51).
func DuotoneGreen(img image.Image) image.Image {
	if img == nil {
		return image.NewRGBA(image.Rect(0, 0, 0, 0))
	}
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	if w == 0 || h == 0 {
		return dst
	}
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r32, g32, b32, a32 := img.At(x, y).RGBA()
			l := lum709(uint8(r32>>8), uint8(g32>>8), uint8(b32>>8)) / 255.0
			dst.SetRGBA(x-b.Min.X, y-b.Min.Y, color.RGBA{
				R: clamp8(51 * l),
				G: clamp8(255 * l),
				B: clamp8(51 * l),
				A: uint8(a32 >> 8),
			})
		}
	}
	return dst
}

// Grayscale maps pixels to Rec.709 luminance gray.
func Grayscale(img image.Image) image.Image {
	if img == nil {
		return image.NewRGBA(image.Rect(0, 0, 0, 0))
	}
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	if w == 0 || h == 0 {
		return dst
	}
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r32, g32, b32, a32 := img.At(x, y).RGBA()
			l := clamp8(lum709(uint8(r32>>8), uint8(g32>>8), uint8(b32>>8)))
			dst.SetRGBA(x-b.Min.X, y-b.Min.Y, color.RGBA{
				R: l,
				G: l,
				B: l,
				A: uint8(a32 >> 8),
			})
		}
	}
	return dst
}
