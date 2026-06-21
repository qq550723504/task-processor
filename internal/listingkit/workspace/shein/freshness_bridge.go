package shein

import (
	sheinmarketplace "task-processor/internal/marketplace/shein/workspace"
	sheinpub "task-processor/internal/publishing/shein"
	sheinattribute "task-processor/internal/shein/api/attribute"
	sheincategory "task-processor/internal/shein/api/category"
)

type AttributeAPI = sheinpub.AttributeAPI

const (
	FreshnessAuthKey          = sheinmarketplace.FreshnessAuthKey
	FreshnessCategoryKey      = sheinmarketplace.FreshnessCategoryKey
	FreshnessAttributeKey     = sheinmarketplace.FreshnessAttributeKey
	FreshnessSaleAttributeKey = sheinmarketplace.FreshnessSaleAttributeKey
)

func BuildFreshnessAuthFailureCheck(err error) ReadinessCheckSpec {
	return sheinmarketplace.BuildFreshnessAuthFailureCheck(err)
}

func BuildFreshnessAuthSuccessCheck() ReadinessCheckSpec {
	return sheinmarketplace.BuildFreshnessAuthSuccessCheck()
}

func BuildFreshnessCategoryCheck(ok bool, message string) ReadinessCheckSpec {
	return sheinmarketplace.BuildFreshnessCategoryCheck(ok, message)
}

func BuildFreshnessAttributeCheck(ok bool, message string) ReadinessCheckSpec {
	return sheinmarketplace.BuildFreshnessAttributeCheck(ok, message)
}

func BuildFreshnessSaleAttributeCheck(ok bool, message string) ReadinessCheckSpec {
	return sheinmarketplace.BuildFreshnessSaleAttributeCheck(ok, message)
}

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
