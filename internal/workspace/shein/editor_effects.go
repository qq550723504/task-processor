package shein

import sheinmarketplace "task-processor/internal/marketplace/shein/workspace"

func BuildCategoryEffects() []EditorEffect {
	return sheinmarketplace.BuildCategoryEffects()
}

func BuildAttributeEffects() []EditorEffect {
	return sheinmarketplace.BuildAttributeEffects()
}

func BuildSaleAttributeEffects() []EditorEffect {
	return sheinmarketplace.BuildSaleAttributeEffects()
}
