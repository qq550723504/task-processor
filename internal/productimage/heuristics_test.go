package productimage

import (
	"image"
	"image/color"
	"testing"
)

func TestLooksWhiteBackground(t *testing.T) {
	t.Parallel()

	whiteBg := image.NewRGBA(image.Rect(0, 0, 50, 50))
	for y := 0; y < 50; y++ {
		for x := 0; x < 50; x++ {
			whiteBg.Set(x, y, color.NRGBA{R: 245, G: 245, B: 245, A: 255})
		}
	}
	if !looksWhiteBackground(whiteBg) {
		t.Fatal("expected white background image to be detected as white")
	}

	darkBg := image.NewRGBA(image.Rect(0, 0, 50, 50))
	for y := 0; y < 50; y++ {
		for x := 0; x < 50; x++ {
			darkBg.Set(x, y, color.NRGBA{R: 40, G: 40, B: 40, A: 255})
		}
	}
	if looksWhiteBackground(darkBg) {
		t.Fatal("expected dark background image to not be detected as white")
	}
}

func TestExtractPrimarySubject(t *testing.T) {
	t.Parallel()

	base := image.NewRGBA(image.Rect(0, 0, 200, 200))
	for y := 0; y < 200; y++ {
		for x := 0; x < 200; x++ {
			base.Set(x, y, color.NRGBA{R: 250, G: 250, B: 250, A: 255})
		}
	}
	for y := 60; y < 140; y++ {
		for x := 70; x < 130; x++ {
			base.Set(x, y, color.NRGBA{R: 30, G: 50, B: 120, A: 255})
		}
	}

	cropped, rect := extractPrimarySubject(base)
	if cropped == nil {
		t.Fatal("expected cropped image")
	}
	if rect.Dx() >= 200 || rect.Dy() >= 200 {
		t.Fatalf("expected subject crop to be tighter than source, got rect=%v", rect)
	}
	if rect.Min.X > 70 || rect.Min.Y > 60 || rect.Max.X < 130 || rect.Max.Y < 140 {
		t.Fatalf("expected crop rect %v to still cover the subject region", rect)
	}
}
