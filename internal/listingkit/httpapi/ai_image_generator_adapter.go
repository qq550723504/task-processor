package httpapi

import (
	"context"

	openaiclient "task-processor/internal/infra/clients/openai"
	"task-processor/internal/listingkit"
)

type listingKitAIImageGenerator struct {
	generator openaiclient.ImageGenerator
}

func adaptListingKitAIImageGenerator(generator openaiclient.ImageGenerator) listingkit.AIImageGenerator {
	if generator == nil {
		return nil
	}
	return listingKitAIImageGenerator{generator: generator}
}

func (g listingKitAIImageGenerator) GenerateImage(ctx context.Context, req *listingkit.AIImageGenerateRequest) (*listingkit.AIImageResponse, error) {
	response, err := g.generator.GenerateImage(ctx, &openaiclient.ImageGenerateRequest{
		Model:          req.Model,
		Prompt:         req.Prompt,
		Size:           req.Size,
		ResponseFormat: req.ResponseFormat,
		N:              req.N,
	})
	if err != nil {
		return nil, err
	}
	return adaptListingKitAIImageResponse(response), nil
}

func (g listingKitAIImageGenerator) EditImage(ctx context.Context, req *listingkit.AIImageEditRequest) (*listingkit.AIImageResponse, error) {
	response, err := g.generator.EditImage(ctx, &openaiclient.ImageEditRequest{
		Model:          req.Model,
		Prompt:         req.Prompt,
		ImageURL:       req.ImageURL,
		ImageURLs:      req.ImageURLs,
		Size:           req.Size,
		ResponseFormat: req.ResponseFormat,
		N:              req.N,
	})
	if err != nil {
		return nil, err
	}
	return adaptListingKitAIImageResponse(response), nil
}

func (g listingKitAIImageGenerator) GetDefaultModel() string {
	return g.generator.GetDefaultModel()
}

func adaptListingKitAIImageResponse(response *openaiclient.ImageResponse) *listingkit.AIImageResponse {
	if response == nil {
		return nil
	}
	data := make([]listingkit.AIImageData, 0, len(response.Data))
	for _, item := range response.Data {
		data = append(data, listingkit.AIImageData{
			URL:           item.URL,
			B64JSON:       item.B64JSON,
			RevisedPrompt: item.RevisedPrompt,
		})
	}
	return &listingkit.AIImageResponse{
		Data:          data,
		Usage:         listingkit.AIUsage(response.Usage),
		RequestID:     response.RequestID,
		UpstreamJobID: response.UpstreamJobID,
		RawResponse:   response.RawResponse,
	}
}
