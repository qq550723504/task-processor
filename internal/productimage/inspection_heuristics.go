package productimage

import "image"

func looksWhiteBackground(img image.Image) bool {
	if img == nil {
		return false
	}
	b := img.Bounds()
	points := []image.Point{
		{X: b.Min.X + 2, Y: b.Min.Y + 2},
		{X: b.Max.X - 3, Y: b.Min.Y + 2},
		{X: b.Min.X + 2, Y: b.Max.Y - 3},
		{X: b.Max.X - 3, Y: b.Max.Y - 3},
	}
	whiteCount := 0
	for _, pt := range points {
		r, g, b, _ := img.At(pt.X, pt.Y).RGBA()
		if r>>8 >= 240 && g>>8 >= 240 && b>>8 >= 240 {
			whiteCount++
		}
	}
	return whiteCount >= 3
}
