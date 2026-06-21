package listingkit

import (
	common "task-processor/internal/publishing/common"
	sheinpub "task-processor/internal/publishing/shein"
)

func replaceSheinImagesWithAIProductImages(pkg *sheinpub.Package, imageURLs []string, sourceImages []string) {
	sheinpub.ReplaceImagesWithAIProductImages(pkg, imageURLs, sourceImages)
}

func appendAIProductImagesToShein(pkg *sheinpub.Package, imageURLs []string, sourceImages []string) {
	sheinpub.AppendAIProductImages(pkg, imageURLs, sourceImages)
}

func imageSetFromAIProductImages(imageURLs []string, sourceImages []string) *common.ImageSet {
	return sheinpub.ImageSetFromAIProductImages(imageURLs, sourceImages)
}

func imageDraftToSet(draft *sheinpub.ImageDraft) *common.ImageSet {
	return sheinpub.ImageDraftToSet(draft)
}
