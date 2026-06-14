package listingkit

import sheinworkspace "task-processor/internal/listingkit/workspace/shein"

func buildSheinImageUploadPreflight(pkg *SheinPackage) *SheinImageUploadPreflight {
	return sheinworkspace.BuildImageUploadPreflight(
		pkg,
		isSheinUploadedImageURL,
		sheinImageUploadCacheHit,
		isSDSImageURL,
	)
}
