package studio

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"
)

func TestValidateTransparentPNGAcceptsAnImageWithTransparentPixels(t *testing.T) {
	if err := ValidateTransparentPNG(testPNG(t, color.RGBA{R: 255, A: 128})); err != nil {
		t.Fatalf("ValidateTransparentPNG() error = %v", err)
	}
}

func TestValidateTransparentPNGRejectsOpaqueOrInvalidData(t *testing.T) {
	if err := ValidateTransparentPNG(testPNG(t, color.RGBA{R: 255, A: 255})); err == nil {
		t.Fatal("ValidateTransparentPNG(opaque) error = nil")
	}
	if err := ValidateTransparentPNG([]byte("not-png")); err == nil {
		t.Fatal("ValidateTransparentPNG(non-png) error = nil")
	}
}

func testPNG(t *testing.T, pixel color.RGBA) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img.SetRGBA(0, 0, pixel)
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("png.Encode() error = %v", err)
	}
	return buf.Bytes()
}
