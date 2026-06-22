package listingkit

import (
	"strings"

	common "task-processor/internal/publishing/common"
	sheinpub "task-processor/internal/publishing/shein"
)

func imageSetFromSDSVariantOption(item SDSSyncVariantOption, sourceImages []string) *common.ImageSet {
	mockups := uniqueNonEmptyStrings(item.MockupImageURLs)
	if len(mockups) == 0 {
		mockups = uniqueNonEmptyStrings([]string{item.MockupImageURL})
	}
	if len(mockups) == 0 {
		return nil
	}
	return sheinpub.ImageSetFromSDSMockups(mockups, sourceImages)
}

func imageSetFromSelectedSDSImages(items []SheinStudioSelectedSDSImage, sourceImages []string) *common.ImageSet {
	if len(items) == 0 {
		return nil
	}
	images := &common.ImageSet{
		MainImage:    items[0].ImageURL,
		SourceImages: uniqueNonEmptyStrings(sourceImages),
	}
	for _, item := range items[1:] {
		if imageURL := strings.TrimSpace(item.ImageURL); imageURL != "" {
			images.Gallery = append(images.Gallery, imageURL)
		}
	}
	return images
}

func normalizeSelectedSDSImages(input []SheinStudioSelectedSDSImage) []SheinStudioSelectedSDSImage {
	result := make([]SheinStudioSelectedSDSImage, 0, len(input))
	seen := map[string]struct{}{}
	for _, item := range input {
		imageURL := strings.TrimSpace(item.ImageURL)
		if imageURL == "" {
			continue
		}
		if _, ok := seen[imageURL]; ok {
			continue
		}
		seen[imageURL] = struct{}{}
		result = append(result, SheinStudioSelectedSDSImage{
			ImageURL:   imageURL,
			VariantSKU: strings.TrimSpace(item.VariantSKU),
			Color:      strings.TrimSpace(item.Color),
		})
	}
	return result
}
