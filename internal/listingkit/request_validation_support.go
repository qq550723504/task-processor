package listingkit

import (
	"fmt"
	"math"
	"strings"
)

func validateRequest(req *GenerateRequest) error {
	if len(req.ImageURLs) == 0 && strings.TrimSpace(req.Text) == "" && strings.TrimSpace(req.ProductURL) == "" {
		return fmt.Errorf("at least one of image_urls, text, or product_url must be provided")
	}
	if len(req.ImageURLs) > 10 {
		return fmt.Errorf("too many image URLs (max 10)")
	}
	if len(req.Platforms) == 0 {
		return fmt.Errorf("at least one platform is required")
	}
	if err := validateSheinStudioAspectRatio(req); err != nil {
		return err
	}
	return nil
}

func validateSheinStudioAspectRatio(req *GenerateRequest) error {
	if req == nil || req.Options == nil || req.Options.SheinStudio == nil || req.Options.SDS == nil {
		return nil
	}
	studio := req.Options.SheinStudio
	sds := req.Options.SDS
	if studio.SourceDesignWidth <= 0 || studio.SourceDesignHeight <= 0 || sds.PrintableWidth <= 0 || sds.PrintableHeight <= 0 {
		return nil
	}
	sourceRatio := float64(studio.SourceDesignWidth) / float64(studio.SourceDesignHeight)
	targetRatio := float64(sds.PrintableWidth) / float64(sds.PrintableHeight)
	if math.Abs(sourceRatio-targetRatio)/targetRatio > 0.25 {
		return fmt.Errorf("shein studio source image ratio differs too much from SDS printable area ratio")
	}
	return nil
}
