package shein

import sheinmarketplace "task-processor/internal/marketplace/shein/workspace"

type ResolutionCacheSummary = sheinmarketplace.ResolutionCacheSummary
type ImageUploadPreflight = sheinmarketplace.ImageUploadPreflight
type ImageUploadClassifier = sheinmarketplace.ImageUploadClassifier
type ImageUploadCacheHit = sheinmarketplace.ImageUploadCacheHit

func BuildResolutionCacheSummary(pkg *Package) *ResolutionCacheSummary {
	return sheinmarketplace.BuildResolutionCacheSummary(pkg)
}

func BuildImageUploadPreflight(
	pkg *Package,
	isUploaded ImageUploadClassifier,
	cacheHit ImageUploadCacheHit,
	isSDS ImageUploadClassifier,
) *ImageUploadPreflight {
	return sheinmarketplace.BuildImageUploadPreflight(pkg, isUploaded, cacheHit, isSDS)
}
