package cover

import (
	"image"
	"image/color"
	"testing"
)

func testImg(w, h int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetRGBA(x, y, color.RGBA{
				R: uint8((x*60 + 40) % 256),
				G: uint8((y*50 + 80) % 256),
				B: uint8(((x+y)*30 + 100) % 256),
				A: 255,
			})
		}
	}
	return img
}

func TestSepiaWarmer(t *testing.T) {
	src := testImg(5, 5)
	out, ok := Sepia(src, false).(*image.RGBA)
	if !ok {
		t.Fatal("Sepia must return *image.RGBA")
	}
	c := out.RGBAAt(2, 2)
	s := src.RGBAAt(2, 2)
	if !(c.R > c.G && c.G > c.B) {
		t.Fatalf("center pixel not warm r>g>b: %+v", c)
	}
	if c.R < s.R {
		t.Fatalf("r did not grow: in=%+v out=%+v", s, c)
	}
	// input not mutated
	if src.RGBAAt(2, 2) != s {
		t.Fatal("input mutated")
	}
}

func TestSepiaVignette(t *testing.T) {
	src := testImg(9, 9)
	plain := Sepia(src, false).(*image.RGBA)
	vig := Sepia(src, true).(*image.RGBA)
	lum := func(c color.RGBA) int { return int(c.R) + int(c.G) + int(c.B) }
	// corner darkens relative to plain
	if lum(vig.RGBAAt(0, 0)) >= lum(plain.RGBAAt(0, 0)) {
		t.Fatal("vignette corner not darker than plain")
	}
	// vignette corner relatively darker than vignette center vs plain ratio
	rc := float64(lum(vig.RGBAAt(0, 0))+1) / float64(lum(plain.RGBAAt(0, 0))+1)
	rm := float64(lum(vig.RGBAAt(4, 4))+1) / float64(lum(plain.RGBAAt(4, 4))+1)
	if rc >= rm {
		t.Fatalf("corner ratio %v not < center ratio %v", rc, rm)
	}
}

func TestDuotoneGreen(t *testing.T) {
	src := testImg(4, 4)
	out, ok := DuotoneGreen(src).(*image.RGBA)
	if !ok {
		t.Fatal("DuotoneGreen must return *image.RGBA")
	}
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			c := out.RGBAAt(x, y)
			if !(c.G >= c.R && c.G >= c.B) {
				t.Fatalf("pixel (%d,%d) not green-dominant: %+v", x, y, c)
			}
		}
	}
}

func TestGrayscale(t *testing.T) {
	src := testImg(4, 4)
	out, ok := Grayscale(src).(*image.RGBA)
	if !ok {
		t.Fatal("Grayscale must return *image.RGBA")
	}
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			c := out.RGBAAt(x, y)
			if !(c.R == c.G && c.G == c.B) {
				t.Fatalf("pixel (%d,%d) not gray: %+v", x, y, c)
			}
		}
	}
}

func TestApplyGrade(t *testing.T) {
	src := testImg(4, 4)
	if got := ApplyGrade(src, GradeNone); got != image.Image(src) {
		t.Fatal("GradeNone must return same pointer")
	}
	for _, g := range []Grade{GradeSepia, GradePhosphor, GradeMono} {
		got := ApplyGrade(src, g)
		if got == image.Image(src) {
			t.Fatalf("grade %d returned same pointer", g)
		}
		// must differ from input on at least one pixel
		same := true
		for y := 0; y < 4 && same; y++ {
			for x := 0; x < 4 && same; x++ {
				if got.At(x, y) != src.At(x, y) {
					same = false
				}
			}
		}
		if same {
			t.Fatalf("grade %d identical to input", g)
		}
	}
}

func TestGradeNilEmpty(t *testing.T) {
	for _, g := range []Grade{GradeNone, GradeSepia, GradePhosphor, GradeMono} {
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("ApplyGrade(nil,%d) panicked: %v", g, r)
				}
			}()
			_ = ApplyGrade(nil, g)
		}()
	}
	empty := image.NewRGBA(image.Rect(0, 0, 0, 0))
	funcs := []func() image.Image{
		func() image.Image { return Sepia(empty, false) },
		func() image.Image { return Sepia(empty, true) },
		func() image.Image { return DuotoneGreen(empty) },
		func() image.Image { return Grayscale(empty) },
		func() image.Image { return ApplyGrade(empty, GradeSepia) },
	}
	for i, f := range funcs {
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("func %d on empty panicked: %v", i, r)
				}
			}()
			_ = f()
		}()
	}
}
