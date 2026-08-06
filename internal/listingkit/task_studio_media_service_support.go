package listingkit

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
)

type studioDesignAsyncQueryResponse struct {
	Result   *AIImageAsyncResult
	Response *StudioDesignResponse
}

type studioDesignAsyncSubmitResponse struct {
	Submit   *AIImageAsyncSubmit
	Response *StudioDesignResponse
}

type studioBackgroundRemovalMaterialization struct {
	ImageURL string
	Model    string
}

func (s *taskStudioMediaService) retryStudioBackgroundRemoval(ctx context.Context, sourceURL string, filename string) (*studioBackgroundRemovalMaterialization, error) {
	if s == nil || s.backgroundRemover == nil {
		return nil, fmt.Errorf("background removal client is not configured")
	}
	data, contentType, err := loadStudioBackgroundRemovalSource(ctx, sourceURL, s.loadUploadedImage)
	if err != nil {
		return nil, err
	}
	removed, err := s.removeStudioBackground(ctx, sourceURL, data, contentType)
	if err != nil {
		return nil, err
	}
	if removed == nil {
		return nil, fmt.Errorf("background removal returned no result")
	}
	if err := validateStudioTransparentPNG(removed.Data); err != nil {
		return nil, err
	}
	imageURL, err := s.persistStudioImageBytes(ctx, removed.Data, "image/png", filename)
	if err != nil {
		return nil, err
	}
	return &studioBackgroundRemovalMaterialization{
		ImageURL: imageURL,
		Model:    strings.TrimSpace(removed.Model),
	}, nil
}

