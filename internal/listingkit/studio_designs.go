package listingkit

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	openaiclient "task-processor/internal/infra/clients/openai"
	"task-processor/internal/pkg/jsonx"
	"task-processor/internal/prompt"
)

const maxStudioDesignCount = 10
const studioDesignTransparentModel = "gpt-image-2"
const (
	studioVariationLight  = "light"
	studioVariationMedium = "medium"
	studioVariationStrong = "strong"
)

func (s *service) GenerateStudioDesigns(ctx context.Context, req *StudioDesignRequest) (*StudioDesignResponse, error) {
	return s.taskStudioMediaOrDefault().GenerateStudioDesigns(ctx, req)
}

func (s *service) generateStudioDesignSiblingThemes(ctx context.Context, req *StudioDesignRequest, count int) ([]string, error) {
	return s.taskStudioMediaOrDefault().generateStudioDesignSiblingThemes(ctx, req, count)
}

func (s *service) generateStudioDesignImage(ctx context.Context, model string, promptText string, size string, referenceURLs []string) (*openaiclient.ImageResponse, error) {
	return s.taskStudioMediaOrDefault().generateStudioDesignImage(ctx, model, promptText, size, referenceURLs)
}

func (s *service) editStudioDesignImageWithReferences(ctx context.Context, model string, promptText string, size string, referenceURLs []string) (*openaiclient.ImageResponse, error) {
	return s.taskStudioMediaOrDefault().editStudioDesignImageWithReferences(ctx, model, promptText, size, referenceURLs)
}

func (s *service) generateStudioDesignImageWithoutReferences(ctx context.Context, model string, promptText string, size string) (*openaiclient.ImageResponse, error) {
	return s.taskStudioMediaOrDefault().generateStudioDesignImageWithoutReferences(ctx, model, promptText, size)
}

func (s *service) taskStudioMediaOrDefault() *taskStudioMediaService {
	if s.taskStudioMedia != nil {
		return s.taskStudioMedia
	}
	s.taskStudioMedia = newTaskStudioMediaService(taskStudioMediaServiceConfig{
		imageGenerator:        s.studioImageGenerator,
		promptDiversifier:     s.studioPromptDiversifier,
		uploadStoreConfigured: s.uploadStore != nil,
		uploadImages:          s.UploadImages,
	})
	return s.taskStudioMedia
}

func buildStudioDesignPrompt(req *StudioDesignRequest) string {
	return buildStudioDesignPromptWithTheme(req, strings.TrimSpace(req.Prompt))
}

func buildStudioDesignPromptWithTheme(req *StudioDesignRequest, theme string) string {
	printableHint := ""
	if req.PrintableWidth > 0 && req.PrintableHeight > 0 {
		printableHint = fmt.Sprintf("Target print area: %d by %d pixels.", req.PrintableWidth, req.PrintableHeight)
	}
	referenceHint := ""
	if len(studioDesignReferenceImageURLs(req.ProductReferenceImageURLs)) > 0 {
		referenceHint = "SDS product mockup/reference images are provided. Use them to understand product color variants, material, print-surface shape, scale, and visual contrast. Generate only the flat artwork/design, not a product photo; make the artwork work across the provided product colors."
	}
	transparentHint := ""
	if req.TransparentBackground {
		transparentHint = "Output the artwork on a true transparent background with alpha channel. Do not simulate transparency with checkerboard, paper texture, white fill, colored fill, or any background pattern."
	}
	vars := map[string]any{
		"PrintableHint":   printableHint,
		"ReferenceHint":   referenceHint,
		"TransparentHint": transparentHint,
		"ThemePrompt":     strings.TrimSpace(theme),
	}
	fallback := "Create a single print-ready graphic for ecommerce POD or customized-product use. Return a flat design only, not a product mockup, model photo, scene photo, or physical product rendering. {{.PrintableHint}} {{.ReferenceHint}} {{.TransparentHint}} Theme prompt: {{.ThemePrompt}}"
	if prompt.GlobalRegistry == nil {
		return renderPromptFallback(fallback, vars)
	}
	rendered, err := prompt.GlobalRegistry.Render(prompt.KProductImageStudioGenerationPodDesign, vars, fallback)
	if err != nil {
		return renderPromptFallback(fallback, vars)
	}
	return strings.TrimSpace(rendered)
}

func normalizeStudioVariationIntensity(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case studioVariationLight:
		return studioVariationLight
	case studioVariationStrong:
		return studioVariationStrong
	default:
		return studioVariationMedium
	}
}

func buildFallbackStudioDesignThemes(promptText string, count int) []string {
	if count <= 0 {
		return nil
	}
	theme := strings.TrimSpace(promptText)
	themes := make([]string, count)
	for idx := range themes {
		themes[idx] = theme
	}
	return themes
}

type studioSiblingPromptResponse struct {
	Prompts []string `json:"prompts,omitempty"`
}

