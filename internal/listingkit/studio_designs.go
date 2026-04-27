package listingkit

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/google/uuid"

	openaiclient "task-processor/internal/infra/clients/openai"
	"task-processor/internal/prompt"
)

const maxStudioDesignCount = 8

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
	promptText := buildStudioDesignPrompt(req)
	size := resolveStudioDesignSize(req.PrintableWidth, req.PrintableHeight)
	referenceURLs := studioDesignReferenceImageURLs(req.ProductReferenceImageURLs)

	response := &StudioDesignResponse{
		Prompt:                theme,
		PrintableWidth:        req.PrintableWidth,
		PrintableHeight:       req.PrintableHeight,
		TransparentBackground: false,
		Images:                make([]StudioGeneratedImage, 0, count),
	}
	for idx := 0; idx < count; idx++ {
		generated, err := s.generateStudioDesignImage(ctx, promptText, size, referenceURLs)
		if err != nil {
			return nil, fmt.Errorf("generate studio design: %w", err)
		}
		imageURL, revisedPrompt, err := s.persistGeneratedStudioImage(ctx, generated, fmt.Sprintf("studio-design-%d.png", idx+1))
		if err != nil {
			return nil, err
		}
		response.Images = append(response.Images, StudioGeneratedImage{
			ID:            uuid.NewString(),
			ImageURL:      imageURL,
			RevisedPrompt: revisedPrompt,
		})
	}
	return response, nil
}

func (s *service) generateStudioDesignImage(ctx context.Context, promptText string, size string, referenceURLs []string) (*openaiclient.ImageResponse, error) {
	if len(referenceURLs) == 0 {
		return s.generateStudioDesignImageWithoutReferences(ctx, promptText, size)
	}
	response, err := s.editStudioDesignImageWithReferences(ctx, promptText, size, referenceURLs)
	if err == nil {
		return response, nil
	}
	if len(referenceURLs) > 1 {
		response, singleErr := s.editStudioDesignImageWithReferences(ctx, promptText, size, referenceURLs[:1])
		if singleErr == nil {
			return response, nil
		}
	}
	return s.generateStudioDesignImageWithoutReferences(ctx, promptText, size)
}

func (s *service) editStudioDesignImageWithReferences(ctx context.Context, promptText string, size string, referenceURLs []string) (*openaiclient.ImageResponse, error) {
	return s.studioImageGenerator.EditImage(ctx, &openaiclient.ImageEditRequest{
		Model:          s.studioImageGenerator.GetDefaultModel(),
		Prompt:         promptText,
		ImageURL:       referenceURLs[0],
		ImageURLs:      referenceURLs,
		Size:           size,
		ResponseFormat: "b64_json",
		N:              1,
	})
}

func (s *service) generateStudioDesignImageWithoutReferences(ctx context.Context, promptText string, size string) (*openaiclient.ImageResponse, error) {
	return s.studioImageGenerator.GenerateImage(ctx, &openaiclient.ImageGenerateRequest{
		Model:          s.studioImageGenerator.GetDefaultModel(),
		Prompt:         promptText,
		Size:           size,
		ResponseFormat: "b64_json",
		N:              1,
	})
}

func buildStudioDesignPrompt(req *StudioDesignRequest) string {
	printableHint := ""
	if req.PrintableWidth > 0 && req.PrintableHeight > 0 {
		printableHint = fmt.Sprintf("Target print area: %d by %d pixels.", req.PrintableWidth, req.PrintableHeight)
	}
	referenceHint := ""
	if len(studioDesignReferenceImageURLs(req.ProductReferenceImageURLs)) > 0 {
		referenceHint = "SDS product mockup/reference images are provided. Use them to understand product color variants, material, print-surface shape, scale, and visual contrast. Generate only the flat artwork/design, not a product photo; make the artwork work across the provided product colors."
	}
	vars := map[string]any{
		"PrintableHint": printableHint,
		"ReferenceHint": referenceHint,
		"ThemePrompt":   strings.TrimSpace(req.Prompt),
	}
	fallback := "Create a single print-ready graphic for ecommerce POD or customized-product use. Return a flat design only, not a product mockup, model photo, scene photo, or physical product rendering. {{PrintableHint}} {{ReferenceHint}} Theme prompt: {{ThemePrompt}}"
	if prompt.GlobalRegistry == nil {
		return renderPromptFallback(fallback, vars)
	}
	rendered, err := prompt.GlobalRegistry.Render(prompt.KProductImageStudioGenerationPodDesign, vars, fallback)
	if err != nil {
		return renderPromptFallback(fallback, vars)
	}
	return strings.TrimSpace(rendered)
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
