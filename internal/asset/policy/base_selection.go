package policy

import "task-processor/internal/asset"

// DeferredBaseKinds returns the ordered base kinds allowed when executing a
// planned generation task. The order is part of the generation contract: it
// prefers the most useful processed representation before falling back to the
// original source image.
func DeferredBaseKinds(kind asset.Kind) []asset.Kind {
	switch kind {
	case asset.KindModelImage:
		return []asset.Kind{asset.KindCleanImage, asset.KindMainImage, asset.KindSubjectCutout, asset.KindGalleryImage, asset.KindSourceImage}
	case asset.KindSellingPointImage, asset.KindSizeSceneImage, asset.KindDetailCrop:
		return []asset.Kind{asset.KindGalleryImage, asset.KindCleanImage, asset.KindMainImage, asset.KindSubjectCutout, asset.KindSourceImage}
	case asset.KindSceneImage:
		return []asset.Kind{asset.KindSceneImage, asset.KindGalleryImage, asset.KindCleanImage, asset.KindMainImage, asset.KindSubjectCutout, asset.KindSourceImage}
	default:
		return []asset.Kind{asset.KindCleanImage, asset.KindMainImage, asset.KindSubjectCutout, asset.KindSourceImage, asset.KindGalleryImage}
	}
}

// CandidateSourceKinds is the shared set of processed/source assets that may
// be offered as a generation task's source candidates.
func CandidateSourceKinds() []asset.Kind {
	return []asset.Kind{
		asset.KindSourceImage,
		asset.KindMainImage,
		asset.KindCleanImage,
		asset.KindSubjectCutout,
		asset.KindGalleryImage,
	}
}

func IsCandidateSourceKind(kind asset.Kind) bool {
	for _, candidate := range CandidateSourceKinds() {
		if candidate == kind {
			return true
		}
	}
	return false
}
