package listingkit

import (
	sheinpub "task-processor/internal/publishing/shein"
	sheinproduct "task-processor/internal/shein/api/product"
)

func applySheinSizeReferenceImages(pkg *sheinpub.Package, imageURLs []string) {
	sheinpub.ApplySizeReferenceImages(pkg, imageURLs)
}

func ensureSheinSizeReferenceDetails(info *sheinproduct.ImageInfo, refs []string) {
	sheinpub.EnsureSizeReferenceDetails(info, refs)
}
