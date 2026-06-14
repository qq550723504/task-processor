package shein

import (
	sheinmarketplace "task-processor/internal/marketplace/shein/workspace"
	sheinpub "task-processor/internal/publishing/shein"
)

func BuildCategoryRecommendationMeta(pkg *sheinpub.Package) *EditorRecommendationMeta {
	return sheinmarketplace.BuildCategoryRecommendationMeta(pkg)
}

func BuildAttributeRecommendationMeta(pkg *sheinpub.Package) *EditorRecommendationMeta {
	return sheinmarketplace.BuildAttributeRecommendationMeta(pkg)
}

func BuildSaleRecommendationMeta(pkg *sheinpub.Package) *EditorRecommendationMeta {
	return sheinmarketplace.BuildSaleRecommendationMeta(pkg)
}

func BuildAttributeSuggestions(pkg *sheinpub.Package) []EditorAttributeSuggestion {
	return sheinmarketplace.BuildAttributeSuggestions(pkg)
}

func BuildSaleCandidateSuggestions(pkg *sheinpub.Package) []EditorSaleCandidateSuggestion {
	return sheinmarketplace.BuildSaleCandidateSuggestions(pkg)
}
