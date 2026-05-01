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

type saleAttributeValueBatchSelection struct {
	Matches []saleAttributeValueSelection `json:"matches,omitempty"`
	Reasons []string                      `json:"reasons,omitempty"`
}

type saleAttributeValueSelection struct {
	SourceValue      string   `json:"source_value,omitempty"`
	AttributeValueID int      `json:"attribute_value_id"`
	Reasons          []string `json:"reasons,omitempty"`
}

func matchSaleAttributeValuesWithLLM(
	attr sheinattribute.AttributeInfo,
	sourceDimension string,
	sourceValues []string,
	scope string,
	llm openaiclient.ChatCompleter,
) (map[string]ResolvedSaleAttribute, []string) {
	if llm == nil || len(attr.AttributeValueInfoList) == 0 || len(sourceValues) == 0 {
		return nil, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()

	response, err := llm.Generate(ctx, buildSaleAttributeValueBatchMappingPrompt(attr, sourceDimension, sourceValues))
	if err != nil {
		return nil, nil
	}
	response = jsonx.CleanLLMResponse(response)
	if strings.TrimSpace(response) == "" {
		return nil, nil
	}

	var batch saleAttributeValueBatchSelection
	if err := json.Unmarshal([]byte(response), &batch); err != nil {
		var single saleAttributeValueSelection
		if singleErr := json.Unmarshal([]byte(response), &single); singleErr != nil {
			return nil, nil
		}
		batch.Matches = []saleAttributeValueSelection{single}
	}

	assignments := make(map[string]ResolvedSaleAttribute, len(batch.Matches))
	notes := append([]string(nil), batch.Reasons...)
	for _, selection := range batch.Matches {
		sourceValue := strings.TrimSpace(selection.SourceValue)
		if sourceValue == "" && len(sourceValues) == 1 {
			sourceValue = strings.TrimSpace(sourceValues[0])
		}
		if sourceValue == "" || selection.AttributeValueID <= 0 {
			notes = append(notes, selection.Reasons...)
			continue
		}
		for _, option := range attr.AttributeValueInfoList {
			if option.AttributeValueID != selection.AttributeValueID {
				continue
			}
			assignments[normalizeText(sourceValue)] = buildResolvedSaleAttribute(attr, option, sourceValue, scope, "llm_attribute_value")
			break
		}
		notes = append(notes, selection.Reasons...)
	}
	if len(assignments) == 0 {
		return nil, dedupeStrings(notes)
	}
	return assignments, dedupeStrings(notes)
}

func buildSaleAttributeValueBatchMappingPrompt(attr sheinattribute.AttributeInfo, sourceDimension string, sourceValues []string) string {
	var builder strings.Builder
	builder.WriteString("You map source sales values to one existing SHEIN template attribute value.\n")
	builder.WriteString("Work only inside the provided candidate list from SHEIN. Do not invent a new value.\n")
	builder.WriteString("For each source_value, choose one candidate attribute_value_id only when the semantic match is safe.\n")
	builder.WriteString("If none is safe, set attribute_value_id to 0.\n")
	builder.WriteString("Ignore packaging words, material words, feature words, weight notes, and decorative prefixes when the actual color/size/style meaning is still clear.\n")
	builder.WriteString("Return JSON only with shape {\"matches\":[{\"source_value\":\"...\",\"attribute_value_id\":123,\"reasons\":[\"...\"]}],\"reasons\":[\"...\"]}.\n\n")
	builder.WriteString(fmt.Sprintf("Source dimension: %q\n", sourceDimension))
	builder.WriteString(fmt.Sprintf("SHEIN template attribute: %q\n", firstNonEmpty(attr.AttributeNameEn, attr.AttributeName)))
	builder.WriteString("Source values:\n")
	for _, sourceValue := range sourceValues {
		builder.WriteString(fmt.Sprintf("- %q\n", sourceValue))
	}
	builder.WriteString("\nCandidates:\n")
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
