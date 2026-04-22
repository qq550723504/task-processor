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

type saleAttributeValueSelection struct {
	AttributeValueID int      `json:"attribute_value_id"`
	Reasons          []string `json:"reasons,omitempty"`
}

func buildValueAssignments(
	values []string,
	sourceDimension string,
	templateName string,
	scope string,
	index *templateIndex,
	llm openaiclient.ChatCompleter,
) (map[string]ResolvedSaleAttribute, []string) {
	if len(values) == 0 || strings.TrimSpace(templateName) == "" || index == nil {
		return nil, nil
	}
	attr := index.FindAttribute(templateName)
	if attr == nil {
		return nil, []string{fmt.Sprintf("SHEIN 销售属性模板 %q 不存在，无法映射源维度 %q", templateName, sourceDimension)}
	}

	assignments := make(map[string]ResolvedSaleAttribute, len(values))
	var notes []string
	for _, value := range uniqueNormalizedValues(values) {
		resolved, matchNotes := matchSaleAttributeValue(attr, sourceDimension, value, scope, llm)
		notes = append(notes, matchNotes...)
		if resolved.AttributeID <= 0 || resolved.AttributeValueID == nil {
			continue
		}
		assignments[normalizeText(value)] = resolved
	}
	if len(assignments) == 0 {
		return nil, dedupeStrings(notes)
	}
	return assignments, dedupeStrings(notes)
}

func matchSaleAttributeValue(
	attr *sheinattribute.AttributeInfo,
	sourceDimension string,
	sourceValue string,
	scope string,
	llm openaiclient.ChatCompleter,
) (ResolvedSaleAttribute, []string) {
	sourceValue = strings.TrimSpace(sourceValue)
	if attr == nil || sourceValue == "" {
		return ResolvedSaleAttribute{}, nil
	}
	if resolved, ok := matchSaleAttributeValueExact(*attr, sourceValue, scope); ok {
		return resolved, nil
	}
	if resolved, ok := matchSaleAttributeValueNormalized(*attr, sourceValue, scope); ok {
		return resolved, nil
	}
	if resolved, reasons, ok := matchSaleAttributeValueWithLLM(*attr, sourceDimension, sourceValue, scope, llm); ok {
		return resolved, reasons
	}

	return ResolvedSaleAttribute{}, []string{
		fmt.Sprintf(
			"SHEIN 销售属性值未匹配: 源维度 %q 的值 %q 无法映射到模板属性 %q",
			sourceDimension,
			sourceValue,
			firstNonEmpty(attr.AttributeNameEn, attr.AttributeName),
		),
	}
}

func matchSaleAttributeValueExact(attr sheinattribute.AttributeInfo, sourceValue string, scope string) (ResolvedSaleAttribute, bool) {
	sourceValue = strings.TrimSpace(sourceValue)
	if sourceValue == "" {
		return ResolvedSaleAttribute{}, false
	}
	for _, option := range attr.AttributeValueInfoList {
		if normalizeText(firstNonEmpty(option.AttributeValueEn, option.AttributeValue)) != normalizeText(sourceValue) {
			continue
		}
		return buildResolvedSaleAttribute(attr, option, sourceValue, scope, "attribute_value"), true
	}
	return ResolvedSaleAttribute{}, false
}

func matchSaleAttributeValueNormalized(attr sheinattribute.AttributeInfo, sourceValue string, scope string) (ResolvedSaleAttribute, bool) {
	normalizedSource := comparableAttributeValueForms(sourceValue)
	if len(normalizedSource) == 0 {
		return ResolvedSaleAttribute{}, false
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
				return buildResolvedSaleAttribute(attr, option, sourceValue, scope, "attribute_value_normalized"), true
			}
		}
	}
	return ResolvedSaleAttribute{}, false
}

func matchSaleAttributeValueWithLLM(
	attr sheinattribute.AttributeInfo,
	sourceDimension string,
	sourceValue string,
	scope string,
	llm openaiclient.ChatCompleter,
) (ResolvedSaleAttribute, []string, bool) {
	if llm == nil || len(attr.AttributeValueInfoList) == 0 {
		return ResolvedSaleAttribute{}, nil, false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	response, err := llm.Generate(ctx, buildSaleAttributeValueMappingPrompt(attr, sourceDimension, sourceValue))
	if err != nil {
		return ResolvedSaleAttribute{}, nil, false
	}
	response = jsonx.CleanLLMResponse(response)
	if strings.TrimSpace(response) == "" {
		return ResolvedSaleAttribute{}, nil, false
	}

	var selection saleAttributeValueSelection
	if err := json.Unmarshal([]byte(response), &selection); err != nil {
		return ResolvedSaleAttribute{}, nil, false
	}
	if selection.AttributeValueID <= 0 {
		return ResolvedSaleAttribute{}, selection.Reasons, false
	}
	for _, option := range attr.AttributeValueInfoList {
		if option.AttributeValueID != selection.AttributeValueID {
			continue
		}
		return buildResolvedSaleAttribute(attr, option, sourceValue, scope, "llm_attribute_value"), selection.Reasons, true
	}
	return ResolvedSaleAttribute{}, selection.Reasons, false
}

func buildSaleAttributeValueMappingPrompt(attr sheinattribute.AttributeInfo, sourceDimension string, sourceValue string) string {
	var builder strings.Builder
	builder.WriteString("You map one source sales value to one SHEIN template attribute value.\n")
	builder.WriteString("Choose exactly one candidate attribute_value_id when there is a safe semantic match.\n")
	builder.WriteString("If none of the candidates is safe, return attribute_value_id as 0.\n")
	builder.WriteString("Return JSON only with keys attribute_value_id and reasons.\n\n")
	builder.WriteString(fmt.Sprintf("Source dimension: %q\n", sourceDimension))
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

func buildResolvedSaleAttribute(
	attr sheinattribute.AttributeInfo,
	option sheinattribute.AttributeValue,
	sourceValue string,
	scope string,
	matchedBy string,
) ResolvedSaleAttribute {
	valueID := option.AttributeValueID
	return ResolvedSaleAttribute{
		Scope:            scope,
		Name:             firstNonEmpty(attr.AttributeNameEn, attr.AttributeName),
		Value:            sourceValue,
		AttributeID:      attr.AttributeID,
		AttributeValueID: &valueID,
		MatchedBy:        matchedBy,
	}
}

func trimSaleAttributeCodePrefix(value string) string {
	for i, r := range value {
		if r > 127 {
			prefix := strings.TrimSpace(value[:i])
			if prefix == "" {
				return value
			}
			if isLikelySaleAttributeCodePrefix(prefix) {
				return value[i:]
			}
			return value
		}
	}
	return value
}

func isLikelySaleAttributeCodePrefix(prefix string) bool {
	if prefix == "" {
		return false
	}
	hasLetterOrDigit := false
	for _, r := range prefix {
		switch {
		case r >= 'a' && r <= 'z':
			hasLetterOrDigit = true
		case r >= '0' && r <= '9':
			hasLetterOrDigit = true
		case strings.ContainsRune(" -_./", r):
		default:
			return false
		}
	}
	return hasLetterOrDigit
}

func dedupeStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		key := strings.TrimSpace(value)
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, key)
	}
	return result
}
