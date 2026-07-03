package listingkit

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

const maxStudioReferenceAnalysisImages = 1

var (
	studioWordPattern               = regexp.MustCompile(`[a-z0-9]+`)
	studioCapitalizedPhrasePattern  = regexp.MustCompile(`\b(?:[A-Z][a-z0-9]*)(?:\s+[A-Z][a-z0-9]*){0,2}\b`)
	studioQuotedTextPattern         = regexp.MustCompile(`["'][^"']+["']`)
	studioProtectedIdentityTerms    = []string{"hello kitty", "adidas", "nike", "mickey mouse", "taylor swift", "elsa", "old navy"}
	studioProtectedIdentityPattern  = regexp.MustCompile(`(?i)\b(?:hello\s+kitty|adidas|nike|mickey\s+mouse|taylor\s+swift|elsa|old\s+navy)\b`)
	studioBrandMarkPattern          = regexp.MustCompile(`\b(?:logo|logos|brand mark|brand marks|wordmark|wordmarks|emblem|emblems|trefoil|swoosh)\b`)
	studioExactTextPattern          = regexp.MustCompile(`\b(?:exact text|exact slogan|same wording|copy this exact|quote|quoted|slogan|tagline|catchphrase)\b`)
	studioExactArtworkPattern       = regexp.MustCompile(`\b(?:exact artwork|same artwork|source artwork|original artwork)\b`)
	studioCharacterIdentityPattern  = regexp.MustCompile(`\b(?:characters?|face|faces|portrait|portraits|person|people|identity|identities|celebrity|celebrities|likeness)\b`)
	studioUniqueLayoutPattern       = regexp.MustCompile(`\b(?:same|exact|identical|signature|unique|distinctive)\s+[a-z0-9\s-]{0,40}?(?:layout|composition|arrangement|split|frame|badge)\b`)
	studioWatermarkPattern          = regexp.MustCompile(`\b(?:watermark|watermarks)\b`)
	studioUnsafeSpacerPattern       = regexp.MustCompile(`[\(\)\[\]\{\}:,;|]+`)
	studioRepeatedWhitespacePattern = regexp.MustCompile(`\s+`)
)

var (
	studioSafeDescriptorWords = map[string]struct{}{
		"abstract": {}, "airy": {}, "allover": {}, "arched": {}, "art": {}, "artwork": {}, "balanced": {}, "badge": {}, "beach": {},
		"black": {}, "block": {}, "blue": {}, "bold": {}, "border": {}, "botanical": {}, "brown": {}, "brush": {}, "center": {}, "centered": {}, "cherry": {},
		"clean": {}, "coastal": {}, "coral": {}, "cream": {}, "gray": {}, "green": {}, "grey": {}, "navy": {}, "old": {}, "orange": {},
		"left": {}, "right": {}, "front": {}, "back": {}, "chest": {}, "large": {}, "sleeve": {}, "placement": {}, "mood": {}, "nostalgic": {},
		"pink": {}, "red": {}, "silver": {}, "white": {}, "gold": {},
		"collegiate": {}, "composition": {}, "crest": {}, "dense": {}, "distressed": {}, "drawn": {}, "dynamic": {},
		"english": {}, "floral": {}, "flower": {}, "flowers": {}, "frame": {}, "framed": {}, "geometric": {}, "glass": {}, "gothic": {},
		"gradient": {}, "hand": {}, "heritage": {}, "illustration": {}, "koi": {}, "layered": {}, "layering": {}, "layout": {},
		"lettering": {}, "linework": {}, "mascot": {}, "medallion": {}, "minimal": {}, "minimalist": {}, "modern": {}, "ombre": {},
		"ornamental": {}, "palette": {}, "pattern": {}, "playful": {}, "repeat": {}, "resort": {}, "retro": {}, "rounded": {},
		"sans": {}, "sea": {}, "seal": {}, "serif": {}, "sky": {}, "sports": {}, "streetwear": {}, "sunset": {}, "teal": {},
		"tan": {}, "texture": {}, "tropical": {}, "vintage": {}, "watercolor": {}, "wave": {}, "wear": {}, "western": {},
	}
	studioMotifPhraseVocabulary = []string{
		"koi wave",
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
		"koi":        "koi",
		"wave":       "wave",
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
		"teal":   "teal",
		"orange": "orange",
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
		"brush lettering",
		"old english",
		"sans serif",
	}
	studioTypographyWordVocabulary = map[string]string{
		"bold":       "bold",
		"brush":      "brush",
		"lettering":  "lettering",
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
		"resort wear",
		"vintage streetwear",
	}
	studioProductFitWordVocabulary = map[string]string{
		"resort":     "resort",
		"wear":       "wear",
		"vintage":    "vintage",
		"streetwear": "streetwear",
		"casual":     "casual",
		"heritage":   "heritage",
		"classic":    "classic",
		"oversized":  "oversized",
		"unisex":     "unisex",
		"athletic":   "athletic",
	}
	studioSafeTitleCasePhraseSet = buildStudioSafeTitleCasePhraseSet(
		studioMotifPhraseVocabulary,
		studioPalettePhraseVocabulary,
		studioTypographyPhraseVocabulary,
		studioDensityPhraseVocabulary,
		studioProductFitPhraseVocabulary,
	)
)

