package shein

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"task-processor/internal/catalog/canonical"
	"task-processor/internal/pkg/jsonx"
	"task-processor/internal/pkg/timeout"
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

type titleAdditionExtraction struct {
	Addition string `json:"addition"`
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

func resolveListingTitle(ctx context.Context, canonical *canonical.Product, fallbackTitle string, aiClient TextGenerator) titleResolution {
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
		if extracted := extractPromptTitleWithLLM(ctx, value, canonical, fallbackTitle, aiClient); extracted != "" {
			return buildResolvedTitle(extracted, "prompt_extracted_llm", "prompt-like "+candidate.source+" replaced by llm-extracted title", true, canonical, fallbackTitle)
		}
		return titleResolution{
			title:       "",
			source:      "unresolved_prompt_title",
			note:        "prompt-like title candidates could not be safely resolved; llm title extraction unavailable or unsafe",
			contaminate: true,
			skcBase:     "",
		}
	}
	title := structuredFallbackTitle(canonical, fallbackTitle)
	return buildResolvedTitle(title, "structured_fallback", "", false, canonical, fallbackTitle)
}

func enrichResolvedListingTitle(ctx context.Context, resolution titleResolution, canonical *canonical.Product, fallbackTitle string, aiClient TextGenerator) titleResolution {
	if !shouldEnrichListingTitle(resolution.title) {
		return resolution
	}
	addition := extractListingTitleAdditionWithLLM(ctx, resolution.title, canonical, fallbackTitle, aiClient)
	if addition == "" {
		return resolution
	}
	enrichedTitle := mergeListingTitleWithAddition(resolution.title, addition)
	if enrichedTitle == "" || isPromptLikeTitle(enrichedTitle) || containsCJK(enrichedTitle) {
		return resolution
	}
	resolution.title = enrichedTitle
	resolution.skcBase = buildSKCBaseTitle(enrichedTitle, canonical, fallbackTitle)
	if strings.TrimSpace(resolution.note) == "" {
		resolution.note = "short structured title enriched with llm-extracted prompt elements"
	} else {
		resolution.note = strings.TrimSpace(resolution.note) + "; short structured title enriched with llm-extracted prompt elements"
	}
	return resolution
}

