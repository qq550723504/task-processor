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
	studioQuotedTextPattern          = regexp.MustCompile(`["'][^"']+["']`)
	studioProperNamePattern          = regexp.MustCompile(`\b[A-Z][a-z]+(?:\s+[A-Z][a-z]+){0,2}\b`)
	studioBrandMarkPattern           = regexp.MustCompile(`\b(?:[a-z0-9]+(?:\s+[a-z0-9]+){0,3}\s+)?(?:logo|logos|brand mark|brand marks|wordmark|wordmarks|emblem|emblems|trefoil|swoosh)\b`)
	studioExactTextPattern           = regexp.MustCompile(`\b(?:exact text|exact slogan|same wording|copy this exact|quote|quoted|slogan|tagline|catchphrase)\b`)
	studioCharacterIdentityPattern   = regexp.MustCompile(`\b(?:characters?|face|faces|portrait|portraits|person|people|identity|identities|celebrity|celebrities|likeness)\b`)
	studioUniqueLayoutPattern        = regexp.MustCompile(`\b(?:same|exact|identical|signature|unique|distinctive)\s+[a-z0-9\s-]{0,40}?(?:layout|composition|arrangement|split|frame|badge)\b`)
	studioUnsafeResidualTokenPattern = regexp.MustCompile(`\b(?:adidas|nike|disney|marvel|pokemon|mickey|mouse|elsa|taylor|swift)\b`)
	studioNonAlphanumericPattern     = regexp.MustCompile(`[^a-z0-9\s,-]+`)
	studioRepeatedWhitespacePattern  = regexp.MustCompile(`\s+`)
	studioUnsafeSignalPattern        = regexp.MustCompile(`(?i)(logo|brand mark|wordmark|emblem|trefoil|swoosh|quote|slogan|tagline|catchphrase|character|face|portrait|person|identity|celebrity|likeness|same .*layout|same .*composition|exact .*layout|exact .*composition|\"[^\"]+\"|'[^']+')`)
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
		analysis := parseStudioReferenceImageAnalysis(raw)
		analyses = append(analyses, analysis)
	}
	if len(analyses) == 0 {
		return nil, fmt.Errorf("reference_analysis_failed: no reference image could be analyzed")
	}

	brief := buildStudioReferenceStyleBrief(req, analyses)
	sanitized := buildSanitizedStudioReferencePrompt(analyses)
	if strings.TrimSpace(sanitized) == "" {
		return nil, fmt.Errorf("reference_analysis_failed: generated reference brief is empty")
	}
	if studioReferenceContainsUnsafeSignals(analyses) {
		warnings = append(warnings, "已移除品牌、Logo、原文案或过于接近原图的描述。")
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

func buildStudioReferenceStyleBrief(req *StudioReferenceAnalysisRequest, analyses []studioReferenceImageAnalysis) string {
	parts := []string{"Hot-selling reference style direction for original POD artwork."}
	if product := strings.TrimSpace(req.ProductName); product != "" {
		parts = append(parts, "Base product: "+product+".")
	}
	if category := strings.TrimSpace(strings.Join(req.CategoryPath, " > ")); category != "" {
		parts = append(parts, "Category: "+category+".")
	}
	for _, item := range analyses {
		if item.Raw != "" && item.Motif == "" && item.Composition == "" {
			parts = append(parts, "Reference notes: "+item.Raw+".")
			continue
		}
		if item.Motif != "" {
			parts = append(parts, "Motif family: "+item.Motif+".")
		}
		if len(item.Palette) > 0 {
			parts = append(parts, "Palette direction: "+strings.Join(item.Palette, ", ")+".")
		}
		if item.Composition != "" {
			parts = append(parts, "Composition family: "+item.Composition+".")
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
		if len(item.Avoid) > 0 {
			parts = append(parts, "Avoid from references: "+strings.Join(item.Avoid, ", ")+".")
		}
	}
	parts = append(parts, "Create a new original design in this broad style family. Do not reproduce logos, brand marks, exact text, characters, faces, or the same unique layout from any reference image.")
	return strings.Join(parts, " ")
}

func buildSanitizedStudioReferencePrompt(analyses []studioReferenceImageAnalysis) string {
	parts := []string{"Create an original POD artwork with a commercially proven graphic style direction."}

	if motifs := collectStudioReferenceFragments(analyses, func(item studioReferenceImageAnalysis) string {
		return sanitizeStudioReferencePrompt(item.Motif)
	}); len(motifs) > 0 {
		parts = append(parts, "Motif direction: "+strings.Join(motifs, ", ")+".")
	}
	if palettes := collectStudioReferencePalettes(analyses); len(palettes) > 0 {
		parts = append(parts, "Palette direction: "+strings.Join(palettes, ", ")+".")
	}
	if compositions := collectStudioReferenceFragments(analyses, func(item studioReferenceImageAnalysis) string {
		return sanitizeStudioReferencePrompt(item.Composition)
	}); len(compositions) > 0 {
		parts = append(parts, "Composition direction: "+strings.Join(compositions, ", ")+".")
	}
	if typography := collectStudioReferenceFragments(analyses, func(item studioReferenceImageAnalysis) string {
		return sanitizeStudioReferencePrompt(item.Typography)
	}); len(typography) > 0 {
		parts = append(parts, "Typography feel: "+strings.Join(typography, ", ")+".")
	}
	if density := collectStudioReferenceFragments(analyses, func(item studioReferenceImageAnalysis) string {
		return sanitizeStudioReferencePrompt(item.Density)
	}); len(density) > 0 {
		parts = append(parts, "Visual density: "+strings.Join(density, ", ")+".")
	}
	if productFit := collectStudioReferenceFragments(analyses, func(item studioReferenceImageAnalysis) string {
		return sanitizeStudioReferencePrompt(item.ProductFit)
	}); len(productFit) > 0 {
		parts = append(parts, "Product fit: "+strings.Join(productFit, ", ")+".")
	}

	parts = append(parts, "Keep all graphics brand-neutral, use fresh custom wording if text appears, avoid recognizable characters or people, and use a clearly original layout.")
	return strings.Join(parts, " ")
}

func collectStudioReferenceFragments(analyses []studioReferenceImageAnalysis, pick func(studioReferenceImageAnalysis) string) []string {
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

func collectStudioReferencePalettes(analyses []studioReferenceImageAnalysis) []string {
	result := make([]string, 0)
	seen := map[string]struct{}{}
	for _, item := range analyses {
		colors := make([]string, 0, len(item.Palette))
		for _, color := range item.Palette {
			sanitized := sanitizeStudioReferencePrompt(color)
			if sanitized == "" {
				continue
			}
			colors = append(colors, sanitized)
		}
		if len(colors) == 0 {
			continue
		}
		palette := strings.Join(colors, ", ")
		if _, ok := seen[palette]; ok {
			continue
		}
		seen[palette] = struct{}{}
		result = append(result, palette)
	}
	return result
}

func sanitizeStudioReferencePrompt(value string) string {
	sanitized := strings.TrimSpace(value)
	if sanitized == "" {
		return ""
	}
	sanitized = studioQuotedTextPattern.ReplaceAllString(sanitized, " custom wording ")
	sanitized = studioProperNamePattern.ReplaceAllString(sanitized, " ")
	sanitized = strings.ToLower(sanitized)
	sanitized = studioUniqueLayoutPattern.ReplaceAllString(sanitized, " balanced original composition ")
	sanitized = studioBrandMarkPattern.ReplaceAllString(sanitized, " brand-neutral graphic accents ")
	sanitized = studioExactTextPattern.ReplaceAllString(sanitized, " custom wording ")
	sanitized = studioCharacterIdentityPattern.ReplaceAllString(sanitized, " abstract illustrated details ")
	sanitized = studioUnsafeResidualTokenPattern.ReplaceAllString(sanitized, " ")
	sanitized = studioNonAlphanumericPattern.ReplaceAllString(sanitized, " ")
	sanitized = strings.Join(strings.Fields(strings.ReplaceAll(sanitized, ",", " ")), " ")
	switch sanitized {
	case "", "custom wording", "brand neutral graphic accents", "abstract illustrated details":
		return ""
	}
	return studioRepeatedWhitespacePattern.ReplaceAllString(strings.TrimSpace(sanitized), " ")
}

func studioReferenceContainsUnsafeSignals(analyses []studioReferenceImageAnalysis) bool {
	for _, item := range analyses {
		if studioUnsafeSignalPattern.MatchString(item.Raw) {
			return true
		}
		for _, avoid := range item.Avoid {
			if studioUnsafeSignalPattern.MatchString(avoid) {
				return true
			}
		}
	}
	return false
}
