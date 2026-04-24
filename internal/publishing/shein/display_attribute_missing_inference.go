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

func inferMissingRequiredDisplayAttributes(
	attributes []sheinattribute.AttributeInfo,
	inputs []common.Attribute,
	resolvedByID map[int]ResolvedAttribute,
	llm openaiclient.ChatCompleter,
) ([]ResolvedAttribute, []string) {
	if len(attributes) == 0 || len(inputs) == 0 {
		return nil, nil
	}
	inferred := make([]ResolvedAttribute, 0)
	notes := make([]string, 0)
	for _, attr := range attributes {
		if !isTemplateRequired(attr) || !dependencyIsActive(attr, resolvedByID) {
			continue
		}
		if len(attr.AttributeValueInfoList) == 0 {
			continue
		}
		if _, ok := resolvedByID[attr.AttributeID]; ok {
			continue
		}
		if llm == nil {
			if match, reasons, ok := inferMissingRequiredDisplayAttributeStatic(attr, inputs); ok {
				inferred = append(inferred, match)
				resolvedByID[match.AttributeID] = match
				notes = append(notes, reasons...)
				continue
			}
			continue
		}
		match, reasons, ok := inferDisplayAttributeValueFromContext(attr, inputs, llm)
		if !ok {
			notes = append(notes, reasons...)
			continue
		}
		inferred = append(inferred, match)
		resolvedByID[match.AttributeID] = match
		notes = append(notes, reasons...)
	}
	return inferred, dedupeStrings(notes)
}

func inferDisplayAttributeValueFromContext(
	attr sheinattribute.AttributeInfo,
	inputs []common.Attribute,
	llm openaiclient.ChatCompleter,
) (ResolvedAttribute, []string, bool) {
	if llm == nil || len(attr.AttributeValueInfoList) == 0 || len(inputs) == 0 {
		return ResolvedAttribute{}, nil, false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	response, err := llm.Generate(ctx, buildMissingDisplayAttributeInferencePrompt(attr, inputs))
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
		sourceValue := firstNonEmpty(option.AttributeValueEn, option.AttributeValue)
		match := buildResolvedAttribute(attr, option, sourceValue, "llm_attribute_inference")
		return match, selection.Reasons, true
	}
	return ResolvedAttribute{}, selection.Reasons, false
}

func buildMissingDisplayAttributeInferencePrompt(attr sheinattribute.AttributeInfo, inputs []common.Attribute) string {
	var builder strings.Builder
	builder.WriteString("You infer one missing required SHEIN display attribute from source product signals.\n")
	builder.WriteString("Choose exactly one candidate attribute_value_id only when the match is safe and well-supported.\n")
	builder.WriteString("If the source product does not clearly support any candidate, return attribute_value_id as 0.\n")
	builder.WriteString("Return JSON only with keys attribute_value_id and reasons.\n\n")
	builder.WriteString(fmt.Sprintf("Missing SHEIN template attribute: %q\n", firstNonEmpty(attr.AttributeNameEn, attr.AttributeName)))
	builder.WriteString("Source product attributes:\n")
	for _, line := range buildDisplayAttributeContextLines(inputs, "", "") {
		builder.WriteString("- ")
		builder.WriteString(line)
		builder.WriteString("\n")
	}
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
