package shein

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"task-processor/internal/pkg/jsonx"
	"task-processor/internal/prompt"
	common "task-processor/internal/publishing/common"
	sheinattribute "task-processor/internal/shein/api/attribute"
	sheinproductattribute "task-processor/internal/shein/product/attribute"
)

type templateAttributeValueSelection struct {
	AttributeValueID int      `json:"attribute_value_id"`
	Reasons          []string `json:"reasons,omitempty"`
}

type templateAttributeValueBatchSelection struct {
	Selections []templateAttributeValueBatchChoice `json:"selections,omitempty"`
	Reasons    []string                            `json:"reasons,omitempty"`
}

type templateAttributeValueBatchChoice struct {
	AttributeID      int      `json:"attribute_id,omitempty"`
	AttributeValueID int      `json:"attribute_value_id"`
	Reasons          []string `json:"reasons,omitempty"`
}

type unresolvedDisplayAttributeValue struct {
	Source common.Attribute
	Attr   sheinattribute.AttributeInfo
}

const maxDisplayAttributeValueBatchEntries = 4

func matchTemplateAttributeValue(
	attr sheinattribute.AttributeInfo,
	sourceName string,
	sourceValue string,
	contextInputs []common.Attribute,
	llm TextGenerator,
) (ResolvedAttribute, []string) {
	resolved, reasons, unresolved, ok := matchTemplateAttributeValueDeterministic(attr, sourceName, sourceValue)
	if ok {
		return resolved, reasons
	}
	if unresolved == nil {
		return ResolvedAttribute{}, nil
	}
	if resolved, reasons, ok := matchTemplateAttributeValueWithLLM(attr, sourceName, sourceValue, contextInputs, llm); ok {
		return resolved, reasons
	}

	return ResolvedAttribute{}, []string{
		fmt.Sprintf("SHEIN 普通属性值未匹配: 属性 %q 的值 %q 无法映射到模板值", firstNonEmpty(attr.AttributeNameEn, attr.AttributeName), sourceValue),
	}
}

func matchTemplateAttributeValueDeterministic(
	attr sheinattribute.AttributeInfo,
	sourceName string,
	sourceValue string,
) (ResolvedAttribute, []string, *unresolvedDisplayAttributeValue, bool) {
	sourceValue = strings.TrimSpace(sourceValue)
	if sourceValue == "" {
		return ResolvedAttribute{}, nil, nil, false
	}

	template := classifyDisplayTemplateAttribute(attr)
	resolvedValue := sourceValue
	if template.Kind == displayAttributeKindNumeric {
		resolvedValue = normalizeNumericDisplayAttributeValue(sourceValue)
	}
	base := ResolvedAttribute{
		Name:                firstNonEmpty(attr.AttributeNameEn, attr.AttributeName),
		Value:               resolvedValue,
		AttributeID:         attr.AttributeID,
		AttributeExtraValue: resolvedValue,
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
		return base, numericAttributeNotes(attr, resolvedValue), nil, true
	case displayAttributeKindComposition:
		return base, compositionAttributeNotes(attr, sourceValue), nil, true
	}

	if len(attr.AttributeValueInfoList) == 0 {
		return base, nil, nil, true
	}
	if resolved, reasons, ok := matchTemplateAttributeValueExact(attr, sourceValue); ok {
		return resolved, reasons, nil, true
	}
	if resolved, reasons, ok := matchTemplateAttributeValueWithLegacyMatcher(attr, sourceValue); ok {
		return resolved, reasons, nil, true
	}
	return ResolvedAttribute{}, nil, &unresolvedDisplayAttributeValue{
		Source: common.Attribute{Name: sourceName, Value: sourceValue},
		Attr:   attr,
	}, false
}

func matchTemplateAttributeValueWithLegacyMatcher(
	attr sheinattribute.AttributeInfo,
	sourceValue string,
) (ResolvedAttribute, []string, bool) {
	matcher := sheinproductattribute.NewAttributeValueMatcher()
	platformValues := buildLegacyPlatformValues(attr)
	if len(platformValues) == 0 {
		return ResolvedAttribute{}, nil, false
	}
	matchedID := matcher.FindMatchingPlatformValue(sourceValue, platformValues)
	if matchedID <= 0 {
		return ResolvedAttribute{}, nil, false
	}
	option, ok := findDisplayAttributeOptionByID(attr, matchedID)
	if !ok {
		return ResolvedAttribute{}, nil, false
	}
	return buildResolvedAttribute(attr, option, sourceValue, "attribute_value_legacy_matcher"), []string{
		fmt.Sprintf(
			"SHEIN 普通属性值匹配命中: 属性 %q 的值 %q 命中模板候选 %q",
			firstNonEmpty(attr.AttributeNameEn, attr.AttributeName),
			strings.TrimSpace(sourceValue),
			firstNonEmpty(option.AttributeValueEn, option.AttributeValue),
		),
	}, true
}

