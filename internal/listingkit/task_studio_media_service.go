package listingkit

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	openaiclient "task-processor/internal/infra/clients/openai"
)

type taskStudioMediaServiceConfig struct {
	imageGenerator        openaiclient.ImageGenerator
	promptDiversifier     openaiclient.ChatCompleter
	uploadStoreConfigured bool
	uploadImages          func(context.Context, *UploadImagesRequest) (*UploadImagesResponse, error)
}

type taskStudioMediaService struct {
	imageGenerator        openaiclient.ImageGenerator
	promptDiversifier     openaiclient.ChatCompleter
	uploadStoreConfigured bool
	uploadImages          func(context.Context, *UploadImagesRequest) (*UploadImagesResponse, error)
}

func newTaskStudioMediaService(config taskStudioMediaServiceConfig) *taskStudioMediaService {
	return &taskStudioMediaService{
		imageGenerator:        config.imageGenerator,
		promptDiversifier:     config.promptDiversifier,
		uploadStoreConfigured: config.uploadStoreConfigured,
		uploadImages:          config.uploadImages,
	}
}

func (s *taskStudioMediaService) GenerateStudioDesigns(ctx context.Context, req *StudioDesignRequest) (*StudioDesignResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("invalid request: request is required")
	}
	theme := strings.TrimSpace(req.Prompt)
	if theme == "" {
		return nil, fmt.Errorf("invalid request: prompt is required")
	}
	if s.imageGenerator == nil {
		return nil, fmt.Errorf("studio image generator is not configured")
	}

	count := req.Count
	if count <= 0 {
		count = 1
	}
	if count > maxStudioDesignCount {
		count = maxStudioDesignCount
	}
	model := resolveStudioDesignImageModel(req, s.imageGenerator.GetDefaultModel())
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
	generateOne := func(idx int) {
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
		errs[idx] = nil
		images[idx] = StudioGeneratedImage{
			ID:                    uuid.NewString(),
			ImageURL:              imageURL,
			Prompt:                theme,
			RevisedPrompt:         revisedPrompt,
			ImageModel:            model,
			TransparentBackground: response.TransparentBackground,
			VariationIntensity:    req.VariationIntensity,
		}
	}
	var wg sync.WaitGroup
	for idx := 0; idx < count; idx++ {
		idx := idx
		wg.Add(1)
		go func() {
			defer wg.Done()
			generateOne(idx)
		}()
	}
	wg.Wait()
	failedIndexes := failedStudioImageIndexes(images)
	if len(failedIndexes) > 0 && len(failedIndexes) < count {
		for _, idx := range failedIndexes {
			generateOne(idx)
		}
	}
	for _, image := range images {
		if strings.TrimSpace(image.ImageURL) != "" {
			response.Images = append(response.Images, image)
		}
	}
	if diversifyErr != nil {
		response.Warnings = append(response.Warnings, "款式变体提示词生成失败，已回退为基础提示词重复生成。")
	}
	if len(response.Images) > 0 && len(response.Images) < count {
		failureCount := count - len(response.Images)
		warning := fmt.Sprintf("请求生成 %d 款，实际仅成功 %d 款，另外 %d 款生成失败。", count, len(response.Images), failureCount)
		if firstErr := firstNonNilError(errs); firstErr != nil {
			warning = fmt.Sprintf("%s 首个失败原因：%s", warning, compactStudioGenerationError(firstErr))
		}
		response.Warnings = append(response.Warnings, warning)
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

func (s *taskStudioMediaService) generateStudioDesignSiblingThemes(ctx context.Context, req *StudioDesignRequest, count int) ([]string, error) {
	baseTheme := strings.TrimSpace(req.Prompt)
	if count <= 1 || baseTheme == "" {
		return buildFallbackStudioDesignThemes(baseTheme, count), nil
	}
	if s.promptDiversifier == nil {
		return buildFallbackStudioDesignThemes(baseTheme, count), nil
	}
	timeoutCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	response, err := s.promptDiversifier.Generate(timeoutCtx, buildStudioDesignSiblingPromptRequest(req, count))
	if err != nil {
		return buildFallbackStudioDesignThemes(baseTheme, count), fmt.Errorf("diversify studio prompts: %w", err)
	}
	themes, parseErr := parseStudioDesignSiblingThemes(response, count)
	if parseErr != nil {
		return buildFallbackStudioDesignThemes(baseTheme, count), parseErr
	}
	return themes, nil
}

func (s *taskStudioMediaService) generateStudioDesignImage(ctx context.Context, model string, promptText string, size string, referenceURLs []string) (*openaiclient.ImageResponse, error) {
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

func (s *taskStudioMediaService) editStudioDesignImageWithReferences(ctx context.Context, model string, promptText string, size string, referenceURLs []string) (*openaiclient.ImageResponse, error) {
	return s.imageGenerator.EditImage(ctx, &openaiclient.ImageEditRequest{
		Model:          model,
		Prompt:         promptText,
		ImageURL:       referenceURLs[0],
		ImageURLs:      referenceURLs,
		Size:           size,
		ResponseFormat: "b64_json",
		N:              1,
	})
}

func (s *taskStudioMediaService) generateStudioDesignImageWithoutReferences(ctx context.Context, model string, promptText string, size string) (*openaiclient.ImageResponse, error) {
	return s.imageGenerator.GenerateImage(ctx, &openaiclient.ImageGenerateRequest{
		Model:          model,
		Prompt:         promptText,
		Size:           size,
		ResponseFormat: "b64_json",
		N:              1,
	})
}

func (s *taskStudioMediaService) persistGeneratedStudioImage(ctx context.Context, response *openaiclient.ImageResponse, filename string) (string, string, error) {
	if response == nil || len(response.Data) == 0 {
		return "", "", fmt.Errorf("studio image generation returned no image data")
	}
	if s == nil || s.uploadImages == nil {
		return "", "", fmt.Errorf("image upload store is not configured")
	}
	first := response.Data[0]
	data, contentType, err := decodeGeneratedImageData(ctx, first)
	if err != nil {
		return "", "", err
	}
	upload, err := s.uploadImages(ctx, &UploadImagesRequest{Files: []ImageUploadInput{{
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

func (s *taskStudioMediaService) GenerateStudioProductImages(ctx context.Context, req *StudioProductImageRequest) (*StudioProductImageResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("invalid request: request is required")
	}
	theme := strings.TrimSpace(req.Prompt)
	if theme == "" {
		return nil, fmt.Errorf("invalid request: prompt is required")
	}
	sourceURL := strings.TrimSpace(req.SourceDesignURL)
	if sourceURL == "" {
		return nil, fmt.Errorf("invalid request: source_design_url is required")
	}
	if s.imageGenerator == nil {
		return nil, fmt.Errorf("studio image generator is not configured")
	}

	count := req.Count
	if count <= 0 {
		count = maxStudioProductImageCount
	}
	if count > maxStudioProductImageCount {
		count = maxStudioProductImageCount
	}

	roles := selectStudioProductImageRoles(count)
	images := make([]StudioGeneratedImage, len(roles))
	errs := make([]error, len(roles))
	sem := make(chan struct{}, studioProductImageConcurrencyLimit(len(roles)))
	var wg sync.WaitGroup
	for idx, role := range roles {
		idx, role := idx, role
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			promptText := buildStudioProductImagePrompt(req, role, idx+1, len(roles))
			imageURL, err := s.generateOneStudioProductImage(ctx, req, sourceURL, promptText)
			if err != nil {
				errs[idx] = fmt.Errorf("%s: %w", role.Label, err)
				return
			}
			images[idx] = StudioGeneratedImage{
				ID:            uuid.NewString(),
				ImageURL:      imageURL,
				RevisedPrompt: fmt.Sprintf("%s %s", firstNonEmptyString(req.StyleName, req.ProductName, "AI"), role.Label),
				Role:          role.Key,
				RoleLabel:     role.Label,
			}
		}()
	}
	wg.Wait()

	response := &StudioProductImageResponse{
		Images: make([]StudioGeneratedImage, 0, len(roles)),
	}
	for _, image := range images {
		if strings.TrimSpace(image.ImageURL) != "" {
			response.Images = append(response.Images, image)
		}
	}
	if len(response.Images) == 0 {
		return nil, errors.Join(nonNilErrors(errs)...)
	}
	return response, nil
}

func (s *taskStudioMediaService) generateOneStudioProductImage(ctx context.Context, req *StudioProductImageRequest, sourceURL string, basePrompt string) (string, error) {
	inputImages := studioProductImageInputURLs(sourceURL, req.ProductReferenceImageURLs)
	generated, err := s.tryGenerateStudioProductImage(ctx, inputImages, strings.TrimSpace(basePrompt))
	if err != nil && isStudioInputFormatError(err) {
		sanitizedURLs, sanitizeErr := s.sanitizeStudioImageInputURLs(ctx, inputImages)
		if sanitizeErr == nil {
			generated, err = s.tryGenerateStudioProductImage(ctx, sanitizedURLs, strings.TrimSpace(basePrompt))
		}
	}
	if err != nil {
		return "", fmt.Errorf("generate product image: %w", err)
	}
	imageURL, _, err := s.persistGeneratedStudioImage(ctx, generated, "studio-product-image.png")
	return imageURL, err
}

func (s *taskStudioMediaService) tryGenerateStudioProductImage(ctx context.Context, inputImages []string, promptText string) (*openaiclient.ImageResponse, error) {
	generated, err := s.imageGenerator.EditImage(ctx, &openaiclient.ImageEditRequest{
		Model:          s.imageGenerator.GetDefaultModel(),
		Prompt:         promptText,
		ImageURL:       inputImages[0],
		ImageURLs:      inputImages,
		Size:           "auto",
		ResponseFormat: "b64_json",
		N:              1,
	})
	if err != nil {
		generated, err = s.imageGenerator.EditImage(ctx, &openaiclient.ImageEditRequest{
			Model:          s.imageGenerator.GetDefaultModel(),
			Prompt:         promptText,
			ImageURL:       inputImages[0],
			ImageURLs:      inputImages[:1],
			Size:           "auto",
			ResponseFormat: "b64_json",
			N:              1,
		})
		if err != nil {
			return nil, err
		}
	}
	return generated, nil
}

func (s *taskStudioMediaService) sanitizeStudioImageInputURLs(ctx context.Context, inputURLs []string) ([]string, error) {
	if s == nil || !s.uploadStoreConfigured || s.uploadImages == nil {
		return nil, fmt.Errorf("image upload store is not configured")
	}
	sanitized := make([]string, 0, len(inputURLs))
	files := make([]ImageUploadInput, 0, len(inputURLs))
	for idx, rawURL := range inputURLs {
		imageURL := strings.TrimSpace(rawURL)
		if imageURL == "" {
			continue
		}
		data, filename, err := downloadAndConvertStudioInputImage(ctx, imageURL, idx)
		if err != nil {
			return nil, err
		}
		files = append(files, ImageUploadInput{
			Filename:    filename,
			ContentType: "image/jpeg",
			Data:        data,
		})
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("no image inputs available to sanitize")
	}
	uploaded, err := s.uploadImages(ctx, &UploadImagesRequest{Files: files})
	if err != nil {
		return nil, fmt.Errorf("upload sanitized studio inputs: %w", err)
	}
	sanitized = append(sanitized, uploaded.ImageURLs...)
	return sanitized, nil
}
