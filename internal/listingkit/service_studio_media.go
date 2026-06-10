package listingkit

import (
	"context"

	openaiclient "task-processor/internal/infra/clients/openai"
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

func (s *service) persistGeneratedStudioImage(ctx context.Context, response *openaiclient.ImageResponse, filename string) (string, string, error) {
	return s.taskStudioMediaOrDefault().persistGeneratedStudioImage(ctx, response, filename)
}

func (s *service) GenerateStudioProductImages(ctx context.Context, req *StudioProductImageRequest) (*StudioProductImageResponse, error) {
	return s.taskStudioMediaOrDefault().GenerateStudioProductImages(ctx, req)
}

func (s *service) generateOneStudioProductImage(ctx context.Context, req *StudioProductImageRequest, sourceURL string, basePrompt string) (string, error) {
	return s.taskStudioMediaOrDefault().generateOneStudioProductImage(ctx, req, sourceURL, basePrompt)
}

func (s *service) tryGenerateStudioProductImage(ctx context.Context, inputImages []string, promptText string) (*openaiclient.ImageResponse, error) {
	return s.taskStudioMediaOrDefault().tryGenerateStudioProductImage(ctx, inputImages, promptText)
}

func (s *service) sanitizeStudioImageInputURLs(ctx context.Context, inputURLs []string) ([]string, error) {
	return s.taskStudioMediaOrDefault().sanitizeStudioImageInputURLs(ctx, inputURLs)
}
