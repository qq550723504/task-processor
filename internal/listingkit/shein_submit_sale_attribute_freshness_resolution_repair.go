package listingkit

import (
	sheinpub "task-processor/internal/publishing/shein"
)

type sheinSaleAttributeFreshnessInvalidState struct {
	invalidSKC []string
	invalidSKU []string
	changed    bool
}

func evaluateSheinSaleAttributeFreshnessInvalidState(
	current *SheinPackage,
	currentResolution *sheinpub.SaleAttributeResolution,
	templateContext sheinSaleAttributeFreshnessTemplateContext,
	api sheinpub.AttributeAPI,
) sheinSaleAttributeFreshnessInvalidState {
	customRelationIDs := sheinFreshnessCustomAttributeValueIDs(currentResolution.CustomAttributeRelation)
	invalidSKC := collectInvalidSaleAttributes(currentResolution.SKCAttributes, templateContext.byID, customRelationIDs)
	invalidSKU := collectInvalidSaleAttributes(currentResolution.SKUAttributes, templateContext.byID, customRelationIDs)
	changed := false
	if len(invalidSKC) > 0 || len(invalidSKU) > 0 {
		repaired := repairSheinFreshnessSaleAttributes(current, templateContext.attrByID, api)
		if repaired {
			changed = true
			currentResolution = current.SaleAttributeResolution
			customRelationIDs = sheinFreshnessCustomAttributeValueIDs(currentResolution.CustomAttributeRelation)
			invalidSKC = collectInvalidSaleAttributes(currentResolution.SKCAttributes, templateContext.byID, customRelationIDs)
			invalidSKU = collectInvalidSaleAttributes(currentResolution.SKUAttributes, templateContext.byID, customRelationIDs)
		}
	}

	return sheinSaleAttributeFreshnessInvalidState{
		invalidSKC: invalidSKC,
		invalidSKU: invalidSKU,
		changed:    changed,
	}
}
