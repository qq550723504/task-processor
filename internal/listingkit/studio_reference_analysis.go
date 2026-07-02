package listingkit

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

const maxStudioReferenceAnalysisImages = 5

var (
	studioWordPattern               = regexp.MustCompile(`[a-z0-9]+`)
	studioQuotedTextPattern         = regexp.MustCompile(`["'][^"']+["']`)
	studioBrandMarkPattern          = regexp.MustCompile(`\b(?:logo|logos|brand mark|brand marks|wordmark|wordmarks|emblem|emblems|trefoil|swoosh)\b`)
	studioExactTextPattern          = regexp.MustCompile(`\b(?:exact text|exact slogan|same wording|copy this exact|quote|quoted|slogan|tagline|catchphrase)\b`)
	studioCharacterIdentityPattern  = regexp.MustCompile(`\b(?:characters?|face|faces|portrait|portraits|person|people|identity|identities|celebrity|celebrities|likeness)\b`)
	studioUniqueLayoutPattern       = regexp.MustCompile(`\b(?:same|exact|identical|signature|unique|distinctive)\s+[a-z0-9\s-]{0,40}?(?:layout|composition|arrangement|split|frame|badge)\b`)
	studioRepeatedWhitespacePattern = regexp.MustCompile(`\s+`)
)

var (
	studioMotifPhraseVocabulary = []string{
		"retro flowers",
		"western floral",
		"sports mascot",
		"floral border",
		"retro cherry",
	}
	studioMotifWordVocabulary = map[string]string{
		"retro":      "retro",
		"vintage":    "vintage",
		"western":    "western",
		"floral":     "floral",
		"flower":     "flower",
		"flowers":    "flowers",
		"botanical":  "botanical",
		"cherry":     "cherry",
		"sports":     "sports",
		"mascot":     "mascot",
		"border":     "border",
		"crest":      "crest",
		"badge":      "badge",
		"ornamental": "ornamental",
		"geometric":  "geometric",
		"abstract":   "abstract",
	}
	studioPalettePhraseVocabulary = []string{
		"cherry red",
		"off white",
		"forest green",
		"sky blue",
	}
	studioPaletteWordVocabulary = map[string]string{
		"cream":  "cream",
		"red":    "red",
		"navy":   "navy",
		"tan":    "tan",
		"black":  "black",
		"white":  "white",
		"blue":   "blue",
		"green":  "green",
		"pink":   "pink",
		"gold":   "gold",
		"silver": "silver",
		"brown":  "brown",
		"gray":   "gray",
		"grey":   "gray",
		"cherry": "cherry",
	}
	studioTypographyPhraseVocabulary = []string{
		"old english",
		"sans serif",
	}
	studioTypographyWordVocabulary = map[string]string{
		"bold":       "bold",
		"collegiate": "collegiate",
		"distressed": "distressed",
		"serif":      "serif",
		"script":     "script",
		"gothic":     "gothic",
		"vintage":    "vintage",
		"western":    "western",
		"block":      "block",
		"clean":      "clean",
	}
	studioDensityPhraseVocabulary = []string{
		"clean layering",
	}
	studioDensityWordVocabulary = map[string]string{
		"clean":    "clean",
		"layering": "layering",
		"layered":  "layered",
		"airy":     "airy",
		"dense":    "dense",
		"balanced": "balanced",
		"minimal":  "minimal",
		"bold":     "bold",
	}
	studioProductFitPhraseVocabulary = []string{
		"vintage streetwear",
	}
	studioProductFitWordVocabulary = map[string]string{
		"vintage":    "vintage",
		"streetwear": "streetwear",
		"casual":     "casual",
		"heritage":   "heritage",
		"classic":    "classic",
		"oversized":  "oversized",
		"unisex":     "unisex",
		"athletic":   "athletic",
	}
)

type studioReferenceImageAnalysis struct {
	Motif       string   `json:"motif,omitempty"`
	Palette     []string `json:"palette,omitempty"`
	Composition string   `json:"composition,omitempty"`
	Typography  string   `json:"typography,omitempty"`
	Density     string   `json:"density,omitempty"`
	ProductFit  string   `json:"product_fit,omitempty"`
	Avoid       []string `json:"avoid,omitempty"`
	Raw         string   `json:"-"`
}

type studioAbstractedReferenceAnalysis struct {
	Motif        string
	Palette      []string
	Composition  []string
	Typography   string
	Density      string
	ProductFit   string
	HadUnsafe    bool
	HadMalformed bool
}

