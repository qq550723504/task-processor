package design

import (
	"context"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"math"
	"net/http"
	"strings"
	"time"
)

const blankRenderedDiffThreshold = 0.035

func (s *Service) renderedCandidateLooksBlank(ctx context.Context, urls []string, blankURL string) bool {
	renderedURL := firstRenderedURL(urls)
	blankURL = strings.TrimSpace(blankURL)
	if renderedURL == "" || blankURL == "" {
		return false
	}
	rendered, ok := downloadImageForComparison(ctx, renderedURL)
	if !ok {
		return false
	}
	blank, ok := downloadImageForComparison(ctx, blankURL)
	if !ok {
		return false
	}
	diff := normalizedRenderedImageDiff(rendered, blank, 96, 96)
	return diff >= 0 && diff < blankRenderedDiffThreshold
}

func firstRenderedURL(urls []string) string {
	for _, value := range urls {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func downloadImageForComparison(ctx context.Context, imageURL string) (image.Image, bool) {
	imageURL = strings.TrimSpace(imageURL)
	if imageURL == "" {
		return nil, false
	}
	reqCtx, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, imageURL, nil)
	if err != nil {
		return nil, false
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, false
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, false
	}
	img, _, err := image.Decode(resp.Body)
	if err != nil {
		return nil, false
	}
	return img, true
}

func normalizedRenderedImageDiff(a, b image.Image, width, height int) float64 {
	if a == nil || b == nil || width <= 0 || height <= 0 {
		return -1
	}
	var total float64
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			ar, ag, ab, _ := a.At(sampleImageX(a.Bounds(), x, width), sampleImageY(a.Bounds(), y, height)).RGBA()
			br, bg, bb, _ := b.At(sampleImageX(b.Bounds(), x, width), sampleImageY(b.Bounds(), y, height)).RGBA()
			total += math.Abs(float64(ar)-float64(br)) / 65535
			total += math.Abs(float64(ag)-float64(bg)) / 65535
			total += math.Abs(float64(ab)-float64(bb)) / 65535
		}
	}
	return total / float64(width*height*3)
}

func sampleImageX(bounds image.Rectangle, x, width int) int {
	if width <= 1 {
		return bounds.Min.X
	}
	return bounds.Min.X + x*(bounds.Dx()-1)/(width-1)
}

func sampleImageY(bounds image.Rectangle, y, height int) int {
	if height <= 1 {
		return bounds.Min.Y
	}
	return bounds.Min.Y + y*(bounds.Dy()-1)/(height-1)
}
