package listingkit

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"path"
	"strings"

	referenceanalysis "task-processor/internal/listing/studio/referenceanalysis"
)

const maxStudioReferenceAnalysisImages = 1

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
	rawAnalyses := make([]string, 0, len(urls))
	for _, imageURL := range resolvedURLs {
		raw, err := s.promptDiversifier.AnalyzeImage(ctx, imageURL, analysisPrompt)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("参考图分析失败：%s", compactStudioGenerationError(err)))
			continue
		}
		rawAnalyses = append(rawAnalyses, raw)
	}
	if len(rawAnalyses) == 0 {
		return nil, fmt.Errorf("reference_analysis_failed: no reference image could be analyzed")
	}

	interpreted, err := referenceanalysis.Interpret(rawAnalyses)
	if err != nil {
		switch {
		case errors.Is(err, referenceanalysis.ErrNoSafeDirection):
			return nil, fmt.Errorf("reference_analysis_failed: no reusable safe style direction extracted")
		case errors.Is(err, referenceanalysis.ErrEmptyPrompt):
			return nil, fmt.Errorf("reference_analysis_failed: generated reference brief is empty")
		case errors.Is(err, referenceanalysis.ErrNoInput):
			return nil, fmt.Errorf("reference_analysis_failed: no reference image could be analyzed")
		default:
			return nil, fmt.Errorf("reference_analysis_failed: %w", err)
		}
	}
	if interpreted.HadUnsafeInput {
		warnings = append(warnings, "已移除品牌、Logo、原文案或过于接近原图的描述。")
	}
	if interpreted.HadMalformedInput {
		warnings = append(warnings, "部分参考图返回了非结构化分析结果，仅保留可安全复用的风格提示。")
	}
	return &StudioReferenceAnalysisResponse{
		ReferenceStyleBrief: interpreted.StyleBrief,
		SanitizedPrompt:     interpreted.SanitizedPrompt,
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

func studioReferenceUploadedImageKeyCandidates(rawURL string) []string {
	trimmed := strings.TrimSpace(rawURL)
	if trimmed == "" {
		return nil
	}
	seen := map[string]struct{}{}
	result := make([]string, 0, 2)
	appendKey := func(key string) {
		key = strings.TrimSpace(strings.TrimPrefix(key, "/"))
		if key == "" {
			return
		}
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		result = append(result, key)
	}
	if key, ok := studioReferenceUploadedImageKeyFromURL(trimmed); ok {
		appendKey(key)
	}
	parsed, err := url.ParseRequestURI(trimmed)
	if err != nil || parsed == nil || !parsed.IsAbs() {
		return result
	}
	cleanPath := strings.TrimPrefix(path.Clean(parsed.Path), "/")
	const assetPrefix = "listingkit-assets/"
	if strings.HasPrefix(cleanPath, assetPrefix) {
		appendKey(strings.TrimPrefix(cleanPath, assetPrefix))
	}
	appendKey(cleanPath)
	return result
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