type studioReferenceImageAnalysis struct {
	Motif            string   `json:"motif,omitempty"`
	Palette          []string `json:"palette,omitempty"`
	Composition      string   `json:"composition,omitempty"`
	Typography       string   `json:"typography,omitempty"`
	Density          string   `json:"density,omitempty"`
	ProductFit       string   `json:"product_fit,omitempty"`
	Mood             string   `json:"mood,omitempty"`
	GarmentPlacement string   `json:"garment_placement,omitempty"`
	Avoid            []string `json:"avoid,omitempty"`
	Raw              string   `json:"-"`
}

type studioAbstractedReferenceAnalysis struct {
	Motif            string
	Palette          []string
	Composition      []string
	Typography       string
	Density          string
	ProductFit       string
	Mood             string
	GarmentPlacement string
	HadUnsafe        bool
	HadMalformed     bool
}

func (s *service) AnalyzeStudioReferenceStyle(ctx context.Context, req *StudioReferenceAnalysisRequest) (*StudioReferenceAnalysisResponse, error) {
	return s.taskStudioMediaOrDefault().AnalyzeStudioReferenceStyle(ctx, req)
}

func (s *taskStudioMediaService) AnalyzeStudioReferenceStyle(ctx context.Context, req *StudioReferenceAnalysisRequest) (*StudioReferenceAnalysisResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("invalid request: request is required")
	}
	urls, err := normalizeStudioReferenceImageURLs(req.ReferenceImageURLs)
	if err != nil {
		return nil, err
	}
	if len(urls) == 0 {
		return nil, fmt.Errorf("invalid request: reference_image_urls is required")
	}
	if s == nil || s.promptDiversifier == nil {
		return nil, fmt.Errorf("reference_analysis_unavailable: studio reference analyzer is not configured")
	}

	if len(urls) > maxStudioReferenceAnalysisImages {
		return nil, fmt.Errorf("invalid request: reference_image_urls supports at most 1 image")
	}
	warnings := make([]string, 0)
	resolvedURLs, err := s.resolveStudioReferenceImageURLs(ctx, urls)
	if err != nil {
		return nil, err
	}

	analysisPrompt := buildStudioReferenceAnalysisPrompt(req)
	analyses := make([]studioReferenceImageAnalysis, 0, len(urls))
	for _, imageURL := range resolvedURLs {
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
	if !hasStudioReusableSafeStyleDirection(abstracted) {
		return nil, fmt.Errorf("reference_analysis_failed: no reusable safe style direction extracted")
	}
	brief := buildStudioReferenceStyleBrief(abstracted)
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

func normalizeStudioReferenceImageURLs(urls []string) ([]string, error) {
	seen := map[string]struct{}{}
	result := make([]string, 0, len(urls))
	for _, raw := range urls {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			continue
		}
		if key, ok := studioReferenceUploadedImageKeyFromURL(trimmed); ok {
			dedupeKey := "upload:" + key
			if _, ok := seen[dedupeKey]; ok {
				continue
			}
			seen[dedupeKey] = struct{}{}
			result = append(result, trimmed)
			continue
		}
		parsed, err := url.ParseRequestURI(trimmed)
		if err != nil || parsed == nil || !parsed.IsAbs() || !strings.EqualFold(parsed.Scheme, "https") || strings.TrimSpace(parsed.Host) == "" {
			return nil, fmt.Errorf("invalid request: reference_image_urls must be absolute https urls or uploaded listingkit paths")
		}
		normalized := parsed.String()
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		result = append(result, normalized)
	}
	return result, nil
}

