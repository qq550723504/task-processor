package shein

import (
	"fmt"
	"strings"

	common "task-processor/internal/publishing/common"
	sheinattribute "task-processor/internal/shein/api/attribute"
)

func selectDisplayTemplateAttributeStatic(
	attributes []sheinattribute.AttributeInfo,
	source common.Attribute,
) (*sheinattribute.AttributeInfo, []string) {
	sourceName := normalizeText(source.Name)
	if sourceName == "" {
		return nil, nil
	}
	sourceAliases := staticDisplayFieldAliases(sourceName)
	for _, attr := range attributes {
		if !matchesAnyNameWithAliases(attr, sourceAliases) {
			continue
		}
		attrCopy := attr
		return &attrCopy, []string{fmt.Sprintf(
			"SHEIN 普通属性字段静态匹配: 源属性 %q 映射到模板字段 %q",
			strings.TrimSpace(source.Name),
			firstNonEmpty(attr.AttributeNameEn, attr.AttributeName),
		)}
	}
	return nil, nil
}

func matchTemplateAttributeValueStatic(
	attr sheinattribute.AttributeInfo,
	sourceValue string,
	matchedBy string,
) (ResolvedAttribute, []string, bool) {
	if len(attr.AttributeValueInfoList) == 0 {
		return ResolvedAttribute{}, nil, false
	}
	valueAliases := staticDisplayValueAliases(firstNonEmpty(attr.AttributeNameEn, attr.AttributeName), sourceValue)
	if len(valueAliases) == 0 {
		return ResolvedAttribute{}, nil, false
	}
	option, ok := findDisplayAttributeOption(attr, valueAliases)
	if !ok {
		return ResolvedAttribute{}, nil, false
	}
	match := buildResolvedAttribute(attr, option, sourceValue, matchedBy)
	return match, []string{fmt.Sprintf(
		"SHEIN 普通属性值静态匹配: 属性 %q 的值 %q 映射到 %q",
		firstNonEmpty(attr.AttributeNameEn, attr.AttributeName),
		strings.TrimSpace(sourceValue),
		firstNonEmpty(option.AttributeValueEn, option.AttributeValue),
	)}, true
}

func inferMissingRequiredDisplayAttributeStatic(
	attr sheinattribute.AttributeInfo,
	inputs []common.Attribute,
) (ResolvedAttribute, []string, bool) {
	if len(attr.AttributeValueInfoList) == 0 {
		return ResolvedAttribute{}, nil, false
	}
	attrName := normalizeText(firstNonEmpty(attr.AttributeNameEn, attr.AttributeName))
	switch attrName {
	case "other material", "其他材质", "其它材质":
		if !sourceHasKnownStandardMaterial(inputs) {
			return ResolvedAttribute{}, nil, false
		}
		return inferStaticOption(attr, []string{
			"no other material",
			"none",
			"no",
			"无其他材质",
			"无其它材质",
			"没有其他材质",
		}, "static_attribute_inference")
	case "hazard category", "危险品类", "危险类别":
		return inferStaticOption(attr, []string{
			"non hazardous",
			"non-hazardous",
			"not hazardous",
			"no hazard",
			"none",
			"no",
			"非危险品",
			"无危险",
			"普通商品",
		}, "static_attribute_inference")
	case "style", "风格":
		return inferStaticOption(attr, []string{
			"basic",
			"simple",
			"classic",
			"casual",
			"other",
			"基础",
			"简约",
			"经典",
			"其他",
		}, "static_attribute_inference")
	default:
		return ResolvedAttribute{}, nil, false
	}
}

func inferStaticOption(
	attr sheinattribute.AttributeInfo,
	aliases []string,
	matchedBy string,
) (ResolvedAttribute, []string, bool) {
	option, ok := findDisplayAttributeOption(attr, aliases)
	if !ok {
		return ResolvedAttribute{}, nil, false
	}
	sourceValue := firstNonEmpty(option.AttributeValueEn, option.AttributeValue)
	match := buildResolvedAttribute(attr, option, sourceValue, matchedBy)
	return match, []string{fmt.Sprintf(
		"SHEIN 必填展示属性静态补齐: 属性 %q 使用模板值 %q",
		firstNonEmpty(attr.AttributeNameEn, attr.AttributeName),
		sourceValue,
	)}, true
}

func matchesAnyNameWithAliases(attr sheinattribute.AttributeInfo, aliases []string) bool {
	if len(aliases) == 0 {
		return false
	}
	candidates := collectAttributeNames(attr)
	for _, alias := range aliases {
		if matchesAnyName(alias, candidates) {
			return true
		}
	}
	return false
}

func findDisplayAttributeOption(attr sheinattribute.AttributeInfo, aliases []string) (sheinattribute.AttributeValue, bool) {
	normalizedAliases := make(map[string]struct{}, len(aliases))
	for _, alias := range aliases {
		alias = normalizeText(alias)
		if alias == "" {
			continue
		}
		normalizedAliases[alias] = struct{}{}
	}
	if len(normalizedAliases) == 0 {
		return sheinattribute.AttributeValue{}, false
	}
	for _, option := range attr.AttributeValueInfoList {
		for _, candidate := range []string{option.AttributeValueEn, option.AttributeValue} {
			if _, ok := normalizedAliases[normalizeText(candidate)]; ok {
				return option, true
			}
		}
	}
	return sheinattribute.AttributeValue{}, false
}

func staticDisplayFieldAliases(sourceName string) []string {
	switch normalizeText(sourceName) {
	case "material", "材质", "texture", "面料", "成分", "material description", "material_description":
		return []string{"material", "材质"}
	case "production process", "production_process", "工艺", "生产工艺", "印刷工艺":
		return []string{"production process", "工艺", "生产工艺"}
	case "design area", "design_area", "印刷区域", "打印区域":
		return []string{"design area", "印刷区域"}
	case "applicable scenarios", "applicable_scenarios", "适用场景", "场景":
		return []string{"occasion", "room", "scene", "适用场景", "场景"}
	default:
		return []string{sourceName}
	}
}

func staticDisplayValueAliases(attrName string, sourceValue string) []string {
	value := normalizeText(sourceValue)
	if value == "" {
		return nil
	}
	switch normalizeText(attrName) {
	case "material", "材质":
		if isPolyesterValue(value) {
			return []string{"polyester", "聚酯纤维", "涤纶"}
		}
		if strings.Contains(value, "cotton") || strings.Contains(value, "棉") {
			return []string{"cotton", "棉", "全棉"}
		}
	}
	return []string{sourceValue}
}

func sourceHasKnownStandardMaterial(inputs []common.Attribute) bool {
	for _, input := range inputs {
		if !matchesAnyName(normalizeText(input.Name), []string{"material", "材质", "texture", "面料", "成分"}) {
			continue
		}
		if isPolyesterValue(normalizeText(input.Value)) || strings.Contains(normalizeText(input.Value), "cotton") || strings.Contains(input.Value, "棉") {
			return true
		}
	}
	return false
}

func isPolyesterValue(value string) bool {
	value = normalizeText(value)
	return strings.Contains(value, "polyester") ||
		strings.Contains(value, "涤纶") ||
		strings.Contains(value, "聚酯") ||
		strings.Contains(value, "聚脂")
}
