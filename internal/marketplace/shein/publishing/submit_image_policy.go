package publishing

import "strings"

// FinalSubmitImagesRequireSKC reports whether final submit image readiness must
// include SKC/swatch evidence for the action.
func FinalSubmitImagesRequireSKC(action string) bool {
	return FinalReviewRequired(action)
}

// IsUploadedImageURL reports whether url already points at a SHEIN-hosted image.
func IsUploadedImageURL(url string) bool {
	value := strings.ToLower(strings.TrimSpace(url))
	return strings.Contains(value, "shein.com") ||
		strings.Contains(value, "sheinimg.com") ||
		strings.Contains(value, "ltwebstatic.com")
}

// IsSDSImageURL reports whether url points at an SDS-generated/source image host.
func IsSDSImageURL(url string) bool {
	value := strings.ToLower(strings.TrimSpace(url))
	return strings.Contains(value, "sdspod.com") || strings.Contains(value, "sdsdiy.com")
}

// CloneImageUploadCache normalizes and filters source-to-uploaded image URL cache entries.
func CloneImageUploadCache(input map[string]string) map[string]string {
	if len(input) == 0 {
		return map[string]string{}
	}
	out := make(map[string]string, len(input))
	for sourceURL, uploadedURL := range input {
		sourceURL = strings.TrimSpace(sourceURL)
		uploadedURL = strings.TrimSpace(uploadedURL)
		if sourceURL == "" || uploadedURL == "" || !IsUploadedImageURL(uploadedURL) {
			continue
		}
		out[sourceURL] = uploadedURL
	}
	return out
}
