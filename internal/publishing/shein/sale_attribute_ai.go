package shein

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

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
	client TextGenerator,
	sourceDimensions []SourceVariantDimension,
	templates []sheinattribute.AttributeInfo,
) (*saleAttributeMappingSelection, error) {
	return selectSaleAttributeMappingWithLLMFeedback(client, sourceDimensions, templates, "")
}

func selectSaleAttributeMappingWithLLMFeedback(
	client TextGenerator,
	sourceDimensions []SourceVariantDimension,
	templates []sheinattribute.AttributeInfo,
	feedback string,
) (*saleAttributeMappingSelection, error) {
	if client == nil || len(sourceDimensions) == 0 || len(templates) == 0 {
		return nil, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()

	response, err := client.Generate(ctx, buildSaleAttributeMappingPrompt(sourceDimensions, templates, feedback))
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

func buildSaleAttributeMappingPrompt(sourceDimensions []SourceVariantDimension, templates []sheinattribute.AttributeInfo, feedback string) string {
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
			"- attribute_id=%d name=%q name_en=%q primary_label=%t required=%t skc_scope=%t sample_values=%q\n",
			template.AttributeID,
			template.AttributeName,
			template.AttributeNameEn,
			template.AttributeLabel == 1,
			isTemplateRequired(template),
			template.SKCScope != nil && *template.SKCScope,
			buildTemplateSampleValues(template),
		))
	}
	feedback = strings.TrimSpace(feedback)
	feedbackBlock := ""
	if feedback != "" {
		feedbackBlock = "\nCorrection feedback from validator:\n" + feedback + "\n"
	}
	return renderSheinSaleAttributePrompt(prompt.KSheinSaleAttributeMapping, `You map source sales dimensions to SHEIN sale attributes.
Use the SHEIN sale attribute templates in the exact order shown.
The first template is the primary SKC sale attribute target. A template with primary_label=true is the authoritative SHEIN primary sale attribute and must be treated as first even if other fields look more variant-distinguishing.
The next usable template is the secondary SKU sale attribute target.
Choose at most one primary_source_dimension for the first template and at most one secondary_source_dimension for the next template.
Keep source dimension names unchanged. Do not choose the most variant-distinguishing source as primary unless it maps to the first SHEIN template.
When the first SHEIN template has no exact same-name source dimension, choose the structured source dimension whose meaning and values are the safest stable surrogate for that target attribute.
If the first template is Style Type / 款式 and the source has only Color and Size, Size usually belongs to the later Size template; choose the safest non-size structured source as the Style Type surrogate when semantically safe.
Do not invent source dimensions, do not use user free-form prompts, and avoid technical ids.
Do not reorder SHEIN templates based on source dimension names; map source dimensions onto the template order.
primary_attribute_id must equal the first SHEIN template attribute_id. secondary_attribute_id must equal the next selected SHEIN template attribute_id.
If there is no safe secondary mapping, leave secondary_source_dimension empty.
Return JSON only with keys primary_source_dimension, secondary_source_dimension, primary_attribute_id, secondary_attribute_id, reasons.

Source dimensions:
{{.SourceDimensionsBlock}}

SHEIN sale attribute templates:
{{.TemplatesBlock}}
{{.FeedbackBlock}}`, map[string]any{
		"SourceDimensionsBlock": sourceBuilder.String(),
		"TemplatesBlock":        templateBuilder.String(),
		"FeedbackBlock":         feedbackBlock,
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
