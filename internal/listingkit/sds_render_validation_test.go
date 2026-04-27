package listingkit

import (
	"image"
	"image/color"
	"image/draw"
	"testing"
)

func TestNormalizedImageDiffDetectsNearIdenticalImages(t *testing.T) {
	t.Parallel()

	blank := image.NewRGBA(image.Rect(0, 0, 120, 120))
	draw.Draw(blank, blank.Bounds(), &image.Uniform{C: color.RGBA{R: 248, G: 248, B: 246, A: 255}}, image.Point{}, draw.Src)

	copyImage := image.NewRGBA(blank.Bounds())
	draw.Draw(copyImage, copyImage.Bounds(), blank, image.Point{}, draw.Src)

	if diff := normalizedImageDiff(blank, copyImage); diff >= sdsBlankRenderDiffThreshold {
		t.Fatalf("diff = %f, want below blank threshold", diff)
	}
}

func TestNormalizedImageDiffSeparatesDesignedImages(t *testing.T) {
	t.Parallel()

	blank := image.NewRGBA(image.Rect(0, 0, 120, 120))
	draw.Draw(blank, blank.Bounds(), &image.Uniform{C: color.RGBA{R: 248, G: 248, B: 246, A: 255}}, image.Point{}, draw.Src)

	designed := image.NewRGBA(blank.Bounds())
	draw.Draw(designed, designed.Bounds(), blank, image.Point{}, draw.Src)
	draw.Draw(designed, image.Rect(30, 30, 90, 90), &image.Uniform{C: color.RGBA{R: 210, G: 40, B: 120, A: 255}}, image.Point{}, draw.Over)

	if diff := normalizedImageDiff(blank, designed); diff <= sdsBlankRenderDiffThreshold {
		t.Fatalf("diff = %f, want above blank threshold", diff)
	}
}
