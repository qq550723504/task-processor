package listingkit

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	_ "image/png"
	"net/http"
	"time"
)

const sheinColorBlockSize = 900

func buildSheinColorBlockImageFromURL(imageURL string) ([]byte, error) {
	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Get(imageURL)
	if err != nil {
		return nil, fmt.Errorf("download color source image: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("download color source image: status %d", resp.StatusCode)
	}
	img, _, err := image.Decode(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("decode color source image: %w", err)
	}
	return encodeSheinColorBlockImage(dominantSheinColor(img))
}

func encodeSheinColorBlockImage(fill color.Color) ([]byte, error) {
	out := image.NewRGBA(image.Rect(0, 0, sheinColorBlockSize, sheinColorBlockSize))
	for y := 0; y < sheinColorBlockSize; y++ {
		for x := 0; x < sheinColorBlockSize; x++ {
			out.Set(x, y, fill)
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, out, &jpeg.Options{Quality: 95}); err != nil {
		return nil, fmt.Errorf("encode color block image: %w", err)
	}
	return buf.Bytes(), nil
}

func dominantSheinColor(img image.Image) color.Color {
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()
	if width <= 0 || height <= 0 {
		return color.RGBA{R: 255, G: 255, B: 255, A: 255}
	}
	startX := bounds.Min.X + width/3
	endX := bounds.Min.X + width*2/3
	startY := bounds.Min.Y + height/3
	endY := bounds.Min.Y + height*2/3
	if startX >= endX {
		startX, endX = bounds.Min.X, bounds.Max.X
	}
	if startY >= endY {
		startY, endY = bounds.Min.Y, bounds.Max.Y
	}
	step := max(1, min(width, height)/200)
	counts := map[[3]uint8]int{}
	best := [3]uint8{255, 255, 255}
	bestCount := 0
	for y := startY; y < endY; y += step {
		for x := startX; x < endX; x += step {
			r, g, b, a := img.At(x, y).RGBA()
			if a < 32768 {
				continue
			}
			key := [3]uint8{uint8(r >> 8), uint8(g >> 8), uint8(b >> 8)}
			counts[key]++
			if counts[key] > bestCount {
				best = key
				bestCount = counts[key]
			}
		}
	}
	return color.RGBA{R: best[0], G: best[1], B: best[2], A: 255}
}