func (s *service) AnalyzeStudioReferenceStyle(ctx context.Context, req *StudioReferenceAnalysisRequest) (*StudioReferenceAnalysisResponse, error) {
	return s.taskStudioMediaOrDefault().AnalyzeStudioReferenceStyle(ctx, req)
}

func (s *taskStudioMediaService) AnalyzeStudioReferenceStyle(ctx context.Context, req *StudioReferenceAnalysisRequest) (*StudioReferenceAnalysisResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("invalid request: request is required")
	}
	urls := normalizeStudioReferenceImageURLs(req.ReferenceImageURLs)
	if len(urls) == 0 {
		return nil, fmt.Errorf("invalid request: reference_image_urls is required")
	}
	if s == nil || s.promptDiversifier == nil {
		return nil, fmt.Errorf("reference_analysis_unavailable: studio reference analyzer is not configured")
	}

	warnings := make([]string, 0)
	if len(urls) > maxStudioReferenceAnalysisImages {
		urls = urls[:maxStudioReferenceAnalysisImages]
		warnings = append(warnings, "最多分析 5 张参考图，已忽略多余图片。")
	}

	analysisPrompt := buildStudioReferenceAnalysisPrompt(req)
	analyses := make([]studioReferenceImageAnalysis, 0, len(urls))
	for _, imageURL := range urls {
		raw, err := s.promptDiversifier.AnalyzeImage(ctx, imageURL, analysisPrompt)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("参考图分析失败：%s", compactStudioGenerationError(err)))
			continue
		}
		analyses = append(analyses, parseStudioReferenceImageAnalysis(raw))
	}
	if len(analyses) == 0 {
		return nil, fmt.Errorf("reference_analysis_failed: no reference image could be analyzed")
	}

	abstracted := abstractStudioReferenceAnalyses(analyses)
	brief := buildStudioReferenceStyleBrief(req, abstracted)
	sanitized := buildSanitizedStudioReferencePrompt(abstracted)
	if strings.TrimSpace(sanitized) == "" {
		return nil, fmt.Errorf("reference_analysis_failed: generated reference brief is empty")
	}

	if studioReferenceContainsUnsafeSignals(analyses, abstracted) {
		warnings = append(warnings, "已移除品牌、Logo、原文案或过于接近原图的描述。")
	}
	if studioReferenceContainsMalformedFallback(abstracted) {
		warnings = append(warnings, "部分参考图返回了非结构化分析结果，仅保留可安全复用的风格提示。")
	}

	return &StudioReferenceAnalysisResponse{
		ReferenceStyleBrief: brief,
		SanitizedPrompt:     sanitized,
		Warnings:            warnings,
	}, nil
}

func normalizeStudioReferenceImageURLs(urls []string) []string {
	seen := map[string]struct{}{}
	result := make([]string, 0, len(urls))
	for _, raw := range urls {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		result = append(result, trimmed)
	}
	return result
}

func buildStudioReferenceAnalysisPrompt(req *StudioReferenceAnalysisRequest) string {
	product := strings.TrimSpace(req.ProductName)
	category := strings.Join(req.CategoryPath, " > ")
	basePrompt := strings.TrimSpace(req.BasePrompt)
	userInstruction := strings.TrimSpace(req.UserInstruction)
	return strings.TrimSpace(fmt.Sprintf(`Analyze this ecommerce product image as a style reference for original POD artwork.
Return JSON only with keys: motif, palette, composition, typography, density, product_fit, avoid.
Extract broad reusable commercial style only. Do not ask to copy logos, brand marks, exact slogans, exact characters, faces, or the same unique layout.
Product: %s
Category: %s
Existing user theme: %s
User instruction: %s`, product, category, basePrompt, userInstruction))
}

func parseStudioReferenceImageAnalysis(raw string) studioReferenceImageAnalysis {
	cleaned := strings.TrimSpace(raw)
	var analysis studioReferenceImageAnalysis
	if err := json.Unmarshal([]byte(cleaned), &analysis); err != nil {
		return studioReferenceImageAnalysis{Raw: cleaned}
	}
	analysis.Raw = cleaned
	return analysis
}

