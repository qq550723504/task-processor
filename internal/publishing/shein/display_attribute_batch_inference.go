package shein

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"task-processor/internal/core/logger"
	"task-processor/internal/pkg/jsonx"
	"task-processor/internal/prompt"
	common "task-processor/internal/publishing/common"
	sheinattribute "task-processor/internal/shein/api/attribute"
)

type templateAttributeBatchSelection struct {
	Selections []templateAttributeBatchChoice `json:"selections,omitempty"`
}

type templateAttributeBatchChoice struct {
	AttributeID         int      `json:"attribute_id,omitempty"`
	AttributeValueID    int      `json:"attribute_value_id,omitempty"`
	AttributeExtraValue string   `json:"attribute_extra_value,omitempty"`
	TextValue           string   `json:"text_value,omitempty"`
	Reasons             []string `json:"reasons,omitempty"`
}

func inferDisplayAttributesTemplateBatch(
	ctx context.Context,
	attributes []sheinattribute.AttributeInfo,
	inputs []common.Attribute,
	resolvedByID map[int]ResolvedAttribute,
	llm TextGenerator,
) ([]ResolvedAttribute, []string) {
	if llm == nil || len(attributes) == 0 || len(inputs) == 0 {
		return nil, nil
	}
	pending := collectTemplateBatchResolvableDisplayAttributes(attributes, inputs, resolvedByID)
	if len(pending) == 0 {
		return nil, nil
	}

	ctx, cancel := context.WithTimeout(contextWithFallback(ctx), 35*time.Second)
	defer cancel()

	log := logger.GetGlobalLogger("shein/attribute")
	promptText := buildDisplayAttributeTemplateBatchPrompt(pending, inputs)
	response, err := generateDisplayAttributeTemplateBatch(ctx, llm, promptText)
	if err != nil {
		log.WithError(err).WithField("pending_count", len(pending)).Warn("SHEIN 模板批量属性决策失败")
		return nil, nil
	}
	response = jsonx.CleanLLMResponse(response)
	if strings.TrimSpace(response) == "" {
		log.WithField("pending_count", len(pending)).Warn("SHEIN 模板批量属性决策返回空响应")
		return nil, nil
	}

	payload, err := parseDisplayAttributeTemplateBatchResponse(response)
	if err != nil {
		log.WithError(err).
			WithField("pending_count", len(pending)).
			WithField("response_preview", truncateDisplayAttributeBatchResponse(response, 600)).
			Warn("SHEIN 模板批量属性决策响应解析失败")
		return nil, nil
	}
	byID := make(map[int]sheinattribute.AttributeInfo, len(pending))
	for _, attr := range pending {
		byID[attr.AttributeID] = attr
	}

	resolved := make([]ResolvedAttribute, 0, len(payload.Selections))
	notes := make([]string, 0, len(payload.Selections))
	for _, choice := range payload.Selections {
		if choice.AttributeID <= 0 {
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
		if len(attr.AttributeValueInfoList) == 0 {
			value := strings.TrimSpace(firstNonEmpty(choice.AttributeExtraValue, choice.TextValue))
			if value == "" {
				notes = append(notes, choice.Reasons...)
				continue
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
				MatchedBy:           "llm_attribute_template_batch",
				Required:            isTemplateRequired(attr),
				Important:           isTemplateImportant(attr),
				SKCScope:            attr.SKCScope != nil && *attr.SKCScope,
			}
			resolved = append(resolved, match)
			resolvedByID[match.AttributeID] = match
			notes = append(notes, choice.Reasons...)
			continue
		}
		if choice.AttributeValueID <= 0 {
			notes = append(notes, choice.Reasons...)
			continue
		}
		option, ok := findDisplayAttributeOptionByID(attr, choice.AttributeValueID)
		if !ok {
			notes = append(notes, choice.Reasons...)
			continue
		}
		sourceValue := firstNonEmpty(option.AttributeValueEn, option.AttributeValue)
		match := buildResolvedAttribute(attr, option, sourceValue, "llm_attribute_template_batch")
		resolved = append(resolved, match)
		resolvedByID[match.AttributeID] = match
		notes = append(notes, choice.Reasons...)
	}
	for _, attr := range pending {
		if _, ok := resolvedByID[attr.AttributeID]; ok {
			continue
		}
		if len(attr.AttributeValueInfoList) == 0 {
			if evidence := describeDisplayAttributeEvidenceFields(inputs, 8); evidence != "" {
				notes = append(notes, fmt.Sprintf(
					"SHEIN 普通属性文本诊断: 属性 %q 当前可用证据字段为 [%s]",
					firstNonEmpty(attr.AttributeNameEn, attr.AttributeName),
					evidence,
				))
			}
			continue
		}
		if narrowed := describeDisplayAttributeCandidates(attr, "", "", inputs, len(attr.AttributeValueInfoList)); narrowed != "" {
			notes = append(notes, fmt.Sprintf(
				"SHEIN 普通属性候选诊断: 属性 %q 在模板批量决策阶段的候选集为 [%s]",
				firstNonEmpty(attr.AttributeNameEn, attr.AttributeName),
				narrowed,
			))
		}
	}
	log.WithFields(map[string]any{
		"pending_count":    len(pending),
		"selection_count":  len(payload.Selections),
		"resolved_count":   len(resolved),
		"response_preview": truncateDisplayAttributeBatchResponse(response, 600),
	}).Info("SHEIN 模板批量属性决策完成")
	return resolved, dedupeStrings(notes)
}

func collectTemplateBatchResolvableDisplayAttributes(
	attributes []sheinattribute.AttributeInfo,
	inputs []common.Attribute,
	resolvedByID map[int]ResolvedAttribute,
) []sheinattribute.AttributeInfo {
	result := make([]sheinattribute.AttributeInfo, 0)
	for _, attr := range attributes {
		if !isTemplateRequired(attr) && !isTemplateImportant(attr) && !hasExactDisplayAttributeInput(attr, inputs) {
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

func hasExactDisplayAttributeInput(attr sheinattribute.AttributeInfo, inputs []common.Attribute) bool {
	for _, input := range inputs {
		if matchesTemplateAttributeNameExactly(attr, normalizeText(input.Name)) {
			return true
		}
	}
	return false
}

func generateDisplayAttributeTemplateBatch(
	ctx context.Context,
	llm TextGenerator,
	promptText string,
) (string, error) {
	resp, err := llm.Generate(ctx, promptText)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(resp) == "" {
		return "", fmt.Errorf("SHEIN 模板批量属性决策未返回有效文本")
	}
	return resp, nil
}

func buildDisplayAttributeTemplateBatchPrompt(attributes []sheinattribute.AttributeInfo, inputs []common.Attribute) string {
	var sourceBlock strings.Builder
	for _, line := range buildAllDisplayAttributeContextLines(inputs) {
		sourceBlock.WriteString("- ")
		sourceBlock.WriteString(line)
		sourceBlock.WriteString("\n")
	}
	var attributeBlock strings.Builder
	for _, attr := range attributes {
		attributeBlock.WriteString(fmt.Sprintf(
			"- attribute_id=%d name=%q type=%d required=%t important=%t\n",
			attr.AttributeID,
			firstNonEmpty(attr.AttributeNameEn, attr.AttributeName),
			attr.AttributeType,
			isTemplateRequired(attr),
			isTemplateImportant(attr),
		))
		if len(attr.AttributeValueInfoList) == 0 {
			attributeBlock.WriteString("  - text_value_allowed=true\n")
			continue
		}
		for _, option := range attr.AttributeValueInfoList {
			attributeBlock.WriteString(fmt.Sprintf(
				"  - attribute_value_id=%d value=%q value_en=%q\n",
				option.AttributeValueID,
				option.AttributeValue,
				option.AttributeValueEn,
			))
		}
	}
	return renderSheinDisplayAttributePrompt(prompt.KSheinDisplayAttributeBatchInference, `You resolve a full batch of unresolved SHEIN display attributes for one live category template.
Use full product context to make consistent choices across all attributes in one pass.
For every listed template attribute, you must return exactly one selection entry.
For enum attributes, select only from the provided attribute_value_id options. Prefer the best-supported candidate when the evidence is directionally clear. Use 0 only when the evidence is genuinely absent or conflicting.
For text attributes, fill attribute_extra_value only when the value is explicitly supported by source evidence. Do not invent product model names or marketing copy.
Return complete JSON only, without markdown or explanatory text:
{"selections":[{"attribute_id":number,"attribute_value_id":number,"attribute_extra_value":string,"reasons":[string]}]}

Source product attributes:
{{.SourceAttributesBlock}}

Unresolved SHEIN template attributes:
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

func truncateDisplayAttributeBatchResponse(response string, limit int) string {
	response = strings.TrimSpace(response)
	if limit <= 0 || len(response) <= limit {
		return response
	}
	return response[:limit] + "..."
}

func parseDisplayAttributeTemplateBatchResponse(response string) (templateAttributeBatchSelection, error) {
	var payload templateAttributeBatchSelection
	if err := json.Unmarshal([]byte(response), &payload); err == nil {
		return payload, nil
	}
	if choices, err := parseDisplayAttributeTemplateBatchChoices(response); err == nil {
		payload.Selections = choices
		return payload, nil
	}

	extracted := extractFirstBalancedJSON(response)
	if extracted == "" || extracted == response {
		return templateAttributeBatchSelection{}, fmt.Errorf("parse batch response: invalid JSON")
	}
	if err := json.Unmarshal([]byte(extracted), &payload); err == nil {
		return payload, nil
	}
	if choices, err := parseDisplayAttributeTemplateBatchChoices(extracted); err == nil {
		payload.Selections = choices
		return payload, nil
	}
	return templateAttributeBatchSelection{}, fmt.Errorf("parse batch response: invalid JSON")
}

func parseDisplayAttributeTemplateBatchChoices(response string) ([]templateAttributeBatchChoice, error) {
	var choices []templateAttributeBatchChoice
	if err := json.Unmarshal([]byte(response), &choices); err != nil {
		return nil, err
	}
	return choices, nil
}

func extractFirstBalancedJSON(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	start := strings.IndexAny(raw, "[{")
	if start < 0 {
		return ""
	}
	segment := raw[start:]
	stack := make([]rune, 0, 8)
	inString := false
	escaped := false
	for i, r := range segment {
		if inString {
			if escaped {
				escaped = false
				continue
			}
			switch r {
			case '\\':
				escaped = true
			case '"':
				inString = false
			}
			continue
		}
		switch r {
		case '"':
			inString = true
		case '{', '[':
			stack = append(stack, r)
		case '}':
			if len(stack) == 0 || stack[len(stack)-1] != '{' {
				return ""
			}
			stack = stack[:len(stack)-1]
			if len(stack) == 0 {
				return strings.TrimSpace(segment[:i+1])
			}
		case ']':
			if len(stack) == 0 || stack[len(stack)-1] != '[' {
				return ""
			}
			stack = stack[:len(stack)-1]
			if len(stack) == 0 {
				return strings.TrimSpace(segment[:i+1])
			}
		}
	}
	return ""
}
