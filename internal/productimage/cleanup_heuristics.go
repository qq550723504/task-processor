package productimage

import (
	"context"
	"image"

	"task-processor/internal/pkg/watermark"
)

func (c *downloadedImageCleaner) detectCleanupRegions(ctx context.Context, img image.Image, lowerSourceURL string) ([]*watermark.WatermarkRegion, bool) {
	if c.watermarkProcessor == nil {
		return c.syntheticOverlayRegions(img, lowerSourceURL), containsAny(lowerSourceURL, "text", "poster", "caption", "label", "desc", "promo", "sale", "discount", "coupon", "price", "badge", "logo", "watermark", "brandmark")
	}
	detection, err := c.watermarkProcessor.DetectOnly(ctx, img)
	if err == nil && detection != nil && len(detection.Regions) > 0 {
		return detection.Regions, true
	}
	regions := c.syntheticOverlayRegions(img, lowerSourceURL)
	return regions, len(regions) > 0
}

func (c *downloadedImageCleaner) syntheticOverlayRegions(img image.Image, lowerSourceURL string) []*watermark.WatermarkRegion {
	if img == nil {
		return nil
	}
	if !containsAny(lowerSourceURL, "text", "poster", "caption", "label", "desc", "promo", "sale", "discount", "coupon", "price", "badge", "logo", "watermark", "brandmark") {
		return nil
	}
	b := img.Bounds()
	width := b.Dx()
	height := b.Dy()
	regionW := max(width/5, 80)
	regionH := max(height/7, 50)
	if regionW > width {
		regionW = width
	}
	if regionH > height {
		regionH = height
	}
	return []*watermark.WatermarkRegion{
		{
			X:           b.Min.X,
			Y:           b.Min.Y,
			Width:       regionW,
			Height:      regionH,
			Type:        watermark.WatermarkTypeText,
			Position:    watermark.PositionTopLeft,
			Confidence:  0.7,
			Description: "synthetic overlay cleanup region",
		},
	}
}
