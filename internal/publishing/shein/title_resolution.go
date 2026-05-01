package shein

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	openaiclient "task-processor/internal/infra/clients/openai"
	"task-processor/internal/pkg/jsonx"
	"task-processor/internal/pkg/timeout"
	"task-processor/internal/productenrich"
)

type titleCandidate struct {
	source string
	value  string
}

type titleResolution struct {
	title       string
	source      string
	note        string
	contaminate bool
	skcBase     string
}

var (
	titlePromptCuePatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)\bplease\s+(design|create|generate)\b`),
		regexp.MustCompile(`(?i)\b(the\s+)?image\s+should\b`),
		regexp.MustCompile(`(?i)\b(include|ensure|avoid|need to|must)\b`),
		regexp.MustCompile(`(?i)\b\d{3,5}\s*(pixels?|px)\b`),
		regexp.MustCompile(`(?i)\b\d+\s*[x*]\s*\d+\b`),
		regexp.MustCompile(`帮我设计|请设计|生成|不要侵权|像素`),
	}
	titlePromptSplitPattern = regexp.MustCompile(`(?i)\b(?:please\s+(?:design|create|generate)|the\s+image\s+should|image\s+should|include|ensure|avoid|need to|must)\b`)
	titleCanvasNoisePattern = regexp.MustCompile(`(?i)([,;:，；：]\s*)?(?:\d{3,5}\s*(?:pixels?|px)|\d+\s*[x*]\s*\d+).*$`)
)

func resolveListingTitle(canonical *productenrich.CanonicalProduct, fallbackTitle string, aiClient openaiclient.ChatCompleter) titleResolution {
	candidates := []titleCandidate{
		{source: "product_english_name", value: lookupCanonicalAttribute(canonical, "product_english_name")},
		{source: "english_name", value: lookupCanonicalAttribute(canonical, "english_name")},
		{source: "canonical_title", value: fallbackTitle},
	}
	var contaminated bool
	for _, candidate := range candidates {
		value := cleanListingText(candidate.value)
		if value == "" || containsCJK(value) {
			continue
		}
		if !isPromptLikeTitle(value) {
			note := ""
			if contaminated {
				note = "prompt-like higher-priority title candidate replaced by " + candidate.source
			}
			return buildResolvedTitle(value, candidate.source, note, contaminated, canonical, fallbackTitle)
		}
		contaminated = true
		if extracted := extractPromptTitleByRules(value, canonical, fallbackTitle); extracted != "" {
			return buildResolvedTitle(extracted, "prompt_extracted_rule", "prompt-like "+candidate.source+" replaced by rule-extracted title", true, canonical, fallbackTitle)
		}
		if extracted := extractPromptTitleWithLLM(value, canonical, fallbackTitle, aiClient); extracted != "" {
			return buildResolvedTitle(extracted, "prompt_extracted_llm", "prompt-like "+candidate.source+" replaced by llm-extracted title", true, canonical, fallbackTitle)
		}
	}
	title := structuredFallbackTitle(canonical, fallbackTitle)
	note := ""
	if contaminated {
		note = "prompt-like title candidates fell back to structured title"
	}
	return buildResolvedTitle(title, "structured_fallback", note, contaminated, canonical, fallbackTitle)
}

func buildResolvedTitle(title, source, note string, contaminated bool, canonical *productenrich.CanonicalProduct, fallbackTitle string) titleResolution {
	title = sanitizeResolvedTitle(title)
	if title == "" || containsCJK(title) || isPromptLikeTitle(title) {
		title = structuredFallbackTitle(canonical, fallbackTitle)
		source = "structured_fallback"
		if contaminated && note == "" {
			note = "prompt-like title candidates fell back to structured title"
		}
	}
	return titleResolution{
		title:       title,
		source:      source,
		note:        note,
		contaminate: contaminated,
		skcBase:     buildSKCBaseTitle(title, canonical, fallbackTitle),
	}
}

func sanitizeResolvedTitle(value string) string {
	value = sanitizeListingCopy(cleanListingText(value))
	value = strings.Trim(value, " -_,.;:/")
	return cleanListingText(value)
}

func isPromptLikeTitle(value string) bool {
	value = cleanListingText(value)
	if value == "" {
		return false
	}
	if strings.Count(value, ".") >= 2 {
		return true
	}
	lower := strings.ToLower(value)
	score := 0
	for _, pattern := range titlePromptCuePatterns {
		if pattern.MatchString(lower) {
			score++
		}
	}
	wordCount := len(strings.Fields(lower))
	switch {
	case score >= 2:
		return true
	case score == 1 && wordCount >= 12:
		return true
	case wordCount >= 18 && (strings.Contains(lower, "please") || strings.Contains(lower, "设计")):
		return true
	default:
		return false
	}
}

func extractPromptTitleByRules(promptText string, canonical *productenrich.CanonicalProduct, fallbackTitle string) string {
	value := cleanListingText(promptText)
	value = titleCanvasNoisePattern.ReplaceAllString(value, "")
	if strings.Contains(value, " - ") {
		head, tail, ok := strings.Cut(value, " - ")
		if ok && isPromptLikeTitle(tail) {
			value = head
		}
	}
	if loc := titlePromptSplitPattern.FindStringIndex(value); len(loc) == 2 && loc[0] > 0 {
		value = strings.TrimSpace(value[:loc[0]])
	}
	value = sanitizeResolvedTitle(value)
	if value != "" && !isPromptLikeTitle(value) && !containsCJK(value) {
		return trimShortTitle(value, 80, 10)
	}
	return ""
}

func buildSKCBaseTitle(title string, canonical *productenrich.CanonicalProduct, fallbackTitle string) string {
	base := sanitizeResolvedTitle(title)
	if base == "" || isPromptLikeTitle(base) {
		base = structuredFallbackTitle(canonical, fallbackTitle)
	}
	if base == "" {
		return ""
	}
	return trimShortTitle(base, 70, 8)
}

func trimShortTitle(value string, maxChars int, maxWords int) string {
	words := strings.Fields(cleanListingText(value))
	if len(words) > maxWords {
		words = words[:maxWords]
	}
	value = strings.Join(words, " ")
	if len(value) <= maxChars {
		return value
	}
	truncated := value[:maxChars]
	if idx := strings.LastIndex(truncated, " "); idx > 0 {
		truncated = truncated[:idx]
	}
	return cleanListingText(truncated)
}

func extractPromptTitleWithLLM(promptText string, canonical *productenrich.CanonicalProduct, fallbackTitle string, aiClient openaiclient.ChatCompleter) string {
	if aiClient == nil {
		return ""
	}
	ctx, cancel := timeout.WithAIShortTimeout(context.Background())
	defer cancel()
	productType := inferEnglishProductType(canonical, fallbackTitle)
	systemPrompt := `You extract short e-commerce product titles from noisy image-generation prompts.
