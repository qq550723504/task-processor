package listingkit

import (
	sheinpub "task-processor/internal/publishing/shein"
	sheinattribute "task-processor/internal/shein/api/attribute"
)

type sheinAttributeFreshnessTemplateContext struct {
	attributes     []sheinattribute.AttributeInfo
	attributeIndex map[int]sheinattribute.AttributeInfo
	resolvedByID   map[int]sheinpub.ResolvedAttribute
}

func buildSheinAttributeFreshnessTemplateContext(
	current *SheinPackage,
	templates *sheinattribute.AttributeTemplateInfo,
) (sheinAttributeFreshnessTemplateContext, bool) {
	attributes := filterSheinFreshnessDisplayAttributes(templates.Data[0].AttributeInfos)
	if len(attributes) == 0 {
		return sheinAttributeFreshnessTemplateContext{}, false
	}

	attributeIndex := make(map[int]sheinattribute.AttributeInfo, len(attributes))
	for _, attr := range attributes {
		attributeIndex[attr.AttributeID] = attr
	}

	resolvedByID := make(map[int]sheinpub.ResolvedAttribute, len(current.ResolvedAttributes))
	for _, item := range current.ResolvedAttributes {
		if item.AttributeID > 0 {
			resolvedByID[item.AttributeID] = item
		}
	}

	return sheinAttributeFreshnessTemplateContext{
		attributes:     attributes,
		attributeIndex: attributeIndex,
		resolvedByID:   resolvedByID,
	}, true
}

func evaluateSheinResolvedAttributeFreshness(
	current *SheinPackage,
	templateContext sheinAttributeFreshnessTemplateContext,
) (bool, string) {
	issueState := evaluateSheinAttributeFreshnessIssueState(current, templateContext)
	return buildSheinAttributeFreshnessOutcome(issueState, templateContext)
}