func buildStudioReferenceStyleBrief(req *StudioReferenceAnalysisRequest, analyses []studioAbstractedReferenceAnalysis) string {
	parts := []string{"Hot-selling reference style direction for original POD artwork."}
	if product := strings.TrimSpace(req.ProductName); product != "" {
		parts = append(parts, "Base product: "+product+".")
	}
	if category := strings.TrimSpace(strings.Join(req.CategoryPath, " > ")); category != "" {
		parts = append(parts, "Category: "+category+".")
	}
	for _, item := range analyses {
		if item.Motif != "" {
			parts = append(parts, "Motif family: "+item.Motif+".")
		}
		if len(item.Palette) > 0 {
			parts = append(parts, "Palette direction: "+strings.Join(item.Palette, ", ")+".")
		}
		if len(item.Composition) > 0 {
			parts = append(parts, "Composition family: "+strings.Join(item.Composition, ", ")+".")
		}
		if item.Typography != "" {
			parts = append(parts, "Typography feel: "+item.Typography+".")
		}
		if item.Density != "" {
			parts = append(parts, "Visual density: "+item.Density+".")
		}
		if item.ProductFit != "" {
			parts = append(parts, "Product fit: "+item.ProductFit+".")
		}
	}
	parts = append(parts, "Create a new original design in this broad style family. Do not reproduce logos, brand marks, exact text, characters, faces, or the same unique layout from any reference image.")
	return strings.Join(parts, " ")
}

func buildSanitizedStudioReferencePrompt(analyses []studioAbstractedReferenceAnalysis) string {
	parts := []string{"Create an original POD artwork with a commercially proven graphic style direction."}
	if motifs := collectStudioReferenceFragments(analyses, func(item studioAbstractedReferenceAnalysis) string {
		return item.Motif
	}); len(motifs) > 0 {
		parts = append(parts, "Motif direction: "+strings.Join(motifs, ", ")+".")
	}
	if palettes := collectStudioReferencePalettes(analyses); len(palettes) > 0 {
		parts = append(parts, "Palette direction: "+strings.Join(palettes, ", ")+".")
	}
	if compositions := collectStudioReferenceCompositionFragments(analyses); len(compositions) > 0 {
		parts = append(parts, "Composition direction: "+strings.Join(compositions, ", ")+".")
	}
	if typography := collectStudioReferenceFragments(analyses, func(item studioAbstractedReferenceAnalysis) string {
		return item.Typography
	}); len(typography) > 0 {
		parts = append(parts, "Typography feel: "+strings.Join(typography, ", ")+".")
	}
	if density := collectStudioReferenceFragments(analyses, func(item studioAbstractedReferenceAnalysis) string {
		return item.Density
	}); len(density) > 0 {
		parts = append(parts, "Visual density: "+strings.Join(density, ", ")+".")
	}
	if productFit := collectStudioReferenceFragments(analyses, func(item studioAbstractedReferenceAnalysis) string {
		return item.ProductFit
	}); len(productFit) > 0 {
		parts = append(parts, "Product fit: "+strings.Join(productFit, ", ")+".")
	}
	parts = append(parts, "Keep all graphics brand-neutral, use fresh custom wording if text appears, avoid recognizable characters or people, and use a clearly original layout.")
	return strings.Join(parts, " ")
}

