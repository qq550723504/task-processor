package shein

import sheinattribute "task-processor/internal/shein/api/attribute"

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
		if isSaleScopeAttribute(attr) {
			continue
		}
		displayAttrs = append(displayAttrs, attr)
	}
	return newTemplateIndex(displayAttrs)
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
