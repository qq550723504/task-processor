package listingkit

import (
	"context"
	"fmt"
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

func (s *taskStudioMediaService) generateStudioDesignImage(ctx context.Context, model string, promptText string, size string, referenceURLs []string) (*AIImageResponse, error) {
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

func (s *taskStudioMediaService) editStudioDesignImageWithReferences(ctx context.Context, model string, promptText string, size string, referenceURLs []string) (*AIImageResponse, error) {
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

func (s *taskStudioMediaService) materializeAsyncStudioDesignResult(ctx context.Context, req *StudioDesignRequest, result *AIImageAsyncResult) (*StudioDesignResponse, error) {
	if result == nil || result.Response == nil || len(result.Response.Data) == 0 {
		return nil, fmt.Errorf("studio async image query returned no image data")
	}

	model := resolveStudioDesignImageModel(req, s.imageGenerator.GetDefaultModel())
	response := &StudioDesignResponse{
		Prompt:                strings.TrimSpace(req.Prompt),
		PrintableWidth:        req.PrintableWidth,
		PrintableHeight:       req.PrintableHeight,
		ImageModel:            model,
		TransparentBackground: req.TransparentBackground && model == studioDesignTransparentModel,
		RequestID:             strings.TrimSpace(firstNonEmpty(result.RequestID, result.Response.RequestID)),
		UpstreamJobID:         strings.TrimSpace(result.JobID),
		RawResponse:           strings.TrimSpace(result.RawResultResponse),
		Usage:                 result.Usage,
		Images:                make([]StudioGeneratedImage, 0, len(result.Response.Data)),
	}

	for index, item := range result.Response.Data {
		generated := &AIImageResponse{
			Data:          []AIImageData{item},
			Usage:         result.Usage,
			RequestID:     response.RequestID,
			UpstreamJobID: response.UpstreamJobID,
			RawResponse:   response.RawResponse,
		}
		imageURL, revisedPrompt, err := s.persistGeneratedStudioImage(ctx, generated, fmt.Sprintf("studio-design-%d.png", index+1))
		if err != nil {
			return nil, fmt.Errorf("persist async studio design %d: %w", index+1, err)
		}
		response.Images = append(response.Images, StudioGeneratedImage{
			ID:                    uuid.NewString(),
			ImageURL:              imageURL,
			Prompt:                response.Prompt,
			RevisedPrompt:         revisedPrompt,
			ImageModel:            model,
			TransparentBackground: response.TransparentBackground,
			VariationIntensity:    req.VariationIntensity,
			RequestID:             response.RequestID,
			UpstreamJobID:         response.UpstreamJobID,
			RawResponse:           response.RawResponse,
			Usage:                 result.Usage,
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
	generated, err := s.imageGenerator.EditImage(ctx, &AIImageEditRequest{
		Model:          s.imageGenerator.GetDefaultModel(),
		Prompt:         promptText,
		ImageURL:       inputImages[0],
		ImageURLs:      inputImages,
		Size:           "auto",
		ResponseFormat: "b64_json",
		N:              1,
	})
	if err != nil {
		generated, err = s.imageGenerator.EditImage(ctx, &AIImageEditRequest{
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