func collectStudioReferenceFragments(analyses []studioAbstractedReferenceAnalysis, pick func(studioAbstractedReferenceAnalysis) string) []string {
	result := make([]string, 0)
	seen := map[string]struct{}{}
	for _, item := range analyses {
		value := strings.TrimSpace(pick(item))
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func collectStudioReferencePalettes(analyses []studioAbstractedReferenceAnalysis) []string {
	result := make([]string, 0)
	seen := map[string]struct{}{}
	for _, item := range analyses {
		if len(item.Palette) == 0 {
			continue
		}
		palette := strings.Join(item.Palette, ", ")
		if _, ok := seen[palette]; ok {
			continue
		}
		seen[palette] = struct{}{}
		result = append(result, palette)
	}
	return result
}

func collectStudioReferenceCompositionFragments(analyses []studioAbstractedReferenceAnalysis) []string {
	result := make([]string, 0)
	seen := map[string]struct{}{}
	for _, item := range analyses {
		for _, composition := range item.Composition {
			composition = strings.TrimSpace(composition)
			if composition == "" {
				continue
			}
			if _, ok := seen[composition]; ok {
				continue
			}
			seen[composition] = struct{}{}
			result = append(result, composition)
		}
	}
	return result
}

func abstractStudioReferenceAnalyses(analyses []studioReferenceImageAnalysis) []studioAbstractedReferenceAnalysis {
	result := make([]studioAbstractedReferenceAnalysis, 0, len(analyses))
	for _, item := range analyses {
		abstracted := abstractStudioReferenceAnalysis(item)
		if abstracted.Motif == "" &&
			len(abstracted.Palette) == 0 &&
			len(abstracted.Composition) == 0 &&
			abstracted.Typography == "" &&
			abstracted.Density == "" &&
			abstracted.ProductFit == "" &&
			!(abstracted.HadUnsafe || abstracted.HadMalformed) {
			continue
		}
		result = append(result, abstracted)
	}
	return result
}

func abstractStudioReferenceAnalysis(item studioReferenceImageAnalysis) studioAbstractedReferenceAnalysis {
	abstracted := studioAbstractedReferenceAnalysis{
		Motif:       abstractStudioMotif(item.Motif),
		Palette:     abstractStudioPalette(item.Palette),
		Composition: abstractStudioComposition(item.Composition),
		Typography:  abstractStudioTypography(item.Typography),
		Density:     abstractStudioDensity(item.Density),
		ProductFit:  abstractStudioProductFit(item.ProductFit),
	}
	abstracted.HadMalformed = isMalformedStudioReferenceAnalysis(item)

	if abstracted.HadMalformed {
		if abstracted.Motif == "" {
			abstracted.Motif = abstractStudioMotif(item.Raw)
		}
		if len(abstracted.Palette) == 0 {
			abstracted.Palette = abstractStudioPalette([]string{item.Raw})
		}
		if len(abstracted.Composition) == 0 {
			abstracted.Composition = abstractStudioComposition(item.Raw)
		}
		if abstracted.Typography == "" {
			abstracted.Typography = abstractStudioTypography(item.Raw)
		}
		if abstracted.Density == "" {
			abstracted.Density = abstractStudioDensity(item.Raw)
		}
		if abstracted.ProductFit == "" {
			abstracted.ProductFit = abstractStudioProductFit(item.Raw)
		}
	}

	abstracted.HadUnsafe = studioReferenceFieldContainsUnsafeSignals(item.Raw)
	for _, avoid := range item.Avoid {
		if strings.TrimSpace(avoid) != "" {
			abstracted.HadUnsafe = true
		}
		if studioReferenceFieldContainsUnsafeSignals(avoid) {
			abstracted.HadUnsafe = true
		}
	}
	abstracted.HadUnsafe = abstracted.HadUnsafe || studioStructuredFieldWasDropped(item.Motif, abstracted.Motif)
	abstracted.HadUnsafe = abstracted.HadUnsafe || studioStructuredFieldWasDropped(item.Typography, abstracted.Typography)
	abstracted.HadUnsafe = abstracted.HadUnsafe || studioStructuredFieldWasDropped(item.Density, abstracted.Density)
	abstracted.HadUnsafe = abstracted.HadUnsafe || studioStructuredFieldWasDropped(item.ProductFit, abstracted.ProductFit)
	abstracted.HadUnsafe = abstracted.HadUnsafe || studioCompositionWasAbstracted(item.Composition, abstracted.Composition)
	return abstracted
}

func abstractStudioMotif(value string) string {
	return abstractStudioVocabularyPhrase(value, studioMotifPhraseVocabulary, studioMotifWordVocabulary)
}

func abstractStudioPalette(values []string) []string {
	result := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range values {
		abstracted := abstractStudioVocabularyPhrase(value, studioPalettePhraseVocabulary, studioPaletteWordVocabulary)
		if abstracted == "" {
			continue
		}
		if _, ok := seen[abstracted]; ok {
			continue
		}
		seen[abstracted] = struct{}{}
		result = append(result, abstracted)
	}
	return result
}

func abstractStudioTypography(value string) string {
	return abstractStudioVocabularyPhrase(value, studioTypographyPhraseVocabulary, studioTypographyWordVocabulary)
}

func abstractStudioDensity(value string) string {
	return abstractStudioVocabularyPhrase(value, studioDensityPhraseVocabulary, studioDensityWordVocabulary)
}

func abstractStudioProductFit(value string) string {
	return abstractStudioVocabularyPhrase(value, studioProductFitPhraseVocabulary, studioProductFitWordVocabulary)
}

func abstractStudioComposition(value string) []string {
	lower := strings.ToLower(strings.TrimSpace(value))
	if lower == "" {
		return nil
	}
	result := make([]string, 0, 2)
	add := func(label string) {
		for _, existing := range result {
			if existing == label {
				return
			}
		}
		result = append(result, label)
	}
	if strings.Contains(lower, "center") {
		add("centered composition")
	}
	if strings.Contains(lower, "badge") || strings.Contains(lower, "crest") || strings.Contains(lower, "seal") || strings.Contains(lower, "medallion") {
		add("badge composition")
	}
	if strings.Contains(lower, "frame") || strings.Contains(lower, "framed") || strings.Contains(lower, "arch") {
		add("framed composition")
	}
	if strings.Contains(lower, "border") {
		add("border composition")
	}
	if strings.Contains(lower, "repeat") || strings.Contains(lower, "pattern") || strings.Contains(lower, "allover") {
		add("repeat pattern composition")
	}
	if strings.Contains(lower, "split") || strings.Contains(lower, "diagonal") || strings.Contains(lower, "collage") || strings.Contains(lower, "offset") || strings.Contains(lower, "layered") {
		add("balanced dynamic composition")
	}
	return result
}

func abstractStudioVocabularyPhrase(value string, phrases []string, words map[string]string) string {
	lower := strings.ToLower(strings.TrimSpace(value))
	if lower == "" {
		return ""
	}
	tokens := studioWordPattern.FindAllString(lower, -1)
	if len(tokens) == 0 {
		return ""
	}
	result := make([]string, 0, len(tokens))
	seen := map[string]struct{}{}
	for i := 0; i < len(tokens); {
		if i+1 < len(tokens) {
			phrase := tokens[i] + " " + tokens[i+1]
			if containsStudioVocabularyPhrase(phrases, phrase) {
				if _, ok := seen[phrase]; !ok {
					seen[phrase] = struct{}{}
					result = append(result, phrase)
				}
				i += 2
				continue
			}
		}
		if mapped, ok := words[tokens[i]]; ok {
			if _, exists := seen[mapped]; !exists {
				seen[mapped] = struct{}{}
				result = append(result, mapped)
			}
		}
		i++
	}
	return studioRepeatedWhitespacePattern.ReplaceAllString(strings.Join(result, " "), " ")
}

func containsStudioVocabularyPhrase(phrases []string, target string) bool {
	for _, phrase := range phrases {
		if phrase == target {
			return true
		}
	}
	return false
}

func studioReferenceContainsUnsafeSignals(analyses []studioReferenceImageAnalysis, abstracted []studioAbstractedReferenceAnalysis) bool {
	for _, item := range analyses {
		if studioReferenceFieldContainsUnsafeSignals(item.Raw) {
			return true
		}
		for _, avoid := range item.Avoid {
			if strings.TrimSpace(avoid) != "" {
				return true
			}
		}
	}
	for _, item := range abstracted {
		if item.HadUnsafe {
			return true
		}
	}
	return false
}

func studioReferenceContainsMalformedFallback(analyses []studioAbstractedReferenceAnalysis) bool {
	for _, item := range analyses {
		if item.HadMalformed {
			return true
		}
	}
	return false
}

func studioReferenceFieldContainsUnsafeSignals(value string) bool {
	lower := strings.ToLower(strings.TrimSpace(value))
	if lower == "" {
		return false
	}
	return studioQuotedTextPattern.MatchString(value) ||
		studioBrandMarkPattern.MatchString(lower) ||
		studioExactTextPattern.MatchString(lower) ||
		studioCharacterIdentityPattern.MatchString(lower) ||
		studioUniqueLayoutPattern.MatchString(lower)
}

func studioStructuredFieldWasDropped(original string, abstracted string) bool {
	return strings.TrimSpace(original) != "" && strings.TrimSpace(abstracted) == ""
}

func studioCompositionWasAbstracted(original string, abstracted []string) bool {
	return strings.TrimSpace(original) != "" &&
		(strings.Contains(strings.ToLower(original), "split") ||
			strings.Contains(strings.ToLower(original), "diagonal") ||
			strings.Contains(strings.ToLower(original), "frame") ||
			strings.Contains(strings.ToLower(original), "arch") ||
			strings.Contains(strings.ToLower(original), "border") ||
			strings.Contains(strings.ToLower(original), "badge") ||
			strings.Contains(strings.ToLower(original), "center")) &&
		len(abstracted) > 0
}

func isMalformedStudioReferenceAnalysis(item studioReferenceImageAnalysis) bool {
	return item.Raw != "" &&
		item.Motif == "" &&
		len(item.Palette) == 0 &&
		item.Composition == "" &&
		item.Typography == "" &&
		item.Density == "" &&
		item.ProductFit == "" &&
		len(item.Avoid) == 0
}
