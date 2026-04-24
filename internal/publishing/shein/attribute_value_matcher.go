package shein

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	openaiclient "task-processor/internal/infra/clients/openai"
	"task-processor/internal/pkg/jsonx"
	common "task-processor/internal/publishing/common"
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
	contextInputs []common.Attribute,
	llm openaiclient.ChatCompleter,
) (ResolvedAttribute, []string) {
	sourceValue = strings.TrimSpace(sourceValue)
	if sourceValue == "" {
		return ResolvedAttribute{}, nil
	}

	template := classifyDisplayTemplateAttribute(attr)
	base := ResolvedAttribute{
		Name:                firstNonEmpty(attr.AttributeNameEn, attr.AttributeName),
		Value:               sourceValue,
		AttributeID:         attr.AttributeID,
		AttributeExtraValue: sourceValue,
		AttributeType:       attr.AttributeType,
		AttributeMode:       attr.AttributeMode,
		DataDimension:       attr.DataDimension,
		CascadeAttributeID:  attr.CascadeAttributeID,
		MatchedBy:           "attribute_name",
		Required:            isTemplateRequired(attr),
		SKCScope:            attr.SKCScope != nil && *attr.SKCScope,
	}
	switch template.Kind {
	case displayAttributeKindNumeric:
		return base, numericAttributeNotes(attr, sourceValue)
	case displayAttributeKindComposition:
		return base, compositionAttributeNotes(attr, sourceValue)
	}

	if len(attr.AttributeValueInfoList) == 0 {
		return base, nil
	}
	if resolved, reasons, ok := matchTemplateAttributeValueWithLLM(attr, sourceName, sourceValue, contextInputs, llm); ok {
		return resolved, reasons
	}
	if llm == nil {
		if resolved, reasons, ok := matchTemplateAttributeValueStatic(attr, sourceValue, "static_attribute_value"); ok {
			return resolved, reasons
		}
	}

	return ResolvedAttribute{}, []string{
		fmt.Sprintf("SHEIN 普通属性值未匹配: 属性 %q 的值 %q 无法映射到模板值", firstNonEmpty(attr.AttributeNameEn, attr.AttributeName), sourceValue),
	}
}

func matchTemplateAttributeValueWithLLM(
	attr sheinattribute.AttributeInfo,
	sourceName string,
	sourceValue string,
	contextInputs []common.Attribute,
	llm openaiclient.ChatCompleter,
) (ResolvedAttribute, []string, bool) {
	if llm == nil || len(attr.AttributeValueInfoList) == 0 {
		return ResolvedAttribute{}, nil, false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	response, err := llm.Generate(ctx, buildTemplateAttributeValueMappingPrompt(attr, sourceName, sourceValue, contextInputs))
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

func buildTemplateAttributeValueMappingPrompt(
	attr sheinattribute.AttributeInfo,
	sourceName string,
	sourceValue string,
	contextInputs []common.Attribute,
) string {
	var builder strings.Builder
	builder.WriteString("You map one source product attribute value to one SHEIN template attribute value.\n")
	builder.WriteString("Choose exactly one candidate attribute_value_id when there is a safe semantic match.\n")
	builder.WriteString("If none of the candidates is safe, return attribute_value_id as 0.\n")
	builder.WriteString("Return JSON only with keys attribute_value_id and reasons.\n\n")
	builder.WriteString(fmt.Sprintf("Source attribute: %q\n", sourceName))
	builder.WriteString(fmt.Sprintf("Source value: %q\n", sourceValue))
	if segments := comparableAttributeSegments(sourceValue); len(segments) > 0 {
		builder.WriteString("Source value segments:\n")
		for _, segment := range segments {
			builder.WriteString(fmt.Sprintf("- %q\n", segment))
		}
	}
	if context := buildDisplayAttributeContextLines(contextInputs, sourceName, sourceValue); len(context) > 0 {
		builder.WriteString("Additional source context:\n")
		for _, line := range context {
			builder.WriteString("- ")
			builder.WriteString(line)
			builder.WriteString("\n")
		}
	}
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

func buildDisplayAttributeContextLines(inputs []common.Attribute, sourceName string, sourceValue string) []string {
	if len(inputs) == 0 {
		return nil
	}
	sourceNameNormalized := normalizeText(sourceName)
	sourceValueNormalized := normalizeText(sourceValue)
	lines := make([]string, 0, min(6, len(inputs)))
	seen := make(map[string]struct{}, len(inputs))
	for _, item := range inputs {
		name := strings.TrimSpace(item.Name)
		value := strings.TrimSpace(item.Value)
		if name == "" || value == "" {
			continue
		}
		if normalizeText(name) == sourceNameNormalized && normalizeText(value) == sourceValueNormalized {
			continue
		}
		line := fmt.Sprintf("%s=%q", name, value)
		key := normalizeText(line)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		lines = append(lines, line)
		if len(lines) >= 6 {
			break
		}
	}
	if len(lines) == 0 {
		return nil
	}
	return lines
}

func buildResolvedAttribute(
	attr sheinattribute.AttributeInfo,
	option sheinattribute.AttributeValue,
	sourceValue string,
	matchedBy string,
) ResolvedAttribute {
	valueID := option.AttributeValueID
	return ResolvedAttribute{
		Name:               firstNonEmpty(attr.AttributeNameEn, attr.AttributeName),
		Value:              sourceValue,
		AttributeID:        attr.AttributeID,
		AttributeValueID:   &valueID,
		AttributeType:      attr.AttributeType,
		AttributeMode:      attr.AttributeMode,
		DataDimension:      attr.DataDimension,
		CascadeAttributeID: attr.CascadeAttributeID,
		MatchedBy:          matchedBy,
		Required:           isTemplateRequired(attr),
		SKCScope:           attr.SKCScope != nil && *attr.SKCScope,
	}
}