func buildStudioDesignSiblingPromptRequest(req *StudioDesignRequest, count int) string {
	var builder strings.Builder
	builder.WriteString("You are preparing sibling prompts for POD flat artwork generation.\n")
	builder.WriteString("Return JSON only.\n")
	builder.WriteString(fmt.Sprintf("Return exactly %d prompts as {\"prompts\":[\"...\", ...]}.\n", count))
	builder.WriteString("Each prompt must preserve the same core selling point and the same visual style family as the source prompt.\n")
	builder.WriteString("Vary composition, motif arrangement, focal hierarchy, negative space, and supporting elements.\n")
	builder.WriteString("Do not write a sentence about how to design. Do not include product mockup wording, camera wording, copyright disclaimers, or pixel/canvas instructions.\n")
	builder.WriteString("Each prompt must be concise, production-oriented, and suitable as a direct theme prompt for flat POD artwork.\n")
	builder.WriteString(fmt.Sprintf("Variation intensity: %s.\n", normalizeStudioVariationIntensity(req.VariationIntensity)))
	builder.WriteString("Intensity guide:\n")
	builder.WriteString("- light: same-series small variations\n")
	builder.WriteString("- medium: clearly different but same series\n")
	builder.WriteString("- strong: stronger composition and motif variation while keeping the same core selling point and visual style\n")
	builder.WriteString(fmt.Sprintf("Source prompt: %q\n", strings.TrimSpace(req.Prompt)))
	if req.PrintableWidth > 0 && req.PrintableHeight > 0 {
		builder.WriteString(fmt.Sprintf("Print area hint: %d x %d pixels.\n", req.PrintableWidth, req.PrintableHeight))
	}
	if req.TransparentBackground {
		builder.WriteString("Keep the artwork compatible with transparent-background output.\n")
	}
	if len(studioDesignReferenceImageURLs(req.ProductReferenceImageURLs)) > 0 {
		builder.WriteString("Reference product images exist; keep prompts compatible with the same product family and print area.\n")
	}
	return builder.String()
}

func parseStudioDesignSiblingThemes(raw string, count int) ([]string, error) {
	cleaned := strings.TrimSpace(jsonx.CleanLLMResponse(raw))
	if cleaned == "" {
		return nil, fmt.Errorf("diversified prompt response is empty")
	}
	var prompts []string
	if err := json.Unmarshal([]byte(cleaned), &prompts); err != nil {
		var payload studioSiblingPromptResponse
		if objectErr := json.Unmarshal([]byte(cleaned), &payload); objectErr != nil {
			return nil, fmt.Errorf("parse diversified prompts: %w", err)
		}
		prompts = payload.Prompts
	}
	themes := make([]string, 0, count)
	seen := make(map[string]struct{}, count)
	for _, item := range prompts {
		theme := strings.TrimSpace(item)
		if theme == "" {
			continue
		}
		key := strings.ToLower(theme)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		themes = append(themes, theme)
		if len(themes) >= count {
			break
		}
	}
	if len(themes) != count {
		return nil, fmt.Errorf("expected %d diversified prompts, got %d", count, len(themes))
	}
	return themes, nil
}

func resolveStudioDesignImageModel(req *StudioDesignRequest, fallback string) string {
	if req != nil && req.TransparentBackground {
		return studioDesignTransparentModel
	}
	if req != nil {
		if model := strings.TrimSpace(req.ImageModel); model != "" {
			return model
		}
	}
	return strings.TrimSpace(fallback)
}

func studioDesignReferenceImageURLs(urls []string) []string {
	const maxStudioDesignReferenceImages = 5
	cleaned := make([]string, 0, min(len(urls), maxStudioDesignReferenceImages))
	seen := make(map[string]struct{}, len(urls))
	for _, raw := range urls {
		item := strings.TrimSpace(raw)
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		cleaned = append(cleaned, item)
		if len(cleaned) >= maxStudioDesignReferenceImages {
			break
		}
	}
	return cleaned
}

func resolveStudioDesignSize(width int, height int) string {
	return "auto"
}

func failedStudioImageIndexes(images []StudioGeneratedImage) []int {
	indexes := make([]int, 0, len(images))
	for idx, image := range images {
		if strings.TrimSpace(image.ImageURL) == "" {
			indexes = append(indexes, idx)
		}
	}
	return indexes
}

func firstNonNilError(errs []error) error {
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}

func compactStudioGenerationError(err error) string {
	message := strings.TrimSpace(err.Error())
	if message == "" {
		return "未知错误"
	}
	message = strings.Join(strings.Fields(message), " ")
	if len([]rune(message)) > 120 {
		runes := []rune(message)
		return string(runes[:120]) + "..."
	}
	return message
}

func (s *service) persistGeneratedStudioImage(ctx context.Context, response *openaiclient.ImageResponse, filename string) (string, string, error) {
	return s.taskStudioMediaOrDefault().persistGeneratedStudioImage(ctx, response, filename)
}

func decodeGeneratedImageData(ctx context.Context, image openaiclient.ImageData) ([]byte, string, error) {
	if strings.TrimSpace(image.B64JSON) != "" {
		data, err := base64.StdEncoding.DecodeString(image.B64JSON)
		if err != nil {
			return nil, "", fmt.Errorf("decode generated image: %w", err)
		}
		return data, http.DetectContentType(data), nil
	}
	if strings.TrimSpace(image.URL) == "" {
		return nil, "", fmt.Errorf("generated image contains neither b64_json nor url")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, image.URL, nil)
	if err != nil {
		return nil, "", err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("download generated image: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, "", fmt.Errorf("download generated image returned status %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return nil, "", fmt.Errorf("read generated image: %w", err)
	}
	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = http.DetectContentType(data)
	}
	return data, contentType, nil
}
