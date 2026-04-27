package shein

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	openaiclient "task-processor/internal/infra/clients/openai"
	"task-processor/internal/pkg/jsonx"
	"task-processor/internal/prompt"
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
		if (!isTemplateRequired(attr) && !isTemplateImportant(attr)) || !dependencyIsActiveWithInputs(attr, resolvedByID, inputs) {
			continue
		}
		if _, ok := resolvedByID[attr.AttributeID]; ok {
			continue
		}
		if match, reasons, ok := inferMissingRequiredDisplayAttributeExact(attr, inputs); ok {
			inferred = append(inferred, match)
			resolvedByID[match.AttributeID] = match
			notes = append(notes, reasons...)
			continue
		}
		if llm == nil {
			continue
		}
		if len(attr.AttributeValueInfoList) == 0 {
			match, reasons, ok := inferDisplayAttributeTextFromContext(attr, inputs, llm)
			if !ok {
				notes = append(notes, reasons...)
				continue
			}
			inferred = append(inferred, match)
			resolvedByID[match.AttributeID] = match
			notes = append(notes, reasons...)
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

type templateAttributeTextSelection struct {
	Value   string   `json:"value,omitempty"`
	Reason  string   `json:"reason,omitempty"`
	Reasons []string `json:"reasons,omitempty"`
}

func inferDisplayAttributeTextFromContext(
	attr sheinattribute.AttributeInfo,
	inputs []common.Attribute,
	llm openaiclient.ChatCompleter,
) (ResolvedAttribute, []string, bool) {
	if llm == nil || len(inputs) == 0 {
		return ResolvedAttribute{}, nil, false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	response, err := llm.Generate(ctx, buildMissingDisplayAttributeTextPrompt(attr, inputs))
	if err != nil {
		return ResolvedAttribute{}, nil, false
	}
	response = jsonx.CleanLLMResponse(response)
	if strings.TrimSpace(response) == "" {
		return ResolvedAttribute{}, nil, false
	}

	var selection templateAttributeTextSelection
	if err := json.Unmarshal([]byte(response), &selection); err != nil {
		return ResolvedAttribute{}, nil, false
	}
	value := strings.TrimSpace(selection.Value)
	if value == "" {
		return ResolvedAttribute{}, append(selection.Reasons, selection.Reason), false
	}
	match := ResolvedAttribute{
		Name:                firstNonEmpty(attr.AttributeNameEn, attr.AttributeName),
		Value:               value,
		AttributeID:         attr.AttributeID,
		AttributeExtraValue: value,
		AttributeType:       attr.AttributeType,
		AttributeMode:       attr.AttributeMode,
		DataDimension:       attr.DataDimension,
		CascadeAttributeID:  attr.CascadeAttributeID,
		MatchedBy:           "llm_attribute_text_inference",
		Required:            isTemplateRequired(attr),
		SKCScope:            attr.SKCScope != nil && *attr.SKCScope,
	}
	return match, append(selection.Reasons, selection.Reason), true
}

func buildMissingDisplayAttributeTextPrompt(attr sheinattribute.AttributeInfo, inputs []common.Attribute) string {
	var sourceBlock strings.Builder
	for _, line := range buildAllDisplayAttributeContextLines(inputs) {
		sourceBlock.WriteString("- ")
		sourceBlock.WriteString(line)
		sourceBlock.WriteString("\n")
	}
	return renderSheinDisplayAttributePrompt(prompt.KSheinDisplayAttributeMissingText, `You infer one missing SHEIN display attribute text value from source product signals.
Use only values explicitly present in source attributes. Do not invent marketing copy.
If the source product does not clearly support a value, return an empty value.
Return JSON only with keys value and reasons.

Missing SHEIN template attribute: {{.TemplateAttribute}}
Template metadata: attribute_id={{.AttributeID}} type={{.AttributeType}} required={{.Required}} important={{.Important}}
Source product attributes:
{{.SourceAttributesBlock}}`, map[string]any{
		"TemplateAttribute":     fmt.Sprintf("%q", firstNonEmpty(attr.AttributeNameEn, attr.AttributeName)),
		"AttributeID":           attr.AttributeID,
		"AttributeType":         attr.AttributeType,
		"Required":              isTemplateRequired(attr),
		"Important":             isTemplateImportant(attr),
		"SourceAttributesBlock": sourceBlock.String(),
	})
}

func buildAllDisplayAttributeContextLines(inputs []common.Attribute) []string {
	if len(inputs) == 0 {
		return nil
	}
	lines := make([]string, 0, len(inputs))
	seen := make(map[string]struct{}, len(inputs))
	for _, item := range inputs {
		name := strings.TrimSpace(item.Name)
		value := strings.TrimSpace(item.Value)
		if name == "" || value == "" {
			continue
		}
		line := fmt.Sprintf("%s=%q", name, value)
		key := normalizeText(line)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		lines = append(lines, line)
	}
	return lines
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
	var sourceBlock strings.Builder
	for _, line := range buildAllDisplayAttributeContextLines(inputs) {
		sourceBlock.WriteString("- ")
		sourceBlock.WriteString(line)
		sourceBlock.WriteString("\n")
	}
	var candidateBlock strings.Builder
	for _, option := range attr.AttributeValueInfoList {
		candidateBlock.WriteString(fmt.Sprintf(
			"- attribute_value_id=%d value=%q value_en=%q\n",
			option.AttributeValueID,
			option.AttributeValue,
			option.AttributeValueEn,
		))
	}
	return renderSheinDisplayAttributePrompt(prompt.KSheinDisplayAttributeMissingValue, `You infer one missing SHEIN display attribute from source product signals and product semantics.
For required template attributes, choose the safest candidate attribute_value_id when the product semantics support it and no source evidence contradicts it.
Prefer broad or neutral candidates over returning 0 for required fields when the candidate list contains a generic fit.
For element, pattern, decoration, or motif attributes, consider artwork, printing, production_process, design_area, and visual theme signals.
For season attributes, choose a broad all-season or multi-season candidate when the product is not limited to a specific season.
For style attributes, choose the safest everyday/neutral style candidate when the product has no niche style signal.
For important-but-not-required attributes, be more conservative and return 0 unless the match is clearly supported.
Do not invent unsupported specific claims. Return 0 when all candidates would be misleading.
Return JSON only with keys attribute_value_id and reasons.

Missing SHEIN template attribute: {{.TemplateAttribute}}
Template metadata: attribute_id={{.AttributeID}} type={{.AttributeType}} required={{.Required}} important={{.Important}}
Source product attributes:
{{.SourceAttributesBlock}}Candidates:
{{.CandidatesBlock}}`, map[string]any{
		"TemplateAttribute":     fmt.Sprintf("%q", firstNonEmpty(attr.AttributeNameEn, attr.AttributeName)),
		"AttributeID":           attr.AttributeID,
		"AttributeType":         attr.AttributeType,
		"Required":              isTemplateRequired(attr),
		"Important":             isTemplateImportant(attr),
		"SourceAttributesBlock": sourceBlock.String(),
		"CandidatesBlock":       candidateBlock.String(),
	})
}
