package shein

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	sheinpublishing "task-processor/internal/marketplace/shein/publishing"
	"task-processor/internal/pkg/jsonx"
)

const minSizeAttributeHeaderLLMConfidence = 0.7

type llmSizeAttributeHeaderResolver struct {
	llm TextGenerator
}

type sizeAttributeHeaderLLMResponse struct {
	Selections []sizeAttributeHeaderLLMSelection `json:"selections,omitempty"`
}

type sizeAttributeHeaderLLMSelection struct {
	Header      string   `json:"header,omitempty"`
	AttributeID int      `json:"attribute_id,omitempty"`
	Confidence  float64  `json:"confidence,omitempty"`
	Reasons     []string `json:"reasons,omitempty"`
}

func NewSizeAttributeHeaderResolver(llm TextGenerator) SizeAttributeHeaderResolver {
	if llm == nil {
		return nil
	}
	return &llmSizeAttributeHeaderResolver{llm: llm}
}

func (r *llmSizeAttributeHeaderResolver) ResolveSizeAttributeHeaders(input SizeAttributeHeaderResolutionInput) SizeAttributeHeaderResolution {
	if r == nil || r.llm == nil || len(input.Headers) == 0 || len(input.TemplateAttributes) == 0 {
		return SizeAttributeHeaderResolution{}
	}
	ctx := input.Context
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()

	response, err := r.llm.Generate(ctx, buildSizeAttributeHeaderPrompt(input))
	if err != nil {
		return SizeAttributeHeaderResolution{}
	}
	response = jsonx.CleanLLMResponse(response)
	if strings.TrimSpace(response) == "" {
		return SizeAttributeHeaderResolution{}
	}

	var parsed sizeAttributeHeaderLLMResponse
	if err := json.Unmarshal([]byte(response), &parsed); err != nil {
		return SizeAttributeHeaderResolution{}
	}
	if len(parsed.Selections) == 0 {
		return SizeAttributeHeaderResolution{}
	}

	headerByKey := indexSizeHeaderInputs(input.Headers)
	validAttributeIDs := indexSizeTemplateAttributeIDs(input.TemplateAttributes)
	result := SizeAttributeHeaderResolution{AttributeIDsByHeader: map[string]int{}}
	for _, selection := range parsed.Selections {
		header, ok := headerByKey[normalizeText(selection.Header)]
		if !ok || selection.AttributeID <= 0 || selection.Confidence < minSizeAttributeHeaderLLMConfidence {
			continue
		}
		if _, ok := validAttributeIDs[selection.AttributeID]; !ok {
			continue
		}
		result.AttributeIDsByHeader[header] = selection.AttributeID
		result.ReviewNotes = append(result.ReviewNotes, buildSizeAttributeHeaderLLMNote(header, selection))
	}
	if len(result.AttributeIDsByHeader) == 0 {
		return SizeAttributeHeaderResolution{}
	}
	result.ReviewNotes = dedupeStrings(result.ReviewNotes)
	return result
}

func buildSizeAttributeHeaderPrompt(input SizeAttributeHeaderResolutionInput) string {
	var headerBlock strings.Builder
	for _, header := range input.Headers {
		header = strings.TrimSpace(header)
		if header == "" {
			continue
		}
		headerBlock.WriteString(fmt.Sprintf("- header=%q\n", header))
	}

	var candidateBlock strings.Builder
	for _, attr := range input.TemplateAttributes {
		if attr.AttributeID <= 0 {
			continue
		}
		candidateBlock.WriteString(fmt.Sprintf(
			"- attribute_id=%d name=%q name_en=%q sort_order=%d\n",
			attr.AttributeID,
			attr.AttributeName,
			attr.AttributeNameEn,
			attr.SortOrder,
		))
	}

	return `You map SDS product size-table headers to SHEIN size-chart template attributes.
Choose only from the provided SHEIN candidate attribute_id values.
Use semantic meaning across Chinese and English names. Do not invent attribute IDs.
Skip uncertain headers by omitting them from selections.
Return JSON only with shape {"selections":[{"header":string,"attribute_id":number,"confidence":number,"reasons":[string]}]}.
confidence must be from 0 to 1 and should be at least 0.7 only for safe matches.

SDS headers:
` + headerBlock.String() + `
SHEIN size-chart candidates:
` + candidateBlock.String()
}

func indexSizeHeaderInputs(headers []string) map[string]string {
	result := map[string]string{}
	for _, header := range headers {
		header = strings.TrimSpace(header)
		if header == "" {
			continue
		}
		result[normalizeText(header)] = header
	}
	return result
}

func indexSizeTemplateAttributeIDs(attrs []sheinpublishing.SizeChartTemplateAttribute) map[int]struct{} {
	result := map[int]struct{}{}
	for _, attr := range attrs {
		if attr.AttributeID > 0 {
			result[attr.AttributeID] = struct{}{}
		}
	}
	return result
}

func buildSizeAttributeHeaderLLMNote(header string, selection sizeAttributeHeaderLLMSelection) string {
	reason := strings.Join(selection.Reasons, "; ")
	if strings.TrimSpace(reason) == "" {
		reason = fmt.Sprintf("confidence %.2f", selection.Confidence)
	}
	return fmt.Sprintf("LLM 尺码表字段匹配: %s -> attribute_id=%d (%s)", strings.TrimSpace(header), selection.AttributeID, reason)
}