func (s *taskStudioMediaService) resolveStudioReferenceImageURLs(ctx context.Context, urls []string) ([]string, error) {
	resolved := make([]string, 0, len(urls))
	for _, rawURL := range urls {
		publicURL, err := s.resolveStudioReferenceImageURL(ctx, rawURL)
		if err != nil {
			return nil, err
		}
		resolved = append(resolved, publicURL)
	}
	return resolved, nil
}

func (s *taskStudioMediaService) resolveStudioReferenceImageURL(ctx context.Context, rawURL string) (string, error) {
	trimmed := strings.TrimSpace(rawURL)
	if key, ok := studioReferenceUploadedImageKeyFromURL(trimmed); ok {
		if s == nil || s.resolveUploadedImagePublicURL == nil {
			return "", fmt.Errorf("invalid request: uploaded reference image %q does not have a public https url configured", trimmed)
		}
		publicURL, err := s.resolveUploadedImagePublicURL(ctx, key)
		if err != nil {
			if errors.Is(err, ErrUploadedImageNotFound) {
				return "", fmt.Errorf("invalid request: uploaded reference image %q does not have a public https url configured", trimmed)
			}
			return "", fmt.Errorf("invalid request: resolve uploaded reference image %q: %w", trimmed, err)
		}
		publicURL, err = validateStudioReferencePublicHTTPSURL(publicURL)
		if err != nil {
			return "", fmt.Errorf("invalid request: uploaded reference image %q does not have a public https url configured", trimmed)
		}
		return publicURL, nil
	}
	publicURL, err := validateStudioReferencePublicHTTPSURL(trimmed)
	if err != nil {
		return "", fmt.Errorf("invalid request: reference_image_urls must be absolute https urls or uploaded listingkit paths")
	}
	return publicURL, nil
}

func studioReferenceUploadedImageKeyFromURL(rawURL string) (string, bool) {
	trimmed := strings.TrimSpace(rawURL)
	if trimmed == "" {
		return "", false
	}
	const prefix = "/api/v1/listing-kits/uploads/files/"
	if strings.HasPrefix(trimmed, prefix) {
		key := strings.TrimSpace(strings.TrimPrefix(trimmed, prefix))
		return key, key != ""
	}
	parsed, err := url.ParseRequestURI(trimmed)
	if err != nil || parsed == nil || !parsed.IsAbs() {
		return "", false
	}
	if !isStudioReferenceLocalHost(parsed.Hostname()) {
		return "", false
	}
	if !strings.HasPrefix(parsed.Path, prefix) {
		return "", false
	}
	key := strings.TrimSpace(strings.TrimPrefix(parsed.Path, prefix))
	return key, key != ""
}

func validateStudioReferencePublicHTTPSURL(rawURL string) (string, error) {
	trimmed := strings.TrimSpace(rawURL)
	if trimmed == "" {
		return "", fmt.Errorf("public https url is required")
	}
	parsed, err := url.ParseRequestURI(trimmed)
	if err != nil || parsed == nil || !parsed.IsAbs() || !strings.EqualFold(parsed.Scheme, "https") || strings.TrimSpace(parsed.Host) == "" {
		return "", fmt.Errorf("public https url is required")
	}
	if isStudioReferenceLocalHost(parsed.Hostname()) {
		return "", fmt.Errorf("public https url is required")
	}
	return parsed.String(), nil
}

func isStudioReferenceLocalHost(host string) bool {
	switch strings.ToLower(strings.TrimSpace(host)) {
	case "localhost", "127.0.0.1", "::1":
		return true
	default:
		return false
	}
}