func loadStudioBackgroundRemovalSource(ctx context.Context, sourceURL string, loadUploadedImage func(context.Context, string) (*UploadedImageFile, error)) ([]byte, string, error) {
	trimmed := strings.TrimSpace(sourceURL)
	if key, ok := studioReferenceUploadedImageKeyFromURL(trimmed); ok {
		if loadUploadedImage == nil {
			return nil, "", fmt.Errorf("original uploaded image loader is not configured")
		}
		file, err := loadUploadedImage(ctx, key)
		if err != nil {
			return nil, "", fmt.Errorf("load original uploaded image: %w", err)
		}
		if file == nil || len(file.Data) == 0 {
			return nil, "", fmt.Errorf("original uploaded image is empty")
		}
		contentType := strings.TrimSpace(file.ContentType)
		if contentType == "" {
			contentType = http.DetectContentType(file.Data)
		}
		return file.Data, contentType, nil
	}
	validated, err := validateStudioReferencePublicHTTPSURL(trimmed)
	if err != nil {
		return nil, "", fmt.Errorf("load original image: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, validated, nil)
	if err != nil {
		return nil, "", fmt.Errorf("build original image request: %w", err)
	}
	resp, err := studioPublicImageHTTPClient.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("download original image: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, "", fmt.Errorf("download original image returned status %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return nil, "", fmt.Errorf("read original image: %w", err)
	}
	if len(data) == 0 {
		return nil, "", fmt.Errorf("original image is empty")
	}
	contentType := strings.TrimSpace(resp.Header.Get("Content-Type"))
	if contentType == "" || !strings.HasPrefix(strings.ToLower(contentType), "image/") {
		contentType = http.DetectContentType(data)
	}
	return data, contentType, nil
}

func (s *taskStudioMediaService) generateStudioDesignSiblingThemes(ctx context.Context, req *StudioDesignRequest, count int) ([]string, error) {
	baseTheme := strings.TrimSpace(req.Prompt)
	if count <= 1 || baseTheme == "" || isStudioRawPromptMode(req.PromptMode) {
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

func (s *taskStudioMediaService) resolveStudioDesignReferenceImageURLs(ctx context.Context, req *StudioDesignRequest) error {
	if req == nil || len(req.ProductReferenceImageURLs) == 0 || s == nil || s.resolveUploadedImagePublicURL == nil {
		return nil
	}
	resolved := make([]string, 0, len(req.ProductReferenceImageURLs))
	for _, rawURL := range req.ProductReferenceImageURLs {
		resolvedURL, err := s.resolveStudioDesignReferenceImageURL(ctx, rawURL)
		if err != nil {
			return err
		}
		resolved = append(resolved, resolvedURL)
	}
	req.ProductReferenceImageURLs = resolved
	return nil
}

func (s *taskStudioMediaService) resolveStudioDesignReferenceImageURL(ctx context.Context, rawURL string) (string, error) {
	trimmed := strings.TrimSpace(rawURL)
	if _, ok := studioReferenceUploadedImageKeyFromURL(trimmed); ok && s != nil && s.loadUploadedImage != nil {
		return trimmed, nil
	}
	if trimmed == "" || s == nil || s.resolveUploadedImagePublicURL == nil {
		return trimmed, nil
	}
	candidates := studioReferenceUploadedImageKeyCandidates(trimmed)
	for _, key := range candidates {
		publicURL, err := s.resolveUploadedImagePublicURL(ctx, key)
		if err == nil {
			validated, validateErr := validateStudioReferencePublicHTTPSURL(publicURL)
			if validateErr != nil {
				return "", fmt.Errorf("invalid request: uploaded reference image %q does not have a public https url configured", trimmed)
			}
			return validated, nil
		}
		if !errors.Is(err, ErrUploadedImageNotFound) {
			return "", fmt.Errorf("invalid request: resolve uploaded reference image %q: %w", trimmed, err)
		}
	}
	return trimmed, nil
}

func (s *taskStudioMediaService) generateStudioDesignImage(ctx context.Context, model string, promptText string, size string, referenceURLs []string) (*AIImageResponse, error) {
	return s.generateStudioDesignImageWithPolicy(ctx, model, promptText, size, referenceURLs, false)
}

func (s *taskStudioMediaService) generateStudioDesignImageWithPolicy(ctx context.Context, model string, promptText string, size string, referenceURLs []string, preserveReferenceOnError bool) (*AIImageResponse, error) {
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
	if preserveReferenceOnError {
		return nil, err
	}
	return s.generateStudioDesignImageWithoutReferences(ctx, model, promptText, size)
}

func (s *taskStudioMediaService) editStudioDesignImageWithReferences(ctx context.Context, model string, promptText string, size string, referenceURLs []string) (*AIImageResponse, error) {
	if s.loadUploadedImage != nil {
		for _, referenceURL := range referenceURLs[1:] {
			if _, ok := studioReferenceUploadedImageKeyFromURL(referenceURL); ok {
				return nil, fmt.Errorf("invalid request: uploaded listingkit image must be the primary image")
			}
		}
	}
	if key, ok := studioReferenceUploadedImageKeyFromURL(referenceURLs[0]); ok && s.loadUploadedImage != nil {
		file, err := s.loadUploadedImage(ctx, key)
		if err != nil {
			return nil, err
		}
		if file == nil || len(file.Data) == 0 || !strings.HasPrefix(strings.ToLower(strings.TrimSpace(file.ContentType)), "image/") {
			return nil, fmt.Errorf("invalid uploaded image data")
		}
		request := &AIImageEditRequest{
			Model:            model,
			Prompt:           promptText,
			ImageData:        file.Data,
			ImageContentType: file.ContentType,
			Size:             size,
			ResponseFormat:   "b64_json",
			N:                1,
		}
		if s.resolveUploadedImagePublicURL != nil {
			publicURL, resolveErr := s.resolveUploadedImagePublicURL(ctx, key)
			if resolveErr == nil {
				validatedURL, validateErr := validateStudioReferencePublicHTTPSURL(publicURL)
				if validateErr != nil {
					return nil, fmt.Errorf("invalid uploaded reference public url: %w", validateErr)
				}
				request.ImageURL = validatedURL
				request.ImageURLs = []string{validatedURL}
			}
		}
		return s.imageGenerator.EditImage(ctx, request)
	}
	return s.imageGenerator.EditImage(ctx, &AIImageEditRequest{
		Model:          model,
		Prompt:         promptText,
		ImageURL:       referenceURLs[0],
		ImageURLs:      referenceURLs,
		Size:           size,
		ResponseFormat: "b64_json",
		N:              1,
	})
}

func (s *taskStudioMediaService) generateStudioDesignImageWithoutReferences(ctx context.Context, model string, promptText string, size string) (*AIImageResponse, error) {
	return s.imageGenerator.GenerateImage(ctx, &AIImageGenerateRequest{
		Model:          model,
		Prompt:         promptText,
		Size:           size,
		ResponseFormat: "b64_json",
		N:              1,
	})
}

func (s *taskStudioMediaService) persistGeneratedStudioImage(ctx context.Context, response *AIImageResponse, filename string) (string, string, error) {
	if response == nil || len(response.Data) == 0 {
		return "", "", fmt.Errorf("studio image generation returned no image data")
	}
	first := response.Data[0]
	data, contentType, err := decodeGeneratedImageData(ctx, first)
	if err != nil {
		return "", "", err
	}
	imageURL, err := s.persistStudioImageBytes(ctx, data, contentType, filename)
	if err != nil {
		return "", "", err
	}
	return imageURL, first.RevisedPrompt, nil
}

func (s *taskStudioMediaService) persistStudioImageBytes(ctx context.Context, data []byte, contentType string, filename string) (string, error) {
	if s == nil || s.uploadImages == nil {
		return "", fmt.Errorf("image upload store is not configured")
	}
	upload, err := s.uploadImages(ctx, &UploadImagesRequest{Files: []ImageUploadInput{{
		Filename:    filename,
		ContentType: contentType,
		Data:        data,
	}}})
	if err != nil {
		return "", err
	}
	if upload == nil || len(upload.ImageURLs) == 0 {
		return "", fmt.Errorf("uploaded studio image but no url returned")
	}
	return upload.ImageURLs[0], nil
}

type studioProcessedImage struct {
	ImageURL                  string
	OriginalImageURL          string
	RevisedPrompt             string
	TransparentBackgroundMode StudioTransparencyMode
	BackgroundRemovalStatus   StudioBackgroundRemovalStatus
	BackgroundRemovalModel    string
	BackgroundRemovalError    string
}

func (s *taskStudioMediaService) processStudioDesignImage(ctx context.Context, req *StudioDesignRequest, generated *AIImageResponse, filename string) (studioProcessedImage, error) {
	if generated == nil || len(generated.Data) == 0 {
		return studioProcessedImage{}, fmt.Errorf("studio image generation returned no image data")
	}
	first := generated.Data[0]
	data, contentType, err := decodeGeneratedImageData(ctx, first)
	if err != nil {
		return studioProcessedImage{}, err
	}
	originalURL, err := s.persistStudioImageBytes(ctx, data, contentType, strings.TrimSuffix(filename, ".png")+"-original.png")
	if err != nil {
		return studioProcessedImage{}, err
	}
	processed := studioProcessedImage{
		ImageURL:                  originalURL,
		OriginalImageURL:          originalURL,
		RevisedPrompt:             first.RevisedPrompt,
		TransparentBackgroundMode: studioDesignTransparencyMode(req),
		BackgroundRemovalStatus:   StudioBackgroundRemovalStatusNotRequested,
	}
	if processed.TransparentBackgroundMode != StudioTransparencyModeRemoval {
		return processed, nil
	}
	processed.BackgroundRemovalStatus = StudioBackgroundRemovalStatusPending
	if s.backgroundRemover == nil {
		processed.BackgroundRemovalStatus = StudioBackgroundRemovalStatusFailed
		processed.BackgroundRemovalError = "background removal client is not configured"
		return processed, nil
	}
	removed, removeErr := s.removeStudioBackground(ctx, originalURL, data, contentType)
	if removeErr != nil {
		processed.BackgroundRemovalStatus = StudioBackgroundRemovalStatusFailed
		processed.BackgroundRemovalError = compactStudioGenerationError(removeErr)
		return processed, nil
	}
	if removed == nil {
		processed.BackgroundRemovalStatus = StudioBackgroundRemovalStatusFailed
		processed.BackgroundRemovalError = "background removal returned no result"
		return processed, nil
	}
	if err := validateStudioTransparentPNG(removed.Data); err != nil {
		processed.BackgroundRemovalStatus = StudioBackgroundRemovalStatusFailed
		processed.BackgroundRemovalError = err.Error()
		return processed, nil
	}
	finalURL, uploadErr := s.persistStudioImageBytes(ctx, removed.Data, "image/png", filename)
	if uploadErr != nil {
		processed.BackgroundRemovalStatus = StudioBackgroundRemovalStatusFailed
		processed.BackgroundRemovalError = compactStudioGenerationError(uploadErr)
		return processed, nil
	}
	processed.ImageURL = finalURL
	processed.BackgroundRemovalStatus = StudioBackgroundRemovalStatusSucceeded
	processed.BackgroundRemovalModel = strings.TrimSpace(removed.Model)
	return processed, nil
}

const defaultStudioBackgroundRemovalConcurrency = 4

func (s *taskStudioMediaService) initializeBackgroundRemovalSlots() {
	if s == nil {
		return
	}
	s.backgroundRemovalOnce.Do(func() {
		concurrency := s.backgroundRemovalConcurrency
		if concurrency <= 0 {
			concurrency = defaultStudioBackgroundRemovalConcurrency
		}
		s.backgroundRemovalSlots = make(chan struct{}, concurrency)
	})
}

func (s *taskStudioMediaService) acquireBackgroundRemovalSlot(ctx context.Context) error {
	if s == nil {
		return fmt.Errorf("background removal service is not configured")
	}
	s.initializeBackgroundRemovalSlots()
	select {
	case s.backgroundRemovalSlots <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *taskStudioMediaService) releaseBackgroundRemovalSlot() {
	if s != nil && s.backgroundRemovalSlots != nil {
		<-s.backgroundRemovalSlots
	}
}

func (s *taskStudioMediaService) removeStudioBackground(ctx context.Context, sourceURL string, data []byte, contentType string) (*StudioBackgroundRemovalResult, error) {
	if s == nil || s.backgroundRemover == nil {
		return nil, fmt.Errorf("background removal client is not configured")
	}
	if err := s.acquireBackgroundRemovalSlot(ctx); err != nil {
		return nil, err
	}
	defer s.releaseBackgroundRemovalSlot()

	if remover, ok := s.backgroundRemover.(StudioBackgroundRemoverFromURL); ok {
		if publicURL, err := s.resolveStudioBackgroundRemovalURL(ctx, sourceURL); err == nil {
			return remover.RemoveFromURL(ctx, publicURL)
		}
	}
	return s.backgroundRemover.Remove(ctx, data, contentType)
}

func (s *taskStudioMediaService) resolveStudioBackgroundRemovalURL(ctx context.Context, sourceURL string) (string, error) {
	trimmed := strings.TrimSpace(sourceURL)
	if key, ok := studioReferenceUploadedImageKeyFromURL(trimmed); ok {
		if s == nil || s.resolveUploadedImagePublicURL == nil {
			return "", fmt.Errorf("uploaded image public url resolver is not configured")
		}
		publicURL, err := s.resolveUploadedImagePublicURL(ctx, key)
		if err != nil {
			return "", err
		}
		return validateStudioReferencePublicHTTPSURL(publicURL)
	}
	return validateStudioReferencePublicHTTPSURL(trimmed)
}

var studioPublicImageHTTPClient = newStudioPublicImageHTTPClient()

func newStudioPublicImageHTTPClient() *http.Client {
	transport, ok := http.DefaultTransport.(*http.Transport)
	if ok {
		transport = transport.Clone()
	} else {
		transport = &http.Transport{}
	}
	dialer := &net.Dialer{}
	transport.DialContext = func(ctx context.Context, network string, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, err
		}
		ips, err := net.LookupIP(host)
		if err != nil {
			return nil, err
		}
		for _, ip := range ips {
			if isStudioReferencePrivateIP(ip) {
				continue
			}
			conn, dialErr := dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
			if dialErr == nil {
				return conn, nil
			}
		}
		return nil, fmt.Errorf("image host resolves only to private or unreachable addresses")
	}
	return &http.Client{
		Transport: transport,
		CheckRedirect: func(req *http.Request, _ []*http.Request) error {
			if _, err := validateStudioReferencePublicHTTPSURL(req.URL.String()); err != nil {
				return err
			}
			return nil
		},
	}
}

func (s *taskStudioMediaService) materializeAsyncStudioDesignResult(ctx context.Context, req *StudioDesignRequest, result *AIImageAsyncResult) (*StudioDesignResponse, error) {
	if result == nil || result.Response == nil || len(result.Response.Data) == 0 {
		return nil, fmt.Errorf("studio async image query returned no image data")
	}

	model := resolveStudioDesignImageModel(req, s.imageGenerator.GetDefaultModel())
	response := &StudioDesignResponse{
		Prompt:                    strings.TrimSpace(req.Prompt),
		PrintableWidth:            req.PrintableWidth,
		PrintableHeight:           req.PrintableHeight,
		ImageModel:                model,
		TransparentBackground:     studioDesignTransparencyMode(req) != StudioTransparencyModeNone,
		TransparentBackgroundMode: studioDesignTransparencyMode(req),
		RequestID:                 strings.TrimSpace(firstNonEmpty(result.RequestID, result.Response.RequestID)),
		UpstreamJobID:             strings.TrimSpace(result.JobID),
		RawResponse:               strings.TrimSpace(result.RawResultResponse),
		Usage:                     result.Usage,
		Images:                    make([]StudioGeneratedImage, 0, len(result.Response.Data)),
	}

	for index, item := range result.Response.Data {
		generated := &AIImageResponse{
			Data:          []AIImageData{item},
			Usage:         result.Usage,
			RequestID:     response.RequestID,
			UpstreamJobID: response.UpstreamJobID,
			RawResponse:   response.RawResponse,
		}
		processed, err := s.processStudioDesignImage(ctx, req, generated, fmt.Sprintf("studio-design-%d.png", index+1))
		if err != nil {
			return nil, fmt.Errorf("persist async studio design %d: %w", index+1, err)
		}
		response.Images = append(response.Images, StudioGeneratedImage{
			ID:                        uuid.NewString(),
			ImageURL:                  processed.ImageURL,
			OriginalImageURL:          processed.OriginalImageURL,
			Prompt:                    response.Prompt,
			RevisedPrompt:             processed.RevisedPrompt,
			ImageModel:                model,
			TransparentBackground:     response.TransparentBackground,
			TransparentBackgroundMode: processed.TransparentBackgroundMode,
			BackgroundRemovalStatus:   processed.BackgroundRemovalStatus,
			BackgroundRemovalModel:    processed.BackgroundRemovalModel,
			BackgroundRemovalError:    processed.BackgroundRemovalError,
			VariationIntensity:        req.VariationIntensity,
			RequestID:                 response.RequestID,
			UpstreamJobID:             response.UpstreamJobID,
			RawResponse:               response.RawResponse,
			Usage:                     result.Usage,
		})
	}
	return response, nil
}

func (s *taskStudioMediaService) buildStudioDesignAsyncSubmitResponse(ctx context.Context, req *StudioDesignRequest, submit *AIImageAsyncSubmit) (*studioDesignAsyncSubmitResponse, error) {
	output := &studioDesignAsyncSubmitResponse{Submit: submit}
	if submit == nil || submit.Status != AIImageAsyncResultSucceeded || submit.Response == nil || len(submit.Response.Data) == 0 {
		return output, nil
	}

	response, err := s.materializeAsyncStudioDesignResult(ctx, req, &AIImageAsyncResult{
		JobID:             strings.TrimSpace(submit.JobID),
		RequestID:         strings.TrimSpace(submit.RequestID),
		Provider:          strings.TrimSpace(submit.Provider),
		Status:            submit.Status,
		RawResultResponse: strings.TrimSpace(submit.RawSubmitResponse),
		Usage:             submit.Response.Usage,
		Response:          submit.Response,
	})
	if err != nil {
		return nil, err
	}
	output.Response = response
	return output, nil
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

func (s *taskStudioMediaService) tryGenerateStudioProductImage(ctx context.Context, inputImages []string, promptText string) (*AIImageResponse, error) {
	if s.loadUploadedImage != nil && hasOwnedListingKitUploadReference(inputImages[1:]) {
		return nil, fmt.Errorf("invalid request: uploaded listingkit image must be the primary image")
	}
	generated, err := s.editStudioDesignImageWithReferences(ctx, s.imageGenerator.GetDefaultModel(), promptText, "auto", inputImages)
	if err != nil {
		generated, err = s.editStudioDesignImageWithReferences(ctx, s.imageGenerator.GetDefaultModel(), promptText, "auto", inputImages[:1])
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
