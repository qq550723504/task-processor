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
	imageGenerator                AIImageGenerator
	backgroundRemover             StudioBackgroundRemover
	backgroundRemovalConcurrency  int
	promptDiversifier             AIChatCompleter
	uploadStoreConfigured         bool
	uploadImages                  func(context.Context, *UploadImagesRequest) (*UploadImagesResponse, error)
	loadUploadedImage             func(context.Context, string) (*UploadedImageFile, error)
	resolveUploadedImagePublicURL func(context.Context, string) (string, error)
}

type taskStudioMediaService struct {
	imageGenerator                AIImageGenerator
	backgroundRemover             StudioBackgroundRemover
	backgroundRemovalConcurrency  int
	backgroundRemovalSlots        chan struct{}
	backgroundRemovalOnce         sync.Once
	promptDiversifier             AIChatCompleter
	uploadStoreConfigured         bool
	uploadImages                  func(context.Context, *UploadImagesRequest) (*UploadImagesResponse, error)
	loadUploadedImage             func(context.Context, string) (*UploadedImageFile, error)
	resolveUploadedImagePublicURL func(context.Context, string) (string, error)
}

func newTaskStudioMediaService(config taskStudioMediaServiceConfig) *taskStudioMediaService {
	service := &taskStudioMediaService{
		imageGenerator:                config.imageGenerator,
		backgroundRemover:             config.backgroundRemover,
		backgroundRemovalConcurrency:  config.backgroundRemovalConcurrency,
		promptDiversifier:             config.promptDiversifier,
		uploadStoreConfigured:         config.uploadStoreConfigured,
		uploadImages:                  config.uploadImages,
		loadUploadedImage:             config.loadUploadedImage,
		resolveUploadedImagePublicURL: config.resolveUploadedImagePublicURL,
	}
	service.initializeBackgroundRemovalSlots()
	return service
}

func (s *taskStudioMediaService) validateStudioDesignGenerationRequest(ctx context.Context, req *StudioDesignRequest) (string, []string, error) {
	if req == nil {
		return "", nil, fmt.Errorf("invalid request: request is required")
	}
	theme := strings.TrimSpace(req.Prompt)
	mode := strings.TrimSpace(req.ArtworkGenerationMode)
	if theme == "" && !strings.EqualFold(mode, studioArtworkGenerationModeHotReference) {
		return "", nil, fmt.Errorf("invalid request: prompt is required")
	}
	if err := s.resolveStudioDesignReferenceImageURLs(ctx, req); err != nil {
		return "", nil, err
	}
	referenceURLs, err := validateStudioDesignReferenceImageURLs(req)
	if err != nil {
		return "", nil, err
	}
	req.ProductReferenceImageURLs = referenceURLs
	if err := validateStudioDesignPromptRequirement(req, referenceURLs); err != nil {
		return "", nil, err
	}
	return theme, referenceURLs, nil
}

func (s *taskStudioMediaService) SubmitStudioDesignsAsync(ctx context.Context, req *StudioDesignRequest) (*studioDesignAsyncSubmitResponse, error) {
	theme, referenceURLs, err := s.validateStudioDesignGenerationRequest(ctx, req)
	if err != nil {
		return nil, err
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
	if count != 1 {
		return nil, ErrAsyncImageGenerationNotSupported
	}
	model := resolveStudioDesignImageModel(req, s.imageGenerator.GetDefaultModel())
	size := resolveStudioDesignSize(req.PrintableWidth, req.PrintableHeight)
	promptText := buildStudioDesignPromptWithContext(ctx, req, theme)
	if len(referenceURLs) > 0 {
		if hasOwnedListingKitUploadReference(referenceURLs) {
			generated, err := s.editStudioDesignImageWithReferences(ctx, model, promptText, size, referenceURLs)
			if err != nil {
				return nil, fmt.Errorf("edit studio design with uploaded reference: %w", err)
			}
			return s.buildStudioDesignAsyncSubmitResponse(ctx, req, &AIImageAsyncSubmit{
				Provider: "listingkit-sync",
				Status:   AIImageAsyncResultSucceeded,
				Response: generated,
			})
		}
		asyncGenerator, ok := s.imageGenerator.(AIAsyncImageGenerator)
		if !ok {
			return nil, ErrAsyncImageGenerationNotSupported
		}
		submit, err := asyncGenerator.SubmitImageEdit(ctx, &AIImageEditRequest{
			Model:          model,
			Prompt:         promptText,
			ImageURL:       referenceURLs[0],
			ImageURLs:      referenceURLs,
			Size:           size,
			ResponseFormat: "b64_json",
			N:              1,
		})
		if err != nil {
			return nil, err
		}
		return s.buildStudioDesignAsyncSubmitResponse(ctx, req, submit)
	}
	asyncGenerator, ok := s.imageGenerator.(AIAsyncImageGenerator)
	if !ok {
		return nil, ErrAsyncImageGenerationNotSupported
	}
	submit, err := asyncGenerator.SubmitImageGeneration(ctx, &AIImageGenerateRequest{
		Model:          model,
		Prompt:         promptText,
		Size:           size,
		ResponseFormat: "b64_json",
		N:              1,
	})
	if err != nil {
		return nil, err
	}
	return s.buildStudioDesignAsyncSubmitResponse(ctx, req, submit)
}

func hasOwnedListingKitUploadReference(referenceURLs []string) bool {
	for _, referenceURL := range referenceURLs {
		if _, ok := studioReferenceUploadedImageKeyFromURL(referenceURL); ok {
			return true
		}
	}
	return false
}

func (s *taskStudioMediaService) QueryStudioDesignsAsync(ctx context.Context, req *StudioDesignRequest, jobID string) (*studioDesignAsyncQueryResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("invalid request: request is required")
	}
	if strings.TrimSpace(jobID) == "" {
		return nil, fmt.Errorf("invalid request: job id is required")
	}
	if s.imageGenerator == nil {
		return nil, fmt.Errorf("studio image generator is not configured")
	}

	asyncGenerator, ok := s.imageGenerator.(AIAsyncImageGenerator)
	if !ok {
		return nil, ErrAsyncImageGenerationNotSupported
	}

	result, err := asyncGenerator.QueryImageGeneration(ctx, jobID)
	if err != nil {
		return nil, err
	}
	output := &studioDesignAsyncQueryResponse{Result: result}
	if result == nil || result.Status != AIImageAsyncResultSucceeded {
		return output, nil
	}

	response, err := s.materializeAsyncStudioDesignResult(ctx, req, result)
	if err != nil {
		return nil, err
	}
	output.Response = response
	return output, nil
}

