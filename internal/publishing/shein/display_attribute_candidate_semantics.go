package shein

import (
	"strings"

	common "task-processor/internal/publishing/common"
	sheinattribute "task-processor/internal/shein/api/attribute"
)

func inferMissingRequiredDisplayAttributesFromCandidateSemantics(
	attributes []sheinattribute.AttributeInfo,
	inputs []common.Attribute,
	resolvedByID map[int]ResolvedAttribute,
) ([]ResolvedAttribute, []string) {
	if len(attributes) == 0 || len(inputs) == 0 {
		return nil, nil
	}
	pending := collectBatchInferableDisplayAttributes(attributes, inputs, resolvedByID)
	resolved := make([]ResolvedAttribute, 0, len(pending))
	notes := make([]string, 0, len(pending))
	for _, attr := range pending {
		option, reason, ok := selectRequiredAttributeCandidateBySemantics(attr, inputs)
		if !ok {
			continue
		}
		sourceValue := firstNonEmpty(option.AttributeValueEn, option.AttributeValue)
		match := buildResolvedAttribute(attr, option, sourceValue, "template_candidate_semantic_repair")
		resolved = append(resolved, match)
		resolvedByID[match.AttributeID] = match
		notes = append(notes, reason)
	}
	return resolved, dedupeStrings(notes)
}

func selectRequiredAttributeCandidateBySemantics(attr sheinattribute.AttributeInfo, inputs []common.Attribute) (sheinattribute.AttributeValue, string, bool) {
	name := normalizeText(firstNonEmpty(attr.AttributeNameEn, attr.AttributeName))
	source := normalizeAttributeSourceText(inputs)
	switch {
	case containsAnyToken(name, "season", "季节"):
		if option, ok := findCandidateByTerms(attr, "all", "all season", "all seasons", "全年", "所有", "四季", "全球"); ok {
			return option, "SHEIN 必填属性 Season 使用模板提供的通用全年候选值", true
		}
	case containsAnyToken(name, "style", "风格"):
		if option, ok := findCandidateByTerms(attr, "casual", "daily", "everyday", "basic", "休闲", "日常", "基础"); ok {
			return option, "SHEIN 必填属性 Style 使用模板提供的通用日常风格候选值", true
		}
	case containsAnyToken(name, "element", "pattern", "decoration", "motif", "元素", "图案", "装饰"):
		if containsAnyToken(source, "print", "printing", "printed", "heat transfer", "烫画", "印花", "印制", "图案") {
			if option, ok := findCandidateByTerms(attr, "print", "printing", "printed", "印花", "印制"); ok {
				return option, "SHEIN 必填属性 Element 使用源商品印制/烫画信号匹配模板印花候选值", true
			}
		}
	}
	return sheinattribute.AttributeValue{}, "", false
}

func findCandidateByTerms(attr sheinattribute.AttributeInfo, terms ...string) (sheinattribute.AttributeValue, bool) {
	for _, option := range attr.AttributeValueInfoList {
		values := []string{option.AttributeValueEn, option.AttributeValue, firstNonEmpty(option.AttributeValueEn, option.AttributeValue)}
		if candidateValueMatchesAnyTerm(values, terms...) {
			return option, true
		}
	}
	return sheinattribute.AttributeValue{}, false
}

func candidateValueMatchesAnyTerm(values []string, terms ...string) bool {
	for _, value := range values {
		normalizedValue := normalizeText(value)
		if normalizedValue == "" {
			continue
		}
		paddedValue := " " + normalizedValue + " "
		for _, term := range terms {
			normalizedTerm := normalizeText(term)
			if normalizedTerm == "" {
				continue
			}
			if normalizedValue == normalizedTerm || strings.Contains(paddedValue, " "+normalizedTerm+" ") {
				return true
			}
			if containsCJK(normalizedTerm) && strings.Contains(normalizedValue, normalizedTerm) {
				return true
			}
		}
	}
	return false
}

func normalizeAttributeSourceText(inputs []common.Attribute) string {
	var parts []string
	for _, input := range inputs {
		parts = append(parts, input.Name, input.Value)
	}
	return normalizeText(strings.Join(parts, " "))
}

func containsAnyToken(value string, terms ...string) bool {
	value = normalizeText(value)
	if value == "" {
		return false
	}
	for _, term := range terms {
		term = normalizeText(term)
		if term != "" && strings.Contains(value, term) {
			return true
		}
	}
	return false
}
