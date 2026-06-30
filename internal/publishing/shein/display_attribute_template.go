package shein

import (
	sheinpublishing "task-processor/internal/marketplace/shein/publishing"
	sheinattribute "task-processor/internal/shein/api/attribute"
)

const (
	displayAttributeKindGeneral     = "general"
	displayAttributeKindNumeric     = "numeric"
	displayAttributeKindComposition = "composition"
)

type displayTemplateAttribute struct {
	Info sheinattribute.AttributeInfo
	Kind string
}

func newDisplayTemplateIndex(attributes []sheinattribute.AttributeInfo) *templateIndex {
	displayAttrs := make([]sheinattribute.AttributeInfo, 0, len(attributes))
	for _, attr := range attributes {
		if isSaleScopeAttribute(attr) || isSizeChartTemplateAttribute(attr) {
			continue
		}
		displayAttrs = append(displayAttrs, attr)
	}
	return newTemplateIndex(displayAttrs)
}

func isSizeChartTemplateAttribute(attr sheinattribute.AttributeInfo) bool {
	return sheinpublishing.IsSizeChartTemplateAttribute(attr)
}

func classifyDisplayTemplateAttribute(attr sheinattribute.AttributeInfo) displayTemplateAttribute {
	kind := displayAttributeKindGeneral
	switch attr.AttributeType {
	case 2:
		kind = displayAttributeKindNumeric
	case 3:
		kind = displayAttributeKindComposition
	}
	return displayTemplateAttribute{
		Info: attr,
		Kind: kind,
	}
}
