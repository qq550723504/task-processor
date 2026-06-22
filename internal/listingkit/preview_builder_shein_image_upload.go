package listingkit

import (
	"strings"

	sheinworkspace "task-processor/internal/marketplace/shein/workspace"
	sheinpub "task-processor/internal/publishing/shein"
)

func buildSheinImageUploadPreflight(pkg *SheinPackage) *SheinImageUploadPreflight {
	return sheinworkspace.BuildImageUploadPreflight(
		pkg,
		sheinpub.IsUploadedImageURL,
		func(pkg *SheinPackage, sourceURL string) bool {
			uploadedURL := strings.TrimSpace(sheinImageUploadCache(pkg)[strings.TrimSpace(sourceURL)])
			return uploadedURL != "" && sheinpub.IsUploadedImageURL(uploadedURL)
		},
		sheinpub.IsSDSImageURL,
	)
}
