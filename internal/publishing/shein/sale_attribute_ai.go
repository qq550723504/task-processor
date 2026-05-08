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
	var sourceBuilder strings.Builder
	for _, dimension := range sourceDimensions {
		sourceBuilder.WriteString(fmt.Sprintf(
			"- name=%q values=%q distinct=%d requires_external_extraction=%t\n",
			dimension.Name,
			strings.Join(dimension.Values, " | "),
			dimension.DistinctCount,
			sourceDimensionRequiresExternalSaleAttributeExtraction(dimension),
		))
	}

	var templateBuilder strings.Builder
	for _, template := range templates {
		templateBuilder.WriteString(fmt.Sprintf(
			"- attribute_id=%d name=%q name_en=%q required=%t skc_scope=%t sample_values=%q\n",
			template.AttributeID,
			template.AttributeName,
			template.AttributeNameEn,
			isTemplateRequired(template),
			template.SKCScope != nil && *template.SKCScope,
			buildTemplateSampleValues(template),
		))
	}
	return renderSheinSaleAttributePrompt(prompt.KSheinSaleAttributeMapping, `You map source sales dimensions to SHEIN sale attributes.
Choose at most one primary_source_dimension for SKC grouping and at most one secondary_source_dimension for SKU grouping.
Keep source dimension names unchanged. Prefer required or SKC-scope SHEIN attributes.
When the platform-required primary SHEIN attribute has no exact same-name source dimension, choose the source dimension whose values are the safest stable variant grouping surrogate for that target attribute.
Avoid long image-generation prompts, technical ids, and size-only dimensions for primary style/design grouping when another source dimension is available.
If ai_style requires external extraction and Color is available, do not choose ai_style for Style or Style Type; choose Color as the stable SDS surrogate.
If there is no safe secondary mapping, leave secondary_source_dimension empty.
Return JSON only with keys primary_source_dimension, secondary_source_dimension, primary_attribute_id, secondary_attribute_id, reasons.

Source dimensions:
{{.SourceDimensionsBlock}}

SHEIN sale attribute templates:
{{.TemplatesBlock}}`, map[string]any{
		"SourceDimensionsBlock": sourceBuilder.String(),
		"TemplatesBlock":        templateBuilder.String(),
	})
}

func sourceDimensionRequiresExternalSaleAttributeExtraction(dimension SourceVariantDimension) bool {
	for _, value := range dimension.Values {
		if shouldExtractSaleAttributeSourceValue(dimension.Name, value) {
			return true
		}
	}
	return false
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
