package productimage

import (
	"image"
	"image/color"

	"github.com/disintegration/imaging"
)

func extractPrimarySubject(img image.Image) (image.Image, image.Rectangle) {
	if img == nil {
		return nil, image.Rectangle{}
	}
	bounds := img.Bounds()
	bg := estimateBackgroundColor(img)
	tolerance := uint32(28)

	minX, minY := bounds.Max.X, bounds.Max.Y
	maxX, maxY := bounds.Min.X, bounds.Min.Y
	found := false

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			if !isNearBackground(img.At(x, y), bg, tolerance) {
				if x < minX {
					minX = x
				}
				if y < minY {
					minY = y
				}
				if x > maxX {
					maxX = x
				}
				if y > maxY {
					maxY = y
				}
				found = true
			}
		}
	}

	if !found {
		return img, bounds
	}

	paddingX := max((maxX-minX+1)/12, 12)
	paddingY := max((maxY-minY+1)/12, 12)
	rect := image.Rect(
		max(bounds.Min.X, minX-paddingX),
		max(bounds.Min.Y, minY-paddingY),
		min(bounds.Max.X, maxX+paddingX+1),
		min(bounds.Max.Y, maxY+paddingY+1),
	)
	if rect.Empty() {
		return img, bounds
	}
	return imaging.Crop(img, rect), rect
}

func estimateBackgroundColor(img image.Image) color.NRGBA {
	b := img.Bounds()
	points := []image.Point{
		{X: b.Min.X + 1, Y: b.Min.Y + 1},
		{X: b.Max.X - 2, Y: b.Min.Y + 1},
		{X: b.Min.X + 1, Y: b.Max.Y - 2},
		{X: b.Max.X - 2, Y: b.Max.Y - 2},
		{X: b.Min.X + b.Dx()/2, Y: b.Min.Y + 1},
		{X: b.Min.X + b.Dx()/2, Y: b.Max.Y - 2},
	}

	var sr, sg, sb, sa uint32
	count := uint32(0)
	for _, pt := range points {
		if !pt.In(b) {
			continue
		}
		r, g, bl, a := img.At(pt.X, pt.Y).RGBA()
		sr += r >> 8
		sg += g >> 8
		sb += bl >> 8
		sa += a >> 8
		count++
	}
	if count == 0 {
		return color.NRGBA{R: 255, G: 255, B: 255, A: 255}
	}
	return color.NRGBA{
		R: uint8(sr / count),
		G: uint8(sg / count),
		B: uint8(sb / count),
		A: uint8(sa / count),
	}
}

func isNearBackground(c color.Color, bg color.NRGBA, tolerance uint32) bool {
	r, g, b, _ := c.RGBA()
	dr := absDiff(r>>8, uint32(bg.R))
	dg := absDiff(g>>8, uint32(bg.G))
	db := absDiff(b>>8, uint32(bg.B))
	return dr <= tolerance && dg <= tolerance && db <= tolerance
}

func absDiff(a, b uint32) uint32 {
	if a > b {
		return a - b
	}
	return b - a
}
