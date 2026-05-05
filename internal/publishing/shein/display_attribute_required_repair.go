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

const maxTargetedDisplayAttributeRepairs = 2

func inferMissingRequiredDisplayAttributesRepair(
	attributes []sheinattribute.AttributeInfo,
	inputs []common.Attribute,
	resolvedByID map[int]ResolvedAttribute,
	llm openaiclient.ChatCompleter,
) ([]ResolvedAttribute, []string) {
	if llm == nil || len(attributes) == 0 || len(inputs) == 0 {
		return nil, nil
	}
	pending := collectTemplateBatchResolvableDisplayAttributes(attributes, inputs, resolvedByID)
	if len(pending) == 0 {
		return nil, nil
	}
	if len(pending) > maxTargetedDisplayAttributeRepairs {
		notes := make([]string, 0, len(pending)+1)
		notes = append(notes, fmt.Sprintf(
			"SHEIN 必填属性仍缺失 %d 项，已跳过逐属性 repair，保留人工复核以避免重复 LLM 调用",
			len(pending),
		))
		for _, attr := range pending {
			if narrowed := describeDisplayAttributeCandidates(attr, "", "", inputs, maxDisplayAttributePromptCandidates); narrowed != "" {
				notes = append(notes, fmt.Sprintf(
					"SHEIN 普通属性候选诊断: 属性 %q 在 repair 前的候选集为 [%s]",
					firstNonEmpty(attr.AttributeNameEn, attr.AttributeName),
					narrowed,
				))
			}
		}
		return nil, dedupeStrings(notes)
	}
	resolved := make([]ResolvedAttribute, 0, len(pending))
	notes := make([]string, 0, len(pending))
	for _, attr := range pending {
		if len(attr.AttributeValueInfoList) == 0 {
			continue
		}
		match, matchNotes, ok := inferRequiredDisplayAttributeRepair(attr, inputs, llm)
		notes = append(notes, matchNotes...)
		if !ok {
			if narrowed := describeDisplayAttributeCandidates(attr, "", "", inputs, maxDisplayAttributePromptCandidates); narrowed != "" {
				notes = append(notes, fmt.Sprintf(
					"SHEIN 普通属性候选诊断: 属性 %q 在 repair 阶段的候选集为 [%s]",
					firstNonEmpty(attr.AttributeNameEn, attr.AttributeName),
					narrowed,
				))
			}
			continue
		}
		if _, exists := resolvedByID[match.AttributeID]; exists {
			continue
		}
		resolved = append(resolved, match)
		resolvedByID[match.AttributeID] = match
	}
	return resolved, dedupeStrings(notes)
}

func inferRequiredDisplayAttributeRepair(
	attr sheinattribute.AttributeInfo,
	inputs []common.Attribute,
	llm openaiclient.ChatCompleter,
) (ResolvedAttribute, []string, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()

	response, err := llm.Generate(ctx, buildRequiredDisplayAttributeRepairPrompt(attr, inputs))
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
	option, ok := findDisplayAttributeOptionByID(attr, selection.AttributeValueID)
	if !ok {
		return ResolvedAttribute{}, selection.Reasons, false
	}
	sourceValue := firstNonEmpty(option.AttributeValueEn, option.AttributeValue)
	return buildResolvedAttribute(attr, option, sourceValue, "llm_required_attribute_repair"), selection.Reasons, true
}

func buildRequiredDisplayAttributeRepairPrompt(attr sheinattribute.AttributeInfo, inputs []common.Attribute) string {
	var sourceBlock strings.Builder
	for _, line := range buildAllDisplayAttributeContextLines(inputs) {
		sourceBlock.WriteString("- ")
		sourceBlock.WriteString(line)
		sourceBlock.WriteString("\n")
	}
	var candidateBlock strings.Builder
	for _, option := range narrowDisplayAttributeValueOptions(attr, "", "", inputs, maxDisplayAttributePromptCandidates) {
		candidateBlock.WriteString(fmt.Sprintf(
			"- attribute_value_id=%d value=%q value_en=%q\n",
			option.AttributeValueID,
			option.AttributeValue,
			option.AttributeValueEn,
		))
	}
	return renderSheinDisplayAttributePrompt(prompt.KSheinDisplayAttributeRequiredRepair, `You are repairing one unresolved required SHEIN display attribute.
The attribute is required by the live SHEIN template, so choose one candidate unless every candidate directly contradicts the source product.
Use only the provided SHEIN candidate IDs. Do not invent values.
Return 0 only if selecting any candidate would create a false product claim.
Return JSON only with keys attribute_value_id and reasons.

Required SHEIN template attribute: {{.TemplateAttribute}}
Template metadata: attribute_id={{.AttributeID}} type={{.AttributeType}}
Source product attributes:
{{.SourceAttributesBlock}}Candidates:
{{.CandidatesBlock}}`, map[string]any{
		"TemplateAttribute":     fmt.Sprintf("%q", firstNonEmpty(attr.AttributeNameEn, attr.AttributeName)),
		"AttributeID":           attr.AttributeID,
		"AttributeType":         attr.AttributeType,
		"SourceAttributesBlock": sourceBlock.String(),
		"CandidatesBlock":       candidateBlock.String(),
	})
}
