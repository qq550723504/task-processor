package listingkit

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/google/uuid"
)

type taskStudioMediaServiceConfig struct {
	imageGenerator        AIImageGenerator
	promptDiversifier     AIChatCompleter
	uploadStoreConfigured bool
	uploadImages          func(context.Context, *UploadImagesRequest) (*UploadImagesResponse, error)
}

type taskStudioMediaService struct {
	imageGenerator        AIImageGenerator
	promptDiversifier     AIChatCompleter
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
