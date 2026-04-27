package listingkit

import (
	"context"
	"image"
	"math"
	"strings"

	"github.com/disintegration/imaging"
)

const sdsBlankRenderDiffThreshold = 0.035

func sdsRenderedLooksBlank(ctx context.Context, summary *SDSSyncSummary, options *SDSSyncOptions) bool {
	if summary == nil || options == nil {
		return false
	}
	renderedURL := firstNonEmptyURL(summary.MockupImageURLs)
	blankURL := strings.TrimSpace(options.BlankDesignURL)
	if renderedURL == "" || blankURL == "" {
		return false
	}

	rendered, err := downloadImageForComposite(ctx, renderedURL)
	if err != nil {
		return false
	}
	blank, err := downloadImageForComposite(ctx, blankURL)
	if err != nil {
		return false
	}
	return normalizedImageDiff(rendered, blank) < sdsBlankRenderDiffThreshold
}

func normalizedImageDiff(a image.Image, b image.Image) float64 {
	if a == nil || b == nil {
		return 1
	}
	const size = 96
	left := imaging.Resize(a, size, size, imaging.Lanczos)
	right := imaging.Resize(b, size, size, imaging.Lanczos)
	var total float64
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			lr, lg, lb, _ := left.At(x, y).RGBA()
			rr, rg, rb, _ := right.At(x, y).RGBA()
			total += math.Abs(float64(lr)-float64(rr)) / 65535
			total += math.Abs(float64(lg)-float64(rg)) / 65535
			total += math.Abs(float64(lb)-float64(rb)) / 65535
		}
	}
	return total / float64(size*size*3)
}

func firstNonEmptyURL(values []string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
