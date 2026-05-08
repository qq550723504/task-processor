package shein

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	openaiclient "task-processor/internal/infra/clients/openai"
	"task-processor/internal/pkg/jsonx"
	"task-processor/internal/prompt"
	sheinattribute "task-processor/internal/shein/api/attribute"
)

type saleAttributeValuePreparation struct {
	Original           string
	Effective          string
	PromptContaminated bool
	Sanitized          bool
	SanitizationSource string
	ResolutionNote     string
	NeedsManualReview  bool
}

type saleAttributeValueSummary struct {
	Sanitized          bool
	PromptContaminated bool
	Source             string
	Note               string
	NeedsManualReview  bool
}

type saleAttributeValueLLMExtraction struct {
	Value   string   `json:"value"`
	Reasons []string `json:"reasons,omitempty"`
}

var (
	saleAttributePromptValueCuePatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)\bplease\s+(design|create|generate)\b`),
		regexp.MustCompile(`(?i)\b(the\s+)?image\s+should\b`),
		regexp.MustCompile(`(?i)\b(include|ensure|avoid|need to|must)\b`),
		regexp.MustCompile(`(?i)\b\d{3,5}\s*(pixels?|px)\b`),
		regexp.MustCompile(`(?i)\b\d+\s*[x*]\s*\d+\b`),
		regexp.MustCompile(`帮我设计|请设计|生成|不要侵权|图片要有|视觉效果|像素`),
	}
	saleAttributeInstructionSplitPattern = regexp.MustCompile(`(?i)\b(?:please\s+(?:design|create|generate)|the\s+image\s+should|image\s+should|include|ensure|avoid|need to|must)\b`)
	saleAttributeCanvasNoisePattern      = regexp.MustCompile(`(?i)([,;:，；：]\s*)?(?:\d{3,5}\s*(?:pixels?|px)|\d+\s*[x*]\s*\d+).*$`)
)

func prepareSaleAttributeSourceValue(
	attr *sheinattribute.AttributeInfo,
	sourceDimension string,
	sourceValue string,
	productTitle string,
	llm openaiclient.ChatCompleter,
) saleAttributeValuePreparation {
	original := strings.TrimSpace(sourceValue)
	if original == "" {
		return saleAttributeValuePreparation{}
	}
	if !shouldExtractSaleAttributeSourceValue(sourceDimension, original) {
		return saleAttributeValuePreparation{
			Original:           original,
			Effective:          original,
			SanitizationSource: "direct",
		}
	}

	prepared := saleAttributeValuePreparation{
		Original:           original,
		PromptContaminated: true,
	}
	attributeName := ""
	if attr != nil {
		attributeName = firstNonEmpty(attr.AttributeNameEn, attr.AttributeName)
	}
	if trimmed := trimPromptLikeSaleAttributeValue(original); trimmed != "" {
		prepared.Effective = trimmed
		prepared.Sanitized = true
		prepared.SanitizationSource = "rule_trimmed"
		prepared.ResolutionNote = "prompt-like " + sourceDimension + " replaced by rule-trimmed " + attributeName + " value"
		return prepared
	}
	if extracted := extractSaleAttributeValueWithLLM(attributeName, sourceDimension, original, productTitle, llm); extracted != "" {
		prepared.Effective = extracted
		prepared.Sanitized = true
		prepared.SanitizationSource = "llm_extracted"
		prepared.ResolutionNote = "prompt-like " + sourceDimension + " replaced by llm-extracted " + attributeName + " value"
		return prepared
	}
	prepared.NeedsManualReview = true
	prepared.ResolutionNote = "prompt-like " + sourceDimension + " could not be reduced to a safe short " + attributeName + " value; manual review required"
	return prepared
}

func isPromptLikeSaleAttributeValue(value string) bool {
	value = cleanListingText(value)
	if value == "" {
		return false
	}
	if strings.Count(value, ".") >= 2 {
		return true
	}
	lower := strings.ToLower(value)
	score := 0
	for _, pattern := range saleAttributePromptValueCuePatterns {
		if pattern.MatchString(lower) {
			score++
		}
	}
	wordCount := len(strings.Fields(lower))
	switch {
	case score >= 2:
		return true
	case score == 1 && wordCount >= 10:
		return true
	case wordCount >= 14 && (strings.Contains(lower, "please") || strings.Contains(lower, "设计")):
		return true
	default:
		return false
	}
}

func shouldExtractSaleAttributeSourceValue(sourceDimension string, value string) bool {
	value = cleanListingText(value)
	if value == "" {
		return false
	}
	if isPromptLikeSaleAttributeValue(value) {
		return true
	}
	if !isAIStyleSourceDimension(sourceDimension) {
		return false
	}
	return containsCJK(value) || !looksLikeCompactSaleAttributeValue(value)
}

func trimPromptLikeSaleAttributeValue(sourceValue string) string {
	value := cleanListingText(sourceValue)
	if value == "" {
		return ""
	}
	value = saleAttributeCanvasNoisePattern.ReplaceAllString(value, "")
	if strings.Contains(value, " - ") {
		head, tail, ok := strings.Cut(value, " - ")
		if ok && isPromptLikeSaleAttributeValue(tail) {
			value = head
		}
	}
	if loc := saleAttributeInstructionSplitPattern.FindStringIndex(value); len(loc) == 2 && loc[0] > 0 {
		value = strings.TrimSpace(value[:loc[0]])
	}
	value = sanitizeResolvedTitle(value)
	if !looksLikeCompactSaleAttributeValue(value) || isPromptLikeSaleAttributeValue(value) {
		return ""
	}
	return trimShortTitle(value, 50, 6)
}

func looksLikeCompactSaleAttributeValue(value string) bool {
	value = cleanListingText(value)
	if value == "" {
		return false
	}
	if strings.Count(value, ".") > 0 {
		return false
	}
	if len(value) > 50 {
		return false
	}
	words := strings.Fields(value)
	if len(words) == 0 || len(words) > 6 {
		return false
	}
	if strings.ContainsAny(value, "\n\r\t") {
		return false
	}
	return true
}

func extractSaleAttributeValueWithLLM(attributeName string, sourceDimension string, sourceValue string, productTitle string, llm openaiclient.ChatCompleter) string {
	if llm == nil {
		return ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()

	promptText := renderSheinSaleAttributePrompt(prompt.KSheinSaleAttributePromptValueExtraction, `You extract a short e-commerce sale attribute value from a noisy image-generation prompt.
