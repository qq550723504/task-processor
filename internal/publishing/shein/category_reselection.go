package shein

import "task-processor/internal/productenrich"

type categoryRecommender interface {
	SuggestAlternative(req *BuildRequest, canonical *productenrich.CanonicalProduct, pkg *Package) *CategorySuggestion
}