func buildResolvedTitle(title, source, note string, contaminated bool, canonical *canonical.Product, fallbackTitle string) titleResolution {
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

func shouldEnrichListingTitle(title string) bool {
	title = cleanListingText(title)
	if title == "" || isPromptLikeTitle(title) {
		return false
	}
	if strings.Contains(strings.ToLower(title), " with ") {
		return false
	}
	words := strings.Fields(title)
	return len(words) > 0 && len(words) <= 3
}

func sanitizeResolvedTitle(value string) string {
	value = sanitizeListingCopy(cleanListingText(value))
	value = strings.Trim(value, " -_,.;:/")
	return cleanListingText(value)
}

func sanitizeListingTitleAddition(value string) string {
	value = sanitizeResolvedTitle(value)
	if value == "" || containsCJK(value) || isPromptLikeTitle(value) {
		return ""
	}
	if strings.Count(value, ".") > 0 || strings.ContainsAny(value, "\n\r\t") {
		return ""
	}
	words := strings.Fields(value)
	if len(words) == 0 || len(words) > 8 {
		return ""
	}
	if len(value) > 64 {
		return ""
	}
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

func extractPromptTitleByRules(promptText string, canonical *canonical.Product, fallbackTitle string) string {
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

func buildSKCBaseTitle(title string, canonical *canonical.Product, fallbackTitle string) string {
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

func collectListingTitlePromptSignals(canonical *canonical.Product) []string {
	values := []string{
		lookupVariantAttribute(canonical, "ai_style"),
		lookupCanonicalAttribute(canonical, "picture_request"),
		lookupTechnicalSpec(canonical, "picture_request"),
		lookupCanonicalAttribute(canonical, "product_english_name"),
		lookupCanonicalAttribute(canonical, "english_name"),
	}
	seen := map[string]struct{}{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = cleanListingText(value)
		if value == "" {
			continue
		}
		key := normalizeText(value)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, value)
	}
	return result
}

func extractListingTitleAdditionWithLLM(ctx context.Context, baseTitle string, canonical *canonical.Product, fallbackTitle string, aiClient TextGenerator) string {
	if aiClient == nil {
		return ""
	}
	signals := collectListingTitlePromptSignals(canonical)
	if len(signals) == 0 {
		return ""
	}
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := timeout.WithAIShortTimeout(ctx)
	defer cancel()
	productType := inferEnglishProductType(canonical, fallbackTitle)
	systemPrompt := `You improve concise e-commerce product titles by extracting a short title addition from print-design instructions.
Return strict JSON only: {"addition":"..."}.
Rules:
1. "addition" must be 3-8 English words and under 64 characters.
2. Keep only concise design/theme/style/pattern elements that help an e-commerce title.
3. Prefer a richer but still compact phrase, combining theme plus visual/pattern cue when obvious.
4. Do not repeat the base product type, material, or size.
5. Do not include sentences, prompt instructions, dimensions, pixels, copyright notes, or platform filler words.
5. Leave "addition" empty if there is no safe concise addition.`
	systemPrompt += tenantGenerationTopicPolicyPromptBlock(ctx)
	userPrompt := fmt.Sprintf(
		"Base title: %s\nFallback product type: %s\nPrompt-like or style signals:\n- %s\nExtract one short addition that makes the base title more suitable for e-commerce.",
		cleanListingText(baseTitle),
		cleanListingText(productType),
		strings.Join(signals, "\n- "),
	)
	content, err := aiClient.Generate(ctx, systemPrompt+"\n\n"+userPrompt)
	if err != nil {
		return ""
	}
	var parsed titleAdditionExtraction
	if err := jsonx.UnmarshalString(jsonx.CleanLLMResponse(content), &parsed, "parse SHEIN title addition extraction"); err != nil {
		return ""
	}
	addition := sanitizeListingTitleAddition(parsed.Addition)
	if addition == "" {
		return ""
	}
	if titleAdditionRedundantWithBase(baseTitle, addition) {
		return ""
	}
	return addition
}

func titleAdditionRedundantWithBase(baseTitle string, addition string) bool {
	baseWords := map[string]struct{}{}
	for _, word := range strings.Fields(normalizeText(baseTitle)) {
		if word != "" {
			baseWords[word] = struct{}{}
		}
	}
	additionWords := strings.Fields(normalizeText(addition))
	if len(additionWords) == 0 {
		return true
	}
	for _, word := range additionWords {
		if _, exists := baseWords[word]; !exists {
			return false
		}
	}
	return true
}

func mergeListingTitleWithAddition(baseTitle string, addition string) string {
	baseTitle = cleanListingText(baseTitle)
	addition = sanitizeListingTitleAddition(addition)
	if baseTitle == "" || addition == "" {
		return baseTitle
	}
	return trimShortTitle(cleanListingText(baseTitle+" with "+addition), 90, 12)
}

func extractPromptTitleWithLLM(ctx context.Context, promptText string, canonical *canonical.Product, fallbackTitle string, aiClient TextGenerator) string {
	if aiClient == nil {
		return ""
	}
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := timeout.WithAIShortTimeout(ctx)
	defer cancel()
	productType := inferEnglishProductType(canonical, fallbackTitle)
	systemPrompt := `You extract short e-commerce product titles from noisy image-generation prompts.
Return strict JSON only: {"title":"..."}.
Rules:
1. Title must be a concise English product name.
2. Keep useful product semantics such as product type, material, style, or theme when obvious.
3. Never include instruction phrases, copyright notes, generation requirements, size/canvas instructions, or sentence-style prompt text.
4. Prefer 3-10 words and under 80 characters.`
	systemPrompt += tenantGenerationTopicPolicyPromptBlock(ctx)
	userPrompt := fmt.Sprintf("Fallback product title: %s\nPrompt-like source text: %s\nExtract a short product title.", productType, cleanListingText(promptText))
	content, err := aiClient.Generate(ctx, systemPrompt+"\n\n"+userPrompt)
	if err != nil {
		return ""
	}
	type llmTitle struct {
		Title string `json:"title"`
	}
	var parsed llmTitle
	if err := jsonx.UnmarshalString(jsonx.CleanLLMResponse(content), &parsed, "parse SHEIN title extraction"); err != nil {
		return ""
	}
	title := sanitizeResolvedTitle(parsed.Title)
	if title == "" || containsCJK(title) || isPromptLikeTitle(title) {
		return ""
	}
	return trimShortTitle(title, 80, 10)
}

func structuredFallbackTitle(canonical *canonical.Product, fallbackTitle string) string {
	candidates := []string{
		firstEnglishCandidate(
			lookupCanonicalAttribute(canonical, "product_english_name"),
			lookupCanonicalAttribute(canonical, "english_name"),
		),
		sanitizeResolvedTitle(inferEnglishProductType(canonical, fallbackTitle)),
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
