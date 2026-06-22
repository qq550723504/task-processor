package listingkit

import sheinworkspace "task-processor/internal/marketplace/shein/workspace"

func buildSheinImageUploadPreflight(pkg *SheinPackage) *SheinImageUploadPreflight {
	return sheinworkspace.BuildImageUploadPreflight(
		pkg,
		isSheinUploadedImageURL,
		sheinImageUploadCacheHit,
		isSDSImageURL,
	)
}
