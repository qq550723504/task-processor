package listingkit

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

const maxStudioReferenceAnalysisImages = 5

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

	analyses := make([]studioReferenceImageAnalysis, 0, len(urls))
	for _, imageURL := range urls {
		raw, err := s.promptDiversifier.AnalyzeImage(ctx, imageURL, buildStudioReferenceAnalysisPrompt(req))
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
	sanitized := sanitizeStudioReferencePrompt(brief)
	if strings.TrimSpace(sanitized) == "" {
		return nil, fmt.Errorf("reference_analysis_failed: generated reference brief is empty")
	}
	if sanitized != brief {
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

func sanitizeStudioReferencePrompt(value string) string {
	replacements := []string{
		"Nike", "", "nike", "", "logo", "brand-neutral mark", "Logo", "brand-neutral mark",
		"exact slogan", "new short generic phrase", "same wording", "new wording",
		"copy this exact", "reinterpret as original", "same character", "original motif",
	}
	sanitized := strings.NewReplacer(replacements...).Replace(value)
	sanitized = strings.Join(strings.Fields(sanitized), " ")
	return strings.TrimSpace(sanitized)
}