Return strict JSON only: {"title":"..."}.
Rules:
1. Title must be a concise English product name.
2. Keep useful product semantics such as product type, material, style, or theme when obvious.
3. Never include instruction phrases, copyright notes, generation requirements, size/canvas instructions, or sentence-style prompt text.
4. Prefer 3-10 words and under 80 characters.`
	userPrompt := fmt.Sprintf("Fallback product title: %s\nPrompt-like source text: %s\nExtract a short product title.", productType, cleanListingText(promptText))
	temperature := float32(0.2)
	resp, err := aiClient.CreateChatCompletion(ctx, &openaiclient.ChatCompletionRequest{
		Model:       aiClient.GetDefaultModel(),
		Temperature: &temperature,
		Messages: []openaiclient.ChatCompletionMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
	})
	if err != nil || resp == nil || len(resp.Choices) == 0 {
		return ""
	}
	type llmTitle struct {
		Title string `json:"title"`
	}
	var parsed llmTitle
	if err := jsonx.UnmarshalString(jsonx.CleanLLMResponse(resp.Choices[0].Message.Content), &parsed, "parse SHEIN title extraction"); err != nil {
		return ""
	}
	title := sanitizeResolvedTitle(parsed.Title)
	if title == "" || containsCJK(title) || isPromptLikeTitle(title) {
		return ""
	}
	return trimShortTitle(title, 80, 10)
}

func structuredFallbackTitle(canonical *productenrich.CanonicalProduct, fallbackTitle string) string {
	candidates := []string{
		firstEnglishCandidate(
			lookupCanonicalAttribute(canonical, "product_english_name"),
			lookupCanonicalAttribute(canonical, "english_name"),
		),
		sanitizeResolvedTitle(inferEnglishProductType(canonical, fallbackTitle)),
		sanitizeResolvedTitle(synthesizeEnglishTitle(canonical, fallbackTitle)),
		sanitizeResolvedTitle(fallbackTitle),
	}
	for _, candidate := range candidates {
		candidate = sanitizeResolvedTitle(candidate)
		if candidate == "" || containsCJK(candidate) || isPromptLikeTitle(candidate) {
			continue
		}
		return trimShortTitle(candidate, 80, 10)
	}
	return ""
}
