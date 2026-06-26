package publishing

import (
	"strings"

	sheinproduct "task-processor/internal/shein/api/product"
)

// DedupeImagesByURL keeps the first non-empty image for each image URL.
func DedupeImagesByURL(images []sheinproduct.ImageDetail) []sheinproduct.ImageDetail {
	seen := map[string]bool{}
	result := make([]sheinproduct.ImageDetail, 0, len(images))
	for _, image := range images {
		url := strings.TrimSpace(image.ImageURL)
		if url == "" || seen[url] {
			continue
		}
		seen[url] = true
		result = append(result, image)
	}
	return result
}

// NormalizeSubmitSKUImageDetail normalizes one SKU image detail for SHEIN submit.
func NormalizeSubmitSKUImageDetail(image sheinproduct.ImageDetail) sheinproduct.ImageDetail {
	image.ImageType = 1
	image.ImageSort = 1
	image.MarketingMainImage = false
	image.SizeImgFlag = false
	image.TransformCVSizeImage = false
	if image.PSTypes == nil {
		image.PSTypes = []string{}
	}
	return image
}

// NormalizeSubmitGalleryImages normalizes gallery image types, ordering, square image, and optional color block.
func NormalizeSubmitGalleryImages(images []sheinproduct.ImageDetail, includeColorBlock bool) []sheinproduct.ImageDetail {
	source := DedupeImagesByURL(images)
	if len(source) == 0 {
		return nil
	}
	colorBlockSource := source[0]
	for _, image := range source {
		if image.ImageType == 6 && !image.SizeImgFlag && strings.TrimSpace(image.ImageURL) != "" {
			colorBlockSource = image
			break
		}
	}
	gallerySource := make([]sheinproduct.ImageDetail, 0, len(source))
	for _, image := range source {
		if image.ImageType == 6 && !image.SizeImgFlag {
			continue
		}
		gallerySource = append(gallerySource, image)
	}
	if len(gallerySource) == 0 {
		gallerySource = []sheinproduct.ImageDetail{source[0]}
	}
	extraCapacity := 1
	if includeColorBlock {
		extraCapacity = 2
	}
	normalized := make([]sheinproduct.ImageDetail, 0, len(gallerySource)+extraCapacity)
	for index, image := range gallerySource {
		image.ImageType = 2
		if index == 0 {
			image.ImageType = 1
		}
		image.ImageSort = index + 1
		image.MarketingMainImage = false
		image.SizeImgFlag = false
		image.TransformCVSizeImage = false
		if image.PSTypes == nil {
			image.PSTypes = []string{}
		}
		normalized = append(normalized, image)
	}
	square := gallerySource[0]
	square.ImageType = 5
	square.ImageSort = len(normalized) + 1
	square.MarketingMainImage = false
	square.SizeImgFlag = false
	square.TransformCVSizeImage = false
	if square.PSTypes == nil {
		square.PSTypes = []string{}
	}
	normalized = append(normalized, square)
	if !includeColorBlock {
		return normalized
	}
	colorBlock := colorBlockSource
	colorBlock.ImageType = 6
	colorBlock.ImageSort = len(normalized) + 1
	colorBlock.MarketingMainImage = false
	colorBlock.SizeImgFlag = false
	colorBlock.TransformCVSizeImage = false
	if colorBlock.PSTypes == nil {
		colorBlock.PSTypes = []string{}
	}
	normalized = append(normalized, colorBlock)
	return normalized
}
