package shein

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	openaiclient "task-processor/internal/infra/clients/openai"
	"task-processor/internal/pkg/jsonx"
	sheinattribute "task-processor/internal/shein/api/attribute"
)

type templateAttributeValueSelection struct {
	AttributeValueID int      `json:"attribute_value_id"`
	Reasons          []string `json:"reasons,omitempty"`
}

func matchTemplateAttributeValue(
	attr sheinattribute.AttributeInfo,
	sourceName string,
	sourceValue string,
	llm openaiclient.ChatCompleter,
) (ResolvedAttribute, []string) {
	sourceValue = strings.TrimSpace(sourceValue)
	if sourceValue == "" {
		return ResolvedAttribute{}, nil
	}

	base := ResolvedAttribute{
		Name:                firstNonEmpty(attr.AttributeNameEn, attr.AttributeName),
		Value:               sourceValue,
		AttributeID:         attr.AttributeID,
		AttributeExtraValue: sourceValue,
		MatchedBy:           "attribute_name",
		Required:            isTemplateRequired(attr),
		SKCScope:            attr.SKCScope != nil && *attr.SKCScope,
	}
	if len(attr.AttributeValueInfoList) == 0 {
		return base, nil
	}

	if resolved, ok := matchTemplateAttributeValueExact(attr, sourceValue); ok {
		return resolved, nil
	}
	if resolved, ok := matchTemplateAttributeValueNormalized(attr, sourceValue); ok {
		return resolved, nil
	}
	if resolved, reasons, ok := matchTemplateAttributeValueWithLLM(attr, sourceName, sourceValue, llm); ok {
		return resolved, reasons
	}

	return base, []string{
		fmt.Sprintf("SHEIN 普通属性值未匹配: 属性 %q 的值 %q 无法映射到模板值", firstNonEmpty(attr.AttributeNameEn, attr.AttributeName), sourceValue),
	}
}

func matchTemplateAttributeValueExact(attr sheinattribute.AttributeInfo, sourceValue string) (ResolvedAttribute, bool) {
	sourceValue = strings.TrimSpace(sourceValue)
	for _, option := range attr.AttributeValueInfoList {
		if normalizeText(firstNonEmpty(option.AttributeValueEn, option.AttributeValue)) != normalizeText(sourceValue) {
			continue
		}
		return buildResolvedAttribute(attr, option, sourceValue, "attribute_value"), true
	}
	return ResolvedAttribute{}, false
}

func matchTemplateAttributeValueNormalized(attr sheinattribute.AttributeInfo, sourceValue string) (ResolvedAttribute, bool) {
	normalizedSource := comparableAttributeValueForms(sourceValue)
	if len(normalizedSource) == 0 {
		return ResolvedAttribute{}, false
	}
	sourceSet := make(map[string]struct{}, len(normalizedSource))
	for _, value := range normalizedSource {
		sourceSet[value] = struct{}{}
	}
	for _, option := range attr.AttributeValueInfoList {
		candidates := []string{
			firstNonEmpty(option.AttributeValueEn, option.AttributeValue),
			option.AttributeValue,
			option.AttributeValueEn,
		}
		for _, candidate := range candidates {
			for _, comparable := range comparableAttributeValueForms(candidate) {
				if _, ok := sourceSet[comparable]; !ok {
					continue
				}
				return buildResolvedAttribute(attr, option, sourceValue, "attribute_value_normalized"), true
			}
		}
	}
	return ResolvedAttribute{}, false
}

func matchTemplateAttributeValueWithLLM(
	attr sheinattribute.AttributeInfo,
	sourceName string,
	sourceValue string,
	llm openaiclient.ChatCompleter,
) (ResolvedAttribute, []string, bool) {
	if llm == nil || len(attr.AttributeValueInfoList) == 0 {
		return ResolvedAttribute{}, nil, false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	response, err := llm.Generate(ctx, buildTemplateAttributeValueMappingPrompt(attr, sourceName, sourceValue))
	if err != nil {
		return ResolvedAttribute{}, nil, false
	}
	response = jsonx.CleanLLMResponse(response)
	if strings.TrimSpace(response) == "" {
		return ResolvedAttribute{}, nil, false
	}

	var selection templateAttributeValueSelection
	if err := json.Unmarshal([]byte(response), &selection); err != nil {
		return ResolvedAttribute{}, nil, false
	}
	if selection.AttributeValueID <= 0 {
		return ResolvedAttribute{}, selection.Reasons, false
	}
	for _, option := range attr.AttributeValueInfoList {
		if option.AttributeValueID != selection.AttributeValueID {
			continue
		}
		return buildResolvedAttribute(attr, option, sourceValue, "llm_attribute_value"), selection.Reasons, true
	}
	return ResolvedAttribute{}, selection.Reasons, false
}

func buildTemplateAttributeValueMappingPrompt(attr sheinattribute.AttributeInfo, sourceName string, sourceValue string) string {
	var builder strings.Builder
	builder.WriteString("You map one source product attribute value to one SHEIN template attribute value.\n")
	builder.WriteString("Choose exactly one candidate attribute_value_id when there is a safe semantic match.\n")
	builder.WriteString("If none of the candidates is safe, return attribute_value_id as 0.\n")
	builder.WriteString("Return JSON only with keys attribute_value_id and reasons.\n\n")
	builder.WriteString(fmt.Sprintf("Source attribute: %q\n", sourceName))
	builder.WriteString(fmt.Sprintf("Source value: %q\n", sourceValue))
	builder.WriteString(fmt.Sprintf("SHEIN template attribute: %q\n", firstNonEmpty(attr.AttributeNameEn, attr.AttributeName)))
	builder.WriteString("Candidates:\n")
	for _, option := range attr.AttributeValueInfoList {
		builder.WriteString(fmt.Sprintf(
			"- attribute_value_id=%d value=%q value_en=%q\n",
			option.AttributeValueID,
			option.AttributeValue,
			option.AttributeValueEn,
		))
	}
	return builder.String()
}

func buildResolvedAttribute(
	attr sheinattribute.AttributeInfo,
	option sheinattribute.AttributeValue,
	sourceValue string,
	matchedBy string,
) ResolvedAttribute {
	valueID := option.AttributeValueID
	return ResolvedAttribute{
		Name:             firstNonEmpty(attr.AttributeNameEn, attr.AttributeName),
		Value:            sourceValue,
		AttributeID:      attr.AttributeID,
		AttributeValueID: &valueID,
		MatchedBy:        matchedBy,
		Required:         isTemplateRequired(attr),
		SKCScope:         attr.SKCScope != nil && *attr.SKCScope,
	}
}