func buildStudioReferenceAnalysisPrompt(req *StudioReferenceAnalysisRequest) string {
	product := strings.TrimSpace(req.ProductName)
	category := strings.Join(req.CategoryPath, " > ")
	basePrompt := strings.TrimSpace(req.BasePrompt)
	userInstruction := strings.TrimSpace(req.UserInstruction)
	return strings.TrimSpace(fmt.Sprintf(`Analyze this ecommerce product image as a style reference for original POD artwork.
Return JSON only with keys: motif, palette, composition, typography, density, product_fit, mood, garment_placement, avoid.
Extract broad reusable commercial style only. Do not ask to copy logos, brand marks, watermarks, exact slogans, exact artwork, exact characters, faces, or the same unique layout.
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

func buildStudioReferenceStyleBrief(analyses []studioAbstractedReferenceAnalysis) string {
	parts := []string{"Reference style cues."}
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
		if item.Mood != "" {
			parts = append(parts, "Mood cue: "+item.Mood+".")
		}
		if item.GarmentPlacement != "" {
			parts = append(parts, "Garment placement: "+item.GarmentPlacement+".")
		}
	}
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
	if mood := collectStudioReferenceFragments(analyses, func(item studioAbstractedReferenceAnalysis) string {
		return item.Mood
	}); len(mood) > 0 {
		parts = append(parts, "Mood cue: "+strings.Join(mood, ", ")+".")
	}
	if garmentPlacement := collectStudioReferenceFragments(analyses, func(item studioAbstractedReferenceAnalysis) string {
		return item.GarmentPlacement
	}); len(garmentPlacement) > 0 {
		parts = append(parts, "Garment placement: "+strings.Join(garmentPlacement, ", ")+".")
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
			abstracted.Mood == "" &&
			abstracted.GarmentPlacement == "" &&
			!(abstracted.HadUnsafe || abstracted.HadMalformed) {
			continue
		}
		result = append(result, abstracted)
	}
	return result
}

func hasStudioReusableSafeStyleDirection(analyses []studioAbstractedReferenceAnalysis) bool {
	for _, item := range analyses {
		if studioAbstractedReferenceAnalysisHasReusableSignals(item) {
			return true
		}
	}
	return false
}

func studioAbstractedReferenceAnalysisHasReusableSignals(item studioAbstractedReferenceAnalysis) bool {
	return item.Motif != "" ||
		len(item.Palette) > 0 ||
		len(item.Composition) > 0 ||
		item.Typography != "" ||
		item.Density != "" ||
		item.ProductFit != "" ||
		item.Mood != "" ||
		item.GarmentPlacement != ""
}

func abstractStudioReferenceAnalysis(item studioReferenceImageAnalysis) studioAbstractedReferenceAnalysis {
	abstracted := studioAbstractedReferenceAnalysis{
		Motif:            abstractStudioMotif(item.Motif),
		Palette:          abstractStudioPalette(item.Palette),
		Composition:      abstractStudioComposition(item.Composition),
		Typography:       abstractStudioTypography(item.Typography),
		Density:          abstractStudioDensity(item.Density),
		ProductFit:       abstractStudioProductFit(item.ProductFit),
		Mood:             abstractStudioMood(item.Mood),
		GarmentPlacement: abstractStudioGarmentPlacement(item.GarmentPlacement),
	}
	abstracted.HadMalformed = isMalformedStudioReferenceAnalysis(item)

	if abstracted.HadMalformed {
		rawUnsafe := studioReferenceFieldContainsUnsafeSignals(item.Raw)
		if !rawUnsafe && abstracted.Motif == "" {
			abstracted.Motif = abstractStudioMotif(item.Raw)
		}
		if !rawUnsafe && len(abstracted.Palette) == 0 {
			abstracted.Palette = abstractStudioPalette([]string{item.Raw})
		}
		if len(abstracted.Composition) == 0 {
			abstracted.Composition = abstractStudioComposition(item.Raw)
		}
		if !rawUnsafe && abstracted.Typography == "" {
			abstracted.Typography = recoverMalformedStudioReferenceField(item.Raw, studioTypographyPhraseVocabulary, studioTypographyWordVocabulary, abstractStudioTypography)
		}
		if !rawUnsafe && abstracted.Density == "" {
			abstracted.Density = recoverMalformedStudioReferenceField(item.Raw, studioDensityPhraseVocabulary, studioDensityWordVocabulary, abstractStudioDensity)
		}
		if !rawUnsafe && abstracted.ProductFit == "" {
			abstracted.ProductFit = recoverMalformedStudioReferenceField(item.Raw, studioProductFitPhraseVocabulary, studioProductFitWordVocabulary, abstractStudioProductFit)
		}
		if !rawUnsafe && abstracted.Mood == "" {
			abstracted.Mood = abstractStudioMood(item.Raw)
		}
		if !rawUnsafe && abstracted.GarmentPlacement == "" {
			abstracted.GarmentPlacement = abstractStudioGarmentPlacement(item.Raw)
		}
		if !rawUnsafe && !studioAbstractedReferenceAnalysisHasReusableSignals(abstracted) {
			abstracted = mergeStudioAbstractedReferenceAnalysis(abstracted, deriveConservativeStudioMalformedFallback(item.Raw))
		}
	}

	applyStructuredStudioFallbacks(item, &abstracted)
	abstracted.HadUnsafe = studioReferenceAnalysisContainsUnsafeSignals(item)
	for _, avoid := range item.Avoid {
		if studioReferenceAvoidContainsUnsafeSignals(avoid) {
			abstracted.HadUnsafe = true
		}
	}
	abstracted.HadUnsafe = abstracted.HadUnsafe || studioStructuredFieldDroppedDueToUnsafeSignal(item.Motif, abstracted.Motif)
	abstracted.HadUnsafe = abstracted.HadUnsafe || studioStructuredFieldDroppedDueToUnsafeSignal(item.Typography, abstracted.Typography)
	abstracted.HadUnsafe = abstracted.HadUnsafe || studioStructuredFieldDroppedDueToUnsafeSignal(item.Density, abstracted.Density)
	abstracted.HadUnsafe = abstracted.HadUnsafe || studioStructuredFieldDroppedDueToUnsafeSignal(item.ProductFit, abstracted.ProductFit)
	abstracted.HadUnsafe = abstracted.HadUnsafe || studioStructuredFieldDroppedDueToUnsafeSignal(item.Mood, abstracted.Mood)
	abstracted.HadUnsafe = abstracted.HadUnsafe || studioStructuredFieldDroppedDueToUnsafeSignal(item.GarmentPlacement, abstracted.GarmentPlacement)
	return abstracted
}

func mergeStudioAbstractedReferenceAnalysis(primary studioAbstractedReferenceAnalysis, fallback studioAbstractedReferenceAnalysis) studioAbstractedReferenceAnalysis {
	if primary.Motif == "" {
		primary.Motif = fallback.Motif
	}
	if len(primary.Palette) == 0 {
		primary.Palette = fallback.Palette
	}
	if len(primary.Composition) == 0 {
		primary.Composition = fallback.Composition
	}
	if primary.Typography == "" {
		primary.Typography = fallback.Typography
	}
	if primary.Density == "" {
		primary.Density = fallback.Density
	}
	if primary.ProductFit == "" {
		primary.ProductFit = fallback.ProductFit
	}
	if primary.Mood == "" {
		primary.Mood = fallback.Mood
	}
	if primary.GarmentPlacement == "" {
		primary.GarmentPlacement = fallback.GarmentPlacement
	}
	return primary
}

func deriveConservativeStudioMalformedFallback(raw string) studioAbstractedReferenceAnalysis {
	if sanitizeStudioMalformedFallbackCandidate(raw) == "" {
		return studioAbstractedReferenceAnalysis{}
	}
	return studioAbstractedReferenceAnalysis{
		Motif:       "abstract",
		Composition: []string{"balanced composition"},
	}
}

func applyStructuredStudioFallbacks(item studioReferenceImageAnalysis, abstracted *studioAbstractedReferenceAnalysis) {
	if abstracted == nil {
		return
	}
	if abstracted.Motif == "" && studioStructuredFieldCanUseFallback(item.Motif) {
		abstracted.Motif = "abstract motif direction"
	}
	if len(abstracted.Palette) == 0 && studioStructuredPaletteCanUseFallback(item.Palette) {
		abstracted.Palette = []string{"balanced palette direction"}
	}
	if abstracted.Typography == "" && studioStructuredFieldCanUseFallback(item.Typography) {
		abstracted.Typography = "decorative typography direction"
	}
	if abstracted.Density == "" && studioStructuredFieldCanUseFallback(item.Density) {
		abstracted.Density = "balanced visual density"
	}
	if abstracted.ProductFit == "" && studioStructuredFieldCanUseFallback(item.ProductFit) {
		abstracted.ProductFit = "general apparel styling"
	}
	if abstracted.Mood == "" && studioStructuredFieldCanUseFallback(item.Mood) {
		abstracted.Mood = "balanced mood"
	}
	if abstracted.GarmentPlacement == "" && studioStructuredFieldCanUseFallback(item.GarmentPlacement) {
		abstracted.GarmentPlacement = "standard garment placement"
	}
}

func studioStructuredFieldCanUseFallback(value string) bool {
	trimmed := strings.TrimSpace(value)
	return trimmed != "" && !studioReferenceFieldContainsUnsafeSignals(trimmed)
}

func studioStructuredPaletteCanUseFallback(values []string) bool {
	if len(values) == 0 {
		return false
	}
	hasNonEmpty := false
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		hasNonEmpty = true
		if studioReferenceFieldContainsUnsafeSignals(trimmed) {
			return false
		}
	}
	return hasNonEmpty
}

func abstractStudioMotif(value string) string {
	return abstractStudioFieldDescriptor(value, studioMotifPhraseVocabulary, studioMotifWordVocabulary)
}

func abstractStudioPalette(values []string) []string {
	result := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range values {
		abstracted := abstractStudioFieldDescriptor(value, studioPalettePhraseVocabulary, studioPaletteWordVocabulary)
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
	return abstractStudioFieldDescriptor(value, studioTypographyPhraseVocabulary, studioTypographyWordVocabulary)
}

func abstractStudioDensity(value string) string {
	return abstractStudioFieldDescriptor(value, studioDensityPhraseVocabulary, studioDensityWordVocabulary)
}

func abstractStudioProductFit(value string) string {
	return abstractStudioFieldDescriptor(value, studioProductFitPhraseVocabulary, studioProductFitWordVocabulary)
}

func abstractStudioMood(value string) string {
	lower := strings.ToLower(sanitizeStudioDescriptorCandidate(value))
	switch {
	case strings.Contains(lower, "playful"):
		return "playful mood"
	case strings.Contains(lower, "airy"):
		return "airy mood"
	case strings.Contains(lower, "nostalgic"), strings.Contains(lower, "retro"), strings.Contains(lower, "vintage"):
		return "nostalgic mood"
	case strings.Contains(lower, "coastal"), strings.Contains(lower, "resort"), strings.Contains(lower, "beach"):
		return "relaxed mood"
	default:
		return ""
	}
}

func abstractStudioGarmentPlacement(value string) string {
	lower := strings.ToLower(sanitizeStudioDescriptorCandidate(value))
	switch {
	case strings.Contains(lower, "left") && strings.Contains(lower, "chest"):
		return "left chest placement"
	case strings.Contains(lower, "right") && strings.Contains(lower, "chest"):
		return "right chest placement"
	case strings.Contains(lower, "front") && strings.Contains(lower, "chest"):
		return "front chest placement"
	case strings.Contains(lower, "large") && (strings.Contains(lower, "center") || strings.Contains(lower, "front")):
		return "large center placement"
	case strings.Contains(lower, "center"):
		return "center placement"
	case strings.Contains(lower, "back"):
		return "back placement"
	case strings.Contains(lower, "sleeve"):
		return "sleeve placement"
	default:
		return ""
	}
}

func abstractStudioComposition(value string) []string {
	lower := strings.ToLower(strings.TrimSpace(value))
	if lower == "" {
		return nil
	}
	result := make([]string, 0, 3)
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

func studioCompositionShouldSuppressLiteral(lower string) bool {
	for _, token := range []string{"same", "exact", "identical", "signature", "unique", "distinctive", "split", "diagonal", "collage", "offset"} {
		if strings.Contains(lower, token) {
			return true
		}
	}
	return false
}

func sanitizeStudioStylePhrase(value string) string {
	return sanitizeStudioSafeDescriptorPhrase(value)
}

func abstractStudioFieldDescriptor(value string, phrases []string, words map[string]string) string {
	vocabularyDerived := abstractStudioVocabularyPhrase(value, phrases, words)
	genericDerived := sanitizeStudioSafeDescriptorPhrase(value)
	return preferRicherStudioDescriptor(vocabularyDerived, genericDerived)
}

func sanitizeStudioSafeDescriptorPhrase(value string) string {
	cleaned := sanitizeStudioDescriptorCandidate(value)
	if cleaned == "" {
		return ""
	}
	tokens := studioWordPattern.FindAllString(cleaned, -1)
	if len(tokens) == 0 {
		return ""
	}
	filtered := make([]string, 0, len(tokens))
	for _, token := range tokens {
		if _, ok := studioSafeDescriptorWords[token]; ok {
			filtered = append(filtered, token)
		}
	}
	return strings.TrimSpace(studioRepeatedWhitespacePattern.ReplaceAllString(strings.Join(filtered, " "), " "))
}

func sanitizeStudioDescriptorCandidate(value string) string {
	original := strings.TrimSpace(value)
	if original == "" {
		return ""
	}
	cleaned, _ := stripStudioProtectedIdentityPhrases(original)
	cleaned, removedSuspiciousName := stripSuspiciousStudioNamedPhrases(cleaned)
	lower := strings.ToLower(cleaned)
	lower = studioUnsafeSpacerPattern.ReplaceAllString(lower, " ")
	lower = studioProtectedIdentityPattern.ReplaceAllString(lower, " ")
	lower = studioQuotedTextPattern.ReplaceAllString(lower, " ")
	lower = studioUniqueLayoutPattern.ReplaceAllString(lower, " ")
	lower = studioCharacterIdentityPattern.ReplaceAllString(lower, " ")
	lower = studioExactArtworkPattern.ReplaceAllString(lower, " ")
	lower = studioExactTextPattern.ReplaceAllString(lower, " ")
	lower = studioBrandMarkPattern.ReplaceAllString(lower, " ")
	lower = studioWatermarkPattern.ReplaceAllString(lower, " ")
	lower = strings.NewReplacer("/", " ", "&", " ").Replace(lower)
	tokens := studioWordPattern.FindAllString(lower, -1)
	if len(tokens) == 0 {
		return ""
	}
	filtered := make([]string, 0, len(tokens))
	for _, token := range tokens {
		if token == "same" || token == "exact" || token == "identical" || token == "signature" || token == "unique" || token == "distinctive" {
			continue
		}
		filtered = append(filtered, token)
	}
	if removedSuspiciousName && len(filtered) <= 1 {
		return ""
	}
	return strings.TrimSpace(studioRepeatedWhitespacePattern.ReplaceAllString(strings.Join(filtered, " "), " "))
}

func sanitizeStudioMalformedFallbackCandidate(value string) string {
	original := strings.TrimSpace(value)
	if original == "" {
		return ""
	}
	cleaned, _ := stripStudioProtectedIdentityPhrases(original)
	cleaned, _ = stripSuspiciousStudioNamedPhrases(cleaned)
	lower := strings.ToLower(cleaned)
	lower = studioUnsafeSpacerPattern.ReplaceAllString(lower, " ")
	lower = studioProtectedIdentityPattern.ReplaceAllString(lower, " ")
	lower = studioQuotedTextPattern.ReplaceAllString(lower, " ")
	lower = studioUniqueLayoutPattern.ReplaceAllString(lower, " ")
	lower = studioCharacterIdentityPattern.ReplaceAllString(lower, " ")
	lower = studioExactArtworkPattern.ReplaceAllString(lower, " ")
	lower = studioExactTextPattern.ReplaceAllString(lower, " ")
	lower = studioBrandMarkPattern.ReplaceAllString(lower, " ")
	lower = studioWatermarkPattern.ReplaceAllString(lower, " ")
	lower = strings.NewReplacer("/", " ", "&", " ").Replace(lower)
	tokens := studioWordPattern.FindAllString(lower, -1)
	if len(tokens) == 0 {
		return ""
	}
	return strings.TrimSpace(studioRepeatedWhitespacePattern.ReplaceAllString(strings.Join(tokens, " "), " "))
}

func stripStudioProtectedIdentityPhrases(value string) (string, bool) {
	result := value
	removed := false
	for _, term := range studioProtectedIdentityTerms {
		pattern := regexp.MustCompile(`(?i)\b` + strings.ReplaceAll(regexp.QuoteMeta(term), `\ `, `\s+`) + `\b`)
		updated := pattern.ReplaceAllString(result, " ")
		if updated != result {
			removed = true
			result = updated
		}
	}
	return result, removed
}

func stripSuspiciousStudioNamedPhrases(value string) (string, bool) {
	removed := false
	result := studioCapitalizedPhrasePattern.ReplaceAllStringFunc(value, func(match string) string {
		if studioCapitalizedPhraseIsSafe(match) {
			return match
		}
		removed = true
		return " "
	})
	return result, removed
}

func studioCapitalizedPhraseIsSafe(value string) bool {
	normalized := strings.ToLower(strings.TrimSpace(value))
	tokens := studioWordPattern.FindAllString(normalized, -1)
	if len(tokens) == 0 {
		return true
	}
	if len(tokens) > 1 {
		_, ok := studioSafeTitleCasePhraseSet[normalized]
		return ok
	}
	for _, token := range tokens {
		if _, ok := studioSafeDescriptorWords[token]; !ok {
			return false
		}
	}
	return true
}

func buildStudioSafeTitleCasePhraseSet(vocabularies ...[]string) map[string]struct{} {
	result := make(map[string]struct{})
	for _, vocabulary := range vocabularies {
		for _, phrase := range vocabulary {
			normalized := strings.ToLower(strings.TrimSpace(phrase))
			if normalized == "" {
				continue
			}
			result[normalized] = struct{}{}
		}
	}
	return result
}

func preferRicherStudioDescriptor(primary string, fallback string) string {
	primary = strings.TrimSpace(primary)
	fallback = strings.TrimSpace(fallback)
	switch {
	case primary == "":
		return fallback
	case fallback == "":
		return primary
	case primary == fallback:
		return primary
	case studioDescriptorWordCount(primary) > studioDescriptorWordCount(fallback):
		return primary
	default:
		return fallback
	}
}

func studioDescriptorWordCount(value string) int {
	return len(studioWordPattern.FindAllString(strings.ToLower(strings.TrimSpace(value)), -1))
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

func recoverMalformedStudioReferenceField(raw string, phrases []string, words map[string]string, abstract func(string) string) string {
	for _, fragment := range splitMalformedStudioReferenceFragments(raw) {
		if !studioFragmentMatchesFieldVocabulary(fragment, phrases, words) {
			continue
		}
		if value := abstract(fragment); strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func splitMalformedStudioReferenceFragments(raw string) []string {
	parts := strings.FieldsFunc(raw, func(r rune) bool {
		switch r {
		case ',', ';', '\n', '\r':
			return true
		default:
			return false
		}
	})
	fragments := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		fragments = append(fragments, trimmed)
	}
	return fragments
}

func studioFragmentMatchesFieldVocabulary(fragment string, phrases []string, words map[string]string) bool {
	lower := strings.ToLower(strings.TrimSpace(fragment))
	if lower == "" {
		return false
	}
	for _, phrase := range phrases {
		if strings.Contains(lower, phrase) {
			return true
		}
	}
	for _, token := range studioWordPattern.FindAllString(lower, -1) {
		if _, ok := words[token]; ok {
			return true
		}
	}
	return false
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
		if studioReferenceAnalysisContainsUnsafeSignals(item) {
			return true
		}
		for _, avoid := range item.Avoid {
			if studioReferenceAvoidContainsUnsafeSignals(avoid) {
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

func studioReferenceAnalysisContainsUnsafeSignals(item studioReferenceImageAnalysis) bool {
	if isMalformedStudioReferenceAnalysis(item) {
		return studioReferenceFieldContainsUnsafeSignals(item.Raw)
	}

	for _, value := range []string{
		item.Motif,
		item.Composition,
		item.Typography,
		item.Density,
		item.ProductFit,
		item.Mood,
		item.GarmentPlacement,
	} {
		if studioReferenceFieldContainsUnsafeSignals(value) {
			return true
		}
	}
	for _, value := range item.Palette {
		if studioReferenceFieldContainsUnsafeSignals(value) {
			return true
		}
	}
	for _, value := range item.Avoid {
		if studioReferenceAvoidContainsUnsafeSignals(value) {
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
	return studioProtectedIdentityPattern.MatchString(value) ||
		studioQuotedTextPattern.MatchString(value) ||
		studioBrandMarkPattern.MatchString(lower) ||
		studioWatermarkPattern.MatchString(lower) ||
		studioExactArtworkPattern.MatchString(lower) ||
		studioExactTextPattern.MatchString(lower) ||
		studioCharacterIdentityPattern.MatchString(lower) ||
		studioUniqueLayoutPattern.MatchString(lower)
}

func studioReferenceAvoidContainsUnsafeSignals(value string) bool {
	return studioReferenceFieldContainsUnsafeSignals(value)
}

func studioStructuredFieldDroppedDueToUnsafeSignal(original string, abstracted string) bool {
	trimmedOriginal := strings.TrimSpace(original)
	if trimmedOriginal == "" || strings.TrimSpace(abstracted) != "" {
		return false
	}
	return studioReferenceFieldContainsUnsafeSignals(trimmedOriginal) || studioDescriptorContainsSuspiciousNamedPhrase(trimmedOriginal)
}

func studioDescriptorContainsSuspiciousNamedPhrase(value string) bool {
	if strings.TrimSpace(value) == "" {
		return false
	}
	_, removed := stripSuspiciousStudioNamedPhrases(value)
	return removed
}

func isMalformedStudioReferenceAnalysis(item studioReferenceImageAnalysis) bool {
	return item.Raw != "" &&
		item.Motif == "" &&
		len(item.Palette) == 0 &&
		item.Composition == "" &&
		item.Typography == "" &&
		item.Density == "" &&
		item.ProductFit == "" &&
		item.Mood == "" &&
		item.GarmentPlacement == "" &&
		len(item.Avoid) == 0
}
