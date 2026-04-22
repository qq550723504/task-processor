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

type saleAttributeMappingSelection struct {
	PrimarySourceDimension   string   `json:"primary_source_dimension"`
	SecondarySourceDimension string   `json:"secondary_source_dimension,omitempty"`
	PrimaryAttributeID       int      `json:"primary_attribute_id,omitempty"`
	SecondaryAttributeID     int      `json:"secondary_attribute_id,omitempty"`
	Reasons                  []string `json:"reasons,omitempty"`
}

func selectSaleAttributeMappingWithLLM(
	client openaiclient.ChatCompleter,
	sourceDimensions []SourceVariantDimension,
	templates []sheinattribute.AttributeInfo,
) (*saleAttributeMappingSelection, error) {
	if client == nil || len(sourceDimensions) == 0 || len(templates) == 0 {
		return nil, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()

	response, err := client.Generate(ctx, buildSaleAttributeMappingPrompt(sourceDimensions, templates))
	if err != nil {
		return nil, err
	}

	response = jsonx.CleanLLMResponse(response)
	if strings.TrimSpace(response) == "" {
		return nil, fmt.Errorf("empty llm response")
	}

	var selection saleAttributeMappingSelection
	if err := json.Unmarshal([]byte(response), &selection); err != nil {
		return nil, err
	}
	return &selection, nil
}

func buildSaleAttributeMappingPrompt(sourceDimensions []SourceVariantDimension, templates []sheinattribute.AttributeInfo) string {
	var builder strings.Builder
	builder.WriteString("You map source sales dimensions to SHEIN sale attributes.\n")
	builder.WriteString("Choose at most one primary_source_dimension for SKC grouping and at most one secondary_source_dimension for SKU grouping.\n")
	builder.WriteString("Keep source dimension names unchanged. Prefer required or SKC-scope SHEIN attributes when they match the source meaning.\n")
	builder.WriteString("If there is no safe secondary mapping, leave secondary_source_dimension empty.\n")
	builder.WriteString("Return JSON only with keys primary_source_dimension, secondary_source_dimension, primary_attribute_id, secondary_attribute_id, reasons.\n\n")

	builder.WriteString("Source dimensions:\n")
	for _, dimension := range sourceDimensions {
		builder.WriteString(fmt.Sprintf("- name=%q values=%q distinct=%d\n", dimension.Name, strings.Join(dimension.Values, " | "), dimension.DistinctCount))
	}

	builder.WriteString("\nSHEIN sale attribute templates:\n")
	for _, template := range templates {
		builder.WriteString(fmt.Sprintf(
			"- attribute_id=%d name=%q name_en=%q required=%t skc_scope=%t sample_values=%q\n",
			template.AttributeID,
			template.AttributeName,
			template.AttributeNameEn,
			isTemplateRequired(template),
			template.SKCScope != nil && *template.SKCScope,
			buildTemplateSampleValues(template),
		))
	}
	return builder.String()
}

func buildTemplateSampleValues(template sheinattribute.AttributeInfo) string {
	if len(template.AttributeValueInfoList) == 0 {
		return ""
	}
	values := make([]string, 0, min(3, len(template.AttributeValueInfoList)))
	for _, item := range template.AttributeValueInfoList {
		value := firstNonEmpty(item.AttributeValueEn, item.AttributeValue)
		if strings.TrimSpace(value) == "" {
			continue
		}
		values = append(values, value)
		if len(values) >= 3 {
			break
		}
	}
	return strings.Join(values, " | ")
}
