package studio

import (
	"bytes"
	"fmt"
	"image/color"
	"image/png"
)

// ValidateTransparentPNG verifies that PNG data decodes to an image with an
// alpha-capable color model and at least one actually transparent pixel.
func ValidateTransparentPNG(data []byte) error {
	if len(data) == 0 {
		return fmt.Errorf("transparent image is empty")
	}
	decoded, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("decode transparent PNG: %w", err)
	}
	switch decoded.ColorModel() {
	case color.RGBAModel, color.NRGBAModel, color.RGBA64Model, color.NRGBA64Model,
		color.AlphaModel, color.Alpha16Model, color.NYCbCrAModel:
		// Continue below to verify that at least one pixel is actually transparent.
	default:
		return fmt.Errorf("transparent image does not expose alpha")
	}
	for y := decoded.Bounds().Min.Y; y < decoded.Bounds().Max.Y; y++ {
		for x := decoded.Bounds().Min.X; x < decoded.Bounds().Max.X; x++ {
			_, _, _, alpha := decoded.At(x, y).RGBA()
			if alpha < 0xffff {
				return nil
			}
		}
	}
	return fmt.Errorf("transparent image contains no transparent pixels")
}
