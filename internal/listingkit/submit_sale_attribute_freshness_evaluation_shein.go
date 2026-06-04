package listingkit

import (
	"fmt"

	sheinpub "task-processor/internal/publishing/shein"
	sheinattribute "task-processor/internal/shein/api/attribute"
)

type sheinSaleAttributeFreshnessTemplateContext struct {
	byID     map[int]sheinpub.SaleAttributeTemplateOption
	attrByID map[int]sheinattribute.AttributeInfo
}

func buildSheinSaleAttributeFreshnessTemplateContext(templates *sheinattribute.AttributeTemplateInfo) (sheinSaleAttributeFreshnessTemplateContext, bool) {
	saleOptions := buildSheinFreshnessSaleTemplateOptions(templates)
	if len(saleOptions) == 0 {
		return sheinSaleAttributeFreshnessTemplateContext{}, false
	}

	byID := make(map[int]sheinpub.SaleAttributeTemplateOption, len(saleOptions))
	for _, option := range saleOptions {
		byID[option.AttributeID] = option
	}

	return sheinSaleAttributeFreshnessTemplateContext{
		byID:     byID,
		attrByID: flattenSheinAttributeTemplatesByID(templates),
	}, true
}

func evaluateSheinSaleAttributeFreshnessResolution(
	current *SheinPackage,
	currentResolution *sheinpub.SaleAttributeResolution,
	templateContext sheinSaleAttributeFreshnessTemplateContext,
	api sheinpub.AttributeAPI,
) (bool, string, bool) {
	baseIssues := make([]string, 0)

	if currentResolution.PrimaryAttributeID > 0 {
		if _, ok := templateContext.byID[currentResolution.PrimaryAttributeID]; !ok {
			baseIssues = append(baseIssues, fmt.Sprintf("主规格 attribute_id=%d 已不在当前销售属性模板中", currentResolution.PrimaryAttributeID))
		}
	}
	if currentResolution.SecondaryAttributeID > 0 {
		if _, ok := templateContext.byID[currentResolution.SecondaryAttributeID]; !ok {
			baseIssues = append(baseIssues, fmt.Sprintf("副规格 attribute_id=%d 已不在当前销售属性模板中", currentResolution.SecondaryAttributeID))
		}
	}

	invalidState := evaluateSheinSaleAttributeFreshnessInvalidState(current, currentResolution, templateContext, api)
	return buildSheinSaleAttributeFreshnessResolutionOutcome(baseIssues, invalidState)
}
