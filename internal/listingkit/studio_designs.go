package listingkit

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	openaiclient "task-processor/internal/infra/clients/openai"
	"task-processor/internal/pkg/jsonx"
	"task-processor/internal/prompt"
)

const maxStudioDesignCount = 5
const studioDesignTransparentModel = "gpt-image-2"
const (
	studioVariationLight  = "light"
	studioVariationMedium = "medium"
	studioVariationStrong = "strong"
)

func (s *service) GenerateStudioDesigns(ctx context.Context, req *StudioDesignRequest) (*StudioDesignResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("invalid request: request is required")
	}
	theme := strings.TrimSpace(req.Prompt)
	if theme == "" {
		return nil, fmt.Errorf("invalid request: prompt is required")
	}
	if s.studioImageGenerator == nil {
		return nil, fmt.Errorf("studio image generator is not configured")
	}

	count := req.Count
	if count <= 0 {
		count = 1
	}
	if count > maxStudioDesignCount {
		count = maxStudioDesignCount
	}
	model := resolveStudioDesignImageModel(req, s.studioImageGenerator.GetDefaultModel())
	themes, diversifyErr := s.generateStudioDesignSiblingThemes(ctx, req, count)
	if len(themes) != count {
		themes = buildFallbackStudioDesignThemes(req.Prompt, count)
	}
	size := resolveStudioDesignSize(req.PrintableWidth, req.PrintableHeight)
	referenceURLs := studioDesignReferenceImageURLs(req.ProductReferenceImageURLs)

	response := &StudioDesignResponse{
		Prompt:                theme,
		PrintableWidth:        req.PrintableWidth,
		PrintableHeight:       req.PrintableHeight,
		ImageModel:            model,
		TransparentBackground: req.TransparentBackground && model == studioDesignTransparentModel,
		Images:                make([]StudioGeneratedImage, 0, count),
	}
	images := make([]StudioGeneratedImage, count)
	errs := make([]error, count)
	var wg sync.WaitGroup
	for idx := 0; idx < count; idx++ {
		idx := idx
		wg.Add(1)
		go func() {
			defer wg.Done()
			promptText := buildStudioDesignPromptWithTheme(req, themes[idx])
			generated, err := s.generateStudioDesignImage(ctx, model, promptText, size, referenceURLs)
			if err != nil {
				errs[idx] = fmt.Errorf("generate studio design %d: %w", idx+1, err)
				return
			}
			imageURL, revisedPrompt, err := s.persistGeneratedStudioImage(ctx, generated, fmt.Sprintf("studio-design-%d.png", idx+1))
			if err != nil {
				errs[idx] = fmt.Errorf("persist studio design %d: %w", idx+1, err)
				return
			}
			images[idx] = StudioGeneratedImage{
				ID:                    uuid.NewString(),
				ImageURL:              imageURL,
				Prompt:                theme,
				RevisedPrompt:         revisedPrompt,
				ImageModel:            model,
				TransparentBackground: response.TransparentBackground,
				VariationIntensity:    req.VariationIntensity,
			}
		}()
	}
	wg.Wait()
	for _, image := range images {
		if strings.TrimSpace(image.ImageURL) != "" {
			response.Images = append(response.Images, image)
		}
	}
	if len(response.Images) == 0 {
		errList := nonNilErrors(errs)
		if diversifyErr != nil {
			errList = append(errList, diversifyErr)
		}
		return nil, errors.Join(errList...)
	}
	return response, nil
}

func (s *service) generateStudioDesignSiblingThemes(ctx context.Context, req *StudioDesignRequest, count int) ([]string, error) {
	baseTheme := strings.TrimSpace(req.Prompt)
	if count <= 1 || baseTheme == "" {
		return buildFallbackStudioDesignThemes(baseTheme, count), nil
	}
	if s.studioPromptDiversifier == nil {
		return buildFallbackStudioDesignThemes(baseTheme, count), nil
	}
	timeoutCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	response, err := s.studioPromptDiversifier.Generate(timeoutCtx, buildStudioDesignSiblingPromptRequest(req, count))
	if err != nil {
		return buildFallbackStudioDesignThemes(baseTheme, count), fmt.Errorf("diversify studio prompts: %w", err)
	}
	themes, parseErr := parseStudioDesignSiblingThemes(response, count)
	if parseErr != nil {
		return buildFallbackStudioDesignThemes(baseTheme, count), parseErr
	}
	return themes, nil
}

func (s *service) generateStudioDesignImage(ctx context.Context, model string, promptText string, size string, referenceURLs []string) (*openaiclient.ImageResponse, error) {
	if len(referenceURLs) == 0 {
		return s.generateStudioDesignImageWithoutReferences(ctx, model, promptText, size)
	}
	response, err := s.editStudioDesignImageWithReferences(ctx, model, promptText, size, referenceURLs)
	if err == nil {
		return response, nil
	}
	if len(referenceURLs) > 1 {
		response, singleErr := s.editStudioDesignImageWithReferences(ctx, model, promptText, size, referenceURLs[:1])
		if singleErr == nil {
			return response, nil
		}
	}
	return s.generateStudioDesignImageWithoutReferences(ctx, model, promptText, size)
}

func (s *service) editStudioDesignImageWithReferences(ctx context.Context, model string, promptText string, size string, referenceURLs []string) (*openaiclient.ImageResponse, error) {
	return s.studioImageGenerator.EditImage(ctx, &openaiclient.ImageEditRequest{
		Model:          model,
		Prompt:         promptText,
		ImageURL:       referenceURLs[0],
		ImageURLs:      referenceURLs,
		Size:           size,
		ResponseFormat: "b64_json",
		N:              1,
	})
}

func (s *service) generateStudioDesignImageWithoutReferences(ctx context.Context, model string, promptText string, size string) (*openaiclient.ImageResponse, error) {
	return s.studioImageGenerator.GenerateImage(ctx, &openaiclient.ImageGenerateRequest{
		Model:          model,
		Prompt:         promptText,
		Size:           size,
		ResponseFormat: "b64_json",
		N:              1,
	})
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
	if width <= 0 || height <= 0 {
		return "1024x1024"
	}
	ratio := float64(width) / float64(height)
	if ratio >= 1.18 {
		return "1536x1024"
	}
	if ratio <= 0.84 {
		return "1024x1536"
	}
	return "1024x1024"
}

func (s *service) persistGeneratedStudioImage(ctx context.Context, response *openaiclient.ImageResponse, filename string) (string, string, error) {
	if response == nil || len(response.Data) == 0 {
		return "", "", fmt.Errorf("studio image generation returned no image data")
	}
	first := response.Data[0]
	data, contentType, err := decodeGeneratedImageData(ctx, first)
	if err != nil {
		return "", "", err
	}
	upload, err := s.UploadImages(ctx, &UploadImagesRequest{Files: []ImageUploadInput{{
		Filename:    filename,
		ContentType: contentType,
		Data:        data,
	}}})
	if err != nil {
		return "", "", err
	}
	if len(upload.ImageURLs) == 0 {
		return "", "", fmt.Errorf("uploaded generated image but no url returned")
	}
	return upload.ImageURLs[0], first.RevisedPrompt, nil
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
