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