func (s *taskStudioMediaService) GenerateStudioDesigns(ctx context.Context, req *StudioDesignRequest) (*StudioDesignResponse, error) {
	theme, referenceURLs, err := s.validateStudioDesignGenerationRequest(ctx, req)
	if err != nil {
		return nil, err
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
	response := &StudioDesignResponse{
		Prompt:                    theme,
		PrintableWidth:            req.PrintableWidth,
		PrintableHeight:           req.PrintableHeight,
		ImageModel:                model,
		TransparentBackground:     studioDesignTransparencyMode(req) != StudioTransparencyModeNone,
		TransparentBackgroundMode: studioDesignTransparencyMode(req),
		Images:                    make([]StudioGeneratedImage, 0, count),
	}
	images := make([]StudioGeneratedImage, count)
	errs := make([]error, count)
	generatedResponses := make([]*AIImageResponse, count)
	generateOne := func(idx int) {
		promptText := buildStudioDesignPromptWithContext(ctx, req, themes[idx])
		generated, err := s.generateStudioDesignImageWithPolicy(
			ctx,
			model,
			promptText,
			size,
			referenceURLs,
			strings.EqualFold(strings.TrimSpace(req.ArtworkGenerationMode), studioArtworkGenerationModeHotReference),
		)
		if err != nil {
			errs[idx] = fmt.Errorf("generate studio design %d: %w", idx+1, err)
			return
		}
		generatedResponses[idx] = generated
		processed, err := s.processStudioDesignImage(ctx, req, generated, fmt.Sprintf("studio-design-%d.png", idx+1))
		if err != nil {
			errs[idx] = fmt.Errorf("persist studio design %d: %w", idx+1, err)
			return
		}
		errs[idx] = nil
		images[idx] = StudioGeneratedImage{
			ID:                        uuid.NewString(),
			ImageURL:                  processed.ImageURL,
			OriginalImageURL:          processed.OriginalImageURL,
			Prompt:                    theme,
			RevisedPrompt:             processed.RevisedPrompt,
			ImageModel:                model,
			TransparentBackground:     response.TransparentBackground,
			TransparentBackgroundMode: processed.TransparentBackgroundMode,
			BackgroundRemovalStatus:   processed.BackgroundRemovalStatus,
			BackgroundRemovalModel:    processed.BackgroundRemovalModel,
			BackgroundRemovalError:    processed.BackgroundRemovalError,
			VariationIntensity:        req.VariationIntensity,
			RequestID:                 strings.TrimSpace(generated.RequestID),
			UpstreamJobID:             strings.TrimSpace(generated.UpstreamJobID),
			RawResponse:               strings.TrimSpace(generated.RawResponse),
			Usage:                     generated.Usage,
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
	for idx, image := range images {
		if strings.TrimSpace(image.ImageURL) != "" {
			response.Images = append(response.Images, image)
			metadata := generatedResponses[idx]
			if metadata != nil {
				if response.RequestID == "" {
					response.RequestID = strings.TrimSpace(metadata.RequestID)
				}
				if response.UpstreamJobID == "" {
					response.UpstreamJobID = strings.TrimSpace(metadata.UpstreamJobID)
				}
				if response.RawResponse == "" {
					response.RawResponse = strings.TrimSpace(metadata.RawResponse)
				}
				response.Usage.PromptTokens += metadata.Usage.PromptTokens
				response.Usage.CompletionTokens += metadata.Usage.CompletionTokens
				response.Usage.TotalTokens += metadata.Usage.TotalTokens
			}
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
