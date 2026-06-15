package shein

import (
	sheinmarketplace "task-processor/internal/marketplace/shein/workspace"
	sheinpub "task-processor/internal/publishing/shein"
	sheinattribute "task-processor/internal/shein/api/attribute"
	sheincategory "task-processor/internal/shein/api/category"
)

type AttributeAPI = sheinpub.AttributeAPI

func EvaluateCategoryFreshness(current *Package, info *sheincategory.CategoryInfo) (bool, string) {
	return sheinmarketplace.EvaluateCategoryFreshness(current, info)
}

func EvaluateAttributeFreshness(current *Package, templates *sheinattribute.AttributeTemplateInfo) (bool, string) {
	return sheinmarketplace.EvaluateAttributeFreshness(current, templates)
}

func EvaluateSaleAttributeFreshness(current *Package, templates *sheinattribute.AttributeTemplateInfo) (bool, string) {
	return sheinmarketplace.EvaluateSaleAttributeFreshness(current, templates)
}

func EvaluateSaleAttributeFreshnessWithCustomValidation(current *Package, templates *sheinattribute.AttributeTemplateInfo, api AttributeAPI) (bool, string, bool) {
	return sheinmarketplace.EvaluateSaleAttributeFreshnessWithCustomValidation(current, templates, api)
}
