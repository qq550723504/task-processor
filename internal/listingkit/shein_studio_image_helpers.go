package listingkit

import "strings"

func appendUniqueImageURLs(existing []string, additions ...string) []string {
	result := append([]string(nil), existing...)
	seen := map[string]struct{}{}
	for _, imageURL := range result {
		imageURL = strings.TrimSpace(imageURL)
		if imageURL != "" {
			seen[imageURL] = struct{}{}
		}
	}
	for _, imageURL := range additions {
		imageURL = strings.TrimSpace(imageURL)
		if imageURL == "" {
			continue
		}
		if _, ok := seen[imageURL]; ok {
			continue
		}
		seen[imageURL] = struct{}{}
		result = append(result, imageURL)
	}
	return result
}
