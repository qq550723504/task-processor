package listingkit

import sheinpub "task-processor/internal/publishing/shein"

type sheinAttributeFreshnessIssueState struct {
	invalid      []string
	invalidItems []sheinpub.ResolvedAttribute
	missing      []string
}

func evaluateSheinAttributeFreshnessIssueState(
	current *SheinPackage,
	templateContext sheinAttributeFreshnessTemplateContext,
) sheinAttributeFreshnessIssueState {
	invalid := make([]string, 0)
	invalidItems := make([]sheinpub.ResolvedAttribute, 0)
	for _, item := range current.ResolvedAttributes {
		if item.AttributeID <= 0 {
			continue
		}
		if sheinResolvedAttributeStillLegal(item, templateContext.attributeIndex) {
			continue
		}
		invalid = append(invalid, formatResolvedAttributeDiffItem(item))
		invalidItems = append(invalidItems, item)
	}

	missingRequired := make([]string, 0)
	for _, attr := range templateContext.attributes {
		if !isSheinTemplateRequired(attr) {
			continue
		}
		if !sheinpubDependencyIsActive(attr, templateContext.resolvedByID) {
			continue
		}
		if _, ok := templateContext.resolvedByID[attr.AttributeID]; ok {
			continue
		}
		missingRequired = append(missingRequired, formatSheinFreshnessAttributeName(attr))
	}

	return sheinAttributeFreshnessIssueState{
		invalid:      invalid,
		invalidItems: invalidItems,
		missing:      missingRequired,
	}
}