func buildLegacyPlatformValues(attr sheinattribute.AttributeInfo) map[string]int {
	platformValues := make(map[string]int, len(attr.AttributeValueInfoList)*4)
	for _, option := range attr.AttributeValueInfoList {
		if option.AttributeValueID <= 0 {
			continue
		}
		if value := strings.TrimSpace(option.AttributeValue); value != "" {
			platformValues[value] = option.AttributeValueID
			platformValues[strings.ToLower(value)] = option.AttributeValueID
		}
		if valueEn := strings.TrimSpace(option.AttributeValueEn); valueEn != "" {
			platformValues[valueEn] = option.AttributeValueID
			platformValues[strings.ToLower(valueEn)] = option.AttributeValueID
		}
	}
	return platformValues
}

func matchTemplateAttributeValueWithLLM(
	attr sheinattribute.AttributeInfo,
	sourceName string,
	sourceValue string,
	contextInputs []common.Attribute,
	llm TextGenerator,
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

func matchTemplateAttributeValuesBatch(
	entries []unresolvedDisplayAttributeValue,
	contextInputs []common.Attribute,
	llm TextGenerator,
) (map[int]ResolvedAttribute, []string) {
	if llm == nil || len(entries) == 0 {
		return nil, nil
	}
	if len(entries) > maxDisplayAttributeValueBatchEntries {
		resolved := make(map[int]ResolvedAttribute, len(entries))
		notes := make([]string, 0)
		for start := 0; start < len(entries); start += maxDisplayAttributeValueBatchEntries {
			end := start + maxDisplayAttributeValueBatchEntries
			if end > len(entries) {
				end = len(entries)
			}
			chunkResolved, chunkNotes := matchTemplateAttributeValuesBatch(entries[start:end], contextInputs, llm)
			for attrID, match := range chunkResolved {
				resolved[attrID] = match
			}
			notes = append(notes, chunkNotes...)
		}
		if len(resolved) == 0 {
			return nil, dedupeStrings(notes)
		}
		return resolved, dedupeStrings(notes)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()

	response, err := llm.Generate(ctx, buildTemplateAttributeValueBatchMappingPrompt(entries, contextInputs))
	if err != nil {
		return nil, nil
	}
	response = jsonx.CleanLLMResponse(response)
	if strings.TrimSpace(response) == "" {
		return nil, nil
	}

	var batch templateAttributeValueBatchSelection
	if err := json.Unmarshal([]byte(response), &batch); err != nil {
		var single templateAttributeValueBatchChoice
		if singleErr := json.Unmarshal([]byte(response), &single); singleErr != nil {
			return nil, nil
		}
		batch.Selections = []templateAttributeValueBatchChoice{single}
	}
	if len(batch.Selections) == 0 {
		return nil, dedupeStrings(batch.Reasons)
	}

	entryByID := make(map[int]unresolvedDisplayAttributeValue, len(entries))
	for _, entry := range entries {
		entryByID[entry.Attr.AttributeID] = entry
	}

	resolved := make(map[int]ResolvedAttribute, len(batch.Selections))
	notes := append([]string(nil), batch.Reasons...)
	for _, selection := range batch.Selections {
		entry, ok := entryByID[selection.AttributeID]
		if !ok || selection.AttributeValueID <= 0 {
			notes = append(notes, selection.Reasons...)
			continue
		}
		for _, option := range entry.Attr.AttributeValueInfoList {
			if option.AttributeValueID != selection.AttributeValueID {
				continue
			}
			resolved[entry.Attr.AttributeID] = buildResolvedAttribute(entry.Attr, option, entry.Source.Value, "llm_attribute_value_batch")
			break
		}
		notes = append(notes, selection.Reasons...)
	}
	if len(resolved) == 0 {
		return nil, dedupeStrings(notes)
	}
	return resolved, dedupeStrings(notes)
}

func buildTemplateAttributeValueMappingPrompt(
	attr sheinattribute.AttributeInfo,
	sourceName string,
	sourceValue string,
	contextInputs []common.Attribute,
) string {
	var segmentBlock strings.Builder
	if segments := comparableAttributeSegments(sourceValue); len(segments) > 0 {
		segmentBlock.WriteString("Source value segments:\n")
		for _, segment := range segments {
			segmentBlock.WriteString(fmt.Sprintf("- %q\n", segment))
		}
	}
	contextBlock := ""
	if context := buildDisplayAttributeContextLines(contextInputs, sourceName, sourceValue); len(context) > 0 {
		contextBlock = "Additional source context:\n"
		for _, line := range context {
			contextBlock += "- " + line + "\n"
		}
	}
	var candidateBlock strings.Builder
	for _, option := range narrowDisplayAttributeValueOptions(attr, sourceName, sourceValue, contextInputs, maxDisplayAttributePromptCandidates) {
		candidateBlock.WriteString(fmt.Sprintf(
			"- attribute_value_id=%d value=%q value_en=%q\n",
			option.AttributeValueID,
			option.AttributeValue,
			option.AttributeValueEn,
		))
	}
	return renderSheinDisplayAttributePrompt(prompt.KSheinDisplayAttributeValueMapping, `You map one source product attribute value to one SHEIN template attribute value.
Choose exactly one candidate attribute_value_id when there is a safe semantic match.
If none of the candidates is safe, return attribute_value_id as 0.
Return JSON only with keys attribute_value_id and reasons.

Source attribute: {{.SourceAttribute}}
Source value: {{.SourceValue}}
{{.SourceSegmentsBlock}}{{.AdditionalContextBlock}}SHEIN template attribute: {{.TemplateAttribute}}
Candidates:
{{.CandidatesBlock}}`, map[string]any{
		"SourceAttribute":        fmt.Sprintf("%q", sourceName),
		"SourceValue":            fmt.Sprintf("%q", sourceValue),
		"SourceSegmentsBlock":    segmentBlock.String(),
		"AdditionalContextBlock": contextBlock,
		"TemplateAttribute":      fmt.Sprintf("%q", firstNonEmpty(attr.AttributeNameEn, attr.AttributeName)),
		"CandidatesBlock":        candidateBlock.String(),
	})
}

func buildTemplateAttributeValueBatchMappingPrompt(
	entries []unresolvedDisplayAttributeValue,
	contextInputs []common.Attribute,
) string {
	var attributeBlock strings.Builder
	for _, entry := range entries {
		attributeBlock.WriteString(fmt.Sprintf(
			"- attribute_id=%d template_attribute=%q source_attribute=%q source_value=%q\n",
			entry.Attr.AttributeID,
			firstNonEmpty(entry.Attr.AttributeNameEn, entry.Attr.AttributeName),
			strings.TrimSpace(entry.Source.Name),
			strings.TrimSpace(entry.Source.Value),
		))
		if segments := comparableAttributeSegments(entry.Source.Value); len(segments) > 0 {
			attributeBlock.WriteString("  source value segments:\n")
			for _, segment := range segments {
				attributeBlock.WriteString(fmt.Sprintf("  - %q\n", segment))
			}
		}
		for _, option := range narrowDisplayAttributeValueOptions(entry.Attr, entry.Source.Name, entry.Source.Value, contextInputs, maxDisplayAttributePromptCandidates) {
			attributeBlock.WriteString(fmt.Sprintf(
				"  - attribute_value_id=%d value=%q value_en=%q\n",
				option.AttributeValueID,
				option.AttributeValue,
				option.AttributeValueEn,
			))
		}
	}
	var contextBlock strings.Builder
	for _, line := range buildAllDisplayAttributeContextLines(contextInputs) {
		contextBlock.WriteString("- ")
		contextBlock.WriteString(line)
		contextBlock.WriteString("\n")
	}

	return renderSheinDisplayAttributePrompt(prompt.KSheinDisplayAttributeValueMappingBatch, `You map multiple source product attribute values to existing SHEIN template attribute values as a batch.
Work only inside each attribute's provided candidate list. Do not invent new values.
For each attribute_id, choose one candidate attribute_value_id only when the semantic match is safe.
If none is safe, set attribute_value_id to 0.
Return JSON only with shape {"selections":[{"attribute_id":number,"attribute_value_id":number,"reasons":[string]}],"reasons":[string]}.

Additional source context:
{{.AdditionalContextBlock}}

Attribute mapping tasks:
{{.AttributeTasksBlock}}`, map[string]any{
		"AdditionalContextBlock": contextBlock.String(),
		"AttributeTasksBlock":    attributeBlock.String(),
	})
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
