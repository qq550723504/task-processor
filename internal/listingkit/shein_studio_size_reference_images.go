package listingkit

import sheinpub "task-processor/internal/publishing/shein"

func applySheinSizeReferenceImages(pkg *sheinpub.Package, imageURLs []string) {
	sheinpub.ApplySizeReferenceImages(pkg, imageURLs)
}