Return JSON only with shape {"value":"...","reasons":["..."]}.
Rules:
1. Output one short English attribute value only.
2. Do not output sentences, commands, copyright notes, size/canvas instructions, or punctuation-heavy text.
3. Prefer 2-6 words and under 50 characters.
4. Keep theme/style semantics when obvious, but do not invent detailed marketing copy.

Attribute name: {{.AttributeName}}
Source dimension: {{.SourceDimension}}
Product title: {{.ProductTitle}}
Prompt-like source value: {{.SourceValue}}`, map[string]any{
		"AttributeName":   fmt.Sprintf("%q", attributeName),
		"SourceDimension": fmt.Sprintf("%q", sourceDimension),
		"ProductTitle":    fmt.Sprintf("%q", cleanListingText(productTitle)),
		"SourceValue":     fmt.Sprintf("%q", cleanListingText(sourceValue)),
	})

	response, err := llm.Generate(ctx, promptText)
	if err != nil {
		return ""
	}
	response = jsonx.CleanLLMResponse(response)
	if strings.TrimSpace(response) == "" {
		return ""
	}
	var parsed saleAttributeValueLLMExtraction
	if err := json.Unmarshal([]byte(response), &parsed); err != nil {
		return ""
	}
	value := sanitizeResolvedTitle(parsed.Value)
	if value == "" || containsCJK(value) || isPromptLikeSaleAttributeValue(value) || !looksLikeCompactSaleAttributeValue(value) {
		return ""
	}
	return trimShortTitle(value, 50, 6)
}

func mergeSaleAttributeValueSummary(summary *saleAttributeValueSummary, prepared saleAttributeValuePreparation) {
	if summary == nil {
		return
	}
	if prepared.PromptContaminated {
		summary.PromptContaminated = true
	}
	if prepared.NeedsManualReview {
		summary.NeedsManualReview = true
	}
	if prepared.Sanitized {
		summary.Sanitized = true
		if shouldReplaceSaleAttributeSummarySource(summary.Source, prepared.SanitizationSource) {
			summary.Source = prepared.SanitizationSource
		}
	}
	appendSaleAttributeSummaryNote(summary, prepared.ResolutionNote)
}

func shouldReplaceSaleAttributeSummarySource(current string, next string) bool {
	if next == "" {
		return false
	}
	if current == "" {
		return true
	}
	priority := map[string]int{
		"direct":        0,
		"rule_trimmed":  1,
		"llm_extracted": 2,
	}
	return priority[next] > priority[current]
}

func appendSaleAttributeSummaryNote(summary *saleAttributeValueSummary, note string) {
	if summary == nil || note == "" {
		return
	}
	if summary.Note == "" {
		summary.Note = note
		return
	}
	if !strings.Contains(summary.Note, note) {
		summary.Note += "; " + note
	}
}

func hasBlockedSaleAttributeValue(summary saleAttributeValueSummary) bool {
	return summary.PromptContaminated && summary.NeedsManualReview
}

func buildBlockedSaleAttributeValueNote(attributeName, sourceDimension, original string) string {
	return fmt.Sprintf(
		"SHEIN 销售属性值待人工确认: 源维度 %q 的值 %q 无法安全提炼为模板属性 %q 的短规格值",
		sourceDimension,
		original,
		firstNonEmpty(attributeName, sourceDimension),
	)
}

func buildSaleAttributeManualReviewNote(attributeName, sourceDimension string) string {
	return fmt.Sprintf(
		"SHEIN 销售属性值待人工确认: 模板属性 %q 当前存在 prompt-like 源值，未自动回填脏文本",
		firstNonEmpty(attributeName, sourceDimension),
	)
}

func applySaleAttributeValueSummary(resolution *SaleAttributeResolution, summary saleAttributeValueSummary) {
	if resolution == nil {
		return
	}
	if summary.PromptContaminated {
		resolution.ValuePromptContaminated = true
	}
	if summary.Sanitized {
		resolution.ValueSanitized = true
		resolution.ValueSanitizationSource = firstNonEmpty(summary.Source, resolution.ValueSanitizationSource)
	}
	if strings.TrimSpace(summary.Note) != "" {
		resolution.ValueResolutionNote = strings.TrimSpace(summary.Note)
	}
}

func saleAttributeResolutionHasPromptLikeValues(resolution *SaleAttributeResolution) (string, bool) {
	if resolution == nil {
		return "", false
	}
	for _, attr := range resolution.SKCAttributes {
		if saleAttributeValueUnsafeForCache(attr.Value) {
			return "cached primary sale attribute value is not a safe short value", true
		}
	}
	for _, attr := range resolution.SKUAttributes {
		if saleAttributeValueUnsafeForCache(attr.Value) {
			return "cached secondary sale attribute value is not a safe short value", true
		}
	}
	for _, attr := range resolution.SKCValueAssignments {
		if saleAttributeValueUnsafeForCache(attr.Value) {
			return "cached primary sale attribute value assignment is not a safe short value", true
		}
	}
	for sourceValue := range resolution.SKCValueAssignments {
		if isPromptLikeSaleAttributeValue(sourceValue) {
			return "cached primary sale attribute source value assignment is prompt-like", true
		}
	}
	for _, attr := range resolution.SKUValueAssignments {
		if saleAttributeValueUnsafeForCache(attr.Value) {
			return "cached secondary sale attribute value assignment is not a safe short value", true
		}
	}
	for sourceValue := range resolution.SKUValueAssignments {
		if isPromptLikeSaleAttributeValue(sourceValue) {
			return "cached secondary sale attribute source value assignment is prompt-like", true
		}
	}
	return "", false
}

func saleAttributeValueUnsafeForCache(value string) bool {
	value = cleanListingText(value)
	if value == "" {
		return false
	}
	return containsCJK(value) || isPromptLikeSaleAttributeValue(value) || !looksLikeCompactSaleAttributeValue(value)
}
