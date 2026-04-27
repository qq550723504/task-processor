package shein

import (
	"fmt"
	"strings"

	common "task-processor/internal/publishing/common"
	sheinattribute "task-processor/internal/shein/api/attribute"
)

func selectDisplayTemplateAttributeExact(
	attributes []sheinattribute.AttributeInfo,
	source common.Attribute,
) (*sheinattribute.AttributeInfo, []string) {
	sourceName := normalizeText(source.Name)
	if sourceName == "" {
		return nil, nil
	}
	for _, attr := range attributes {
		if !matchesTemplateAttributeNameExactly(attr, sourceName) {
			continue
		}
		attrCopy := attr
		return &attrCopy, []string{fmt.Sprintf(
			"SHEIN 普通属性字段精确匹配: 源属性 %q 映射到模板字段 %q",
			strings.TrimSpace(source.Name),
			firstNonEmpty(attr.AttributeNameEn, attr.AttributeName),
		)}
	}
	return nil, nil
}

func matchTemplateAttributeValueExact(
	attr sheinattribute.AttributeInfo,
	sourceValue string,
) (ResolvedAttribute, []string, bool) {
	if len(attr.AttributeValueInfoList) == 0 {
		return ResolvedAttribute{}, nil, false
	}
	option, ok := findDisplayAttributeOptionExactly(attr, sourceValue)
	if !ok {
		return ResolvedAttribute{}, nil, false
	}
	match := buildResolvedAttribute(attr, option, sourceValue, "attribute_value")
	return match, []string{fmt.Sprintf(
		"SHEIN 普通属性值精确匹配: 属性 %q 的值 %q 映射到 %q",
		firstNonEmpty(attr.AttributeNameEn, attr.AttributeName),
		strings.TrimSpace(sourceValue),
		firstNonEmpty(option.AttributeValueEn, option.AttributeValue),
	)}, true
}

func inferMissingRequiredDisplayAttributeExact(
	attr sheinattribute.AttributeInfo,
	inputs []common.Attribute,
) (ResolvedAttribute, []string, bool) {
	return ResolvedAttribute{}, nil, false
}

func matchesTemplateAttributeNameExactly(attr sheinattribute.AttributeInfo, sourceName string) bool {
	for _, candidate := range []string{attr.AttributeNameEn, attr.AttributeName} {
		if normalizeText(candidate) == sourceName {
			return true
		}
	}
	return false
}

func findDisplayAttributeOptionExactly(attr sheinattribute.AttributeInfo, sourceValue string) (sheinattribute.AttributeValue, bool) {
	sourceValue = normalizeText(sourceValue)
	if sourceValue == "" {
		return sheinattribute.AttributeValue{}, false
	}
	for _, option := range attr.AttributeValueInfoList {
		for _, candidate := range []string{option.AttributeValueEn, option.AttributeValue} {
			if normalizeText(candidate) == sourceValue {
				return option, true
			}
		}
	}
	return sheinattribute.AttributeValue{}, false
}
