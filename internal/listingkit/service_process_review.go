package listingkit

import (
	"strings"
)

func taskNeedsReviewReason(result *ListingKitResult) string {
	warnings := reviewReasonsFromResult(result)
	if len(warnings) == 0 {
		return "listing kit requires review"
	}
	return strings.Join(warnings, "; ")
}
