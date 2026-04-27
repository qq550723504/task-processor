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

type templateAttributeBatchSelection struct {
	Selections []templateAttributeBatchChoice `json:"selections,omitempty"`
}

type templateAttributeBatchChoice struct {
	AttributeID      int      `json:"attribute_id,omitempty"`
	AttributeValueID int      `json:"attribute_value_id,omitempty"`
	Reasons          []string `json:"reasons,omitempty"`
}

func inferMissingRequiredDisplayAttributesBatch(
	attributes []sheinattribute.AttributeInfo,
	inputs []common.Attribute,
	resolvedByID map[int]ResolvedAttribute,
	llm openaiclient.ChatCompleter,
) ([]ResolvedAttribute, []string) {
	if llm == nil || len(attributes) == 0 || len(inputs) == 0 {
		return nil, nil
	}
	pending := collectBatchInferableDisplayAttributes(attributes, inputs, resolvedByID)
	if len(pending) == 0 {
		return nil, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	response, err := llm.Generate(ctx, buildMissingDisplayAttributeBatchPrompt(pending, inputs))
	if err != nil {
		return nil, nil
	}
	response = jsonx.CleanLLMResponse(response)
	if strings.TrimSpace(response) == "" {
		return nil, nil
	}

	var payload templateAttributeBatchSelection
	if err := json.Unmarshal([]byte(response), &payload); err != nil {
		var choices []templateAttributeBatchChoice
		if arrayErr := json.Unmarshal([]byte(response), &choices); arrayErr != nil {
			return nil, nil
		}
		payload.Selections = choices
	}
	if len(payload.Selections) == 0 {
		return nil, nil
	}

	byID := make(map[int]sheinattribute.AttributeInfo, len(pending))
	for _, attr := range pending {
		byID[attr.AttributeID] = attr
	}

	resolved := make([]ResolvedAttribute, 0, len(payload.Selections))
	notes := make([]string, 0, len(payload.Selections))
	for _, choice := range payload.Selections {
		if choice.AttributeID <= 0 || choice.AttributeValueID <= 0 {
			notes = append(notes, choice.Reasons...)
			continue
		}
		attr, ok := byID[choice.AttributeID]
		if !ok {
			notes = append(notes, choice.Reasons...)
			continue
		}
		if _, exists := resolvedByID[attr.AttributeID]; exists {
			continue
		}
		option, ok := findDisplayAttributeOptionByID(attr, choice.AttributeValueID)
		if !ok {
			notes = append(notes, choice.Reasons...)
			continue
		}
		sourceValue := firstNonEmpty(option.AttributeValueEn, option.AttributeValue)
		match := buildResolvedAttribute(attr, option, sourceValue, "llm_attribute_batch_inference")
		resolved = append(resolved, match)
		resolvedByID[match.AttributeID] = match
		notes = append(notes, choice.Reasons...)
	}
	return resolved, dedupeStrings(notes)
}

func collectBatchInferableDisplayAttributes(
	attributes []sheinattribute.AttributeInfo,
	inputs []common.Attribute,
	resolvedByID map[int]ResolvedAttribute,
) []sheinattribute.AttributeInfo {
	result := make([]sheinattribute.AttributeInfo, 0)
	for _, attr := range attributes {
		if !isTemplateRequired(attr) || len(attr.AttributeValueInfoList) == 0 {
			continue
		}
		if !dependencyIsActiveWithInputs(attr, resolvedByID, inputs) {
			continue
		}
		if _, ok := resolvedByID[attr.AttributeID]; ok {
			continue
		}
		result = append(result, attr)
	}
	return result
}

func buildMissingDisplayAttributeBatchPrompt(attributes []sheinattribute.AttributeInfo, inputs []common.Attribute) string {
	var sourceBlock strings.Builder
	for _, line := range buildAllDisplayAttributeContextLines(inputs) {
		sourceBlock.WriteString("- ")
		sourceBlock.WriteString(line)
		sourceBlock.WriteString("\n")
	}
	var attributeBlock strings.Builder
	for _, attr := range attributes {
		attributeBlock.WriteString(fmt.Sprintf(
			"- attribute_id=%d name=%q type=%d\n",
			attr.AttributeID,
			firstNonEmpty(attr.AttributeNameEn, attr.AttributeName),
			attr.AttributeType,
		))
		for _, option := range attr.AttributeValueInfoList {
			attributeBlock.WriteString(fmt.Sprintf(
				"  - attribute_value_id=%d value=%q value_en=%q\n",
				option.AttributeValueID,
				option.AttributeValue,
				option.AttributeValueEn,
			))
		}
	}
	return renderSheinDisplayAttributePrompt(prompt.KSheinDisplayAttributeBatchInference, `You complete missing required SHEIN display attributes as a batch.
Use full product context to make consistent choices across attributes.
For each required attribute, choose the safest candidate attribute_value_id when product semantics support it and no source evidence contradicts it.
Prefer broad or neutral candidates over 0 when the candidate list contains a generic fit.
For element, pattern, decoration, or motif attributes, consider artwork, printing, production_process, design_area, and visual theme signals.
For season attributes, choose a broad all-season or multi-season candidate when the product is not limited to a specific season.
For style attributes, choose the safest everyday/neutral style candidate when the product has no niche style signal.
Return JSON only: {"selections":[{"attribute_id":number,"attribute_value_id":number,"reasons":[string]}]}.
Use attribute_value_id 0 only when every candidate would be misleading.

Source product attributes:
{{.SourceAttributesBlock}}

Missing required SHEIN attributes:
{{.AttributesBlock}}`, map[string]any{
		"SourceAttributesBlock": sourceBlock.String(),
		"AttributesBlock":       attributeBlock.String(),
	})
}

func findDisplayAttributeOptionByID(attr sheinattribute.AttributeInfo, valueID int) (sheinattribute.AttributeValue, bool) {
	for _, option := range attr.AttributeValueInfoList {
		if option.AttributeValueID == valueID {
			return option, true
		}
	}
	return sheinattribute.AttributeValue{}, false
}
