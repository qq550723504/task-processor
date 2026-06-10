package listingkit

import "context"

func (s *service) GenerateStudioDesigns(ctx context.Context, req *StudioDesignRequest) (*StudioDesignResponse, error) {
	return s.taskStudioMediaOrDefault().GenerateStudioDesigns(ctx, req)
}

func (s *service) GenerateStudioProductImages(ctx context.Context, req *StudioProductImageRequest) (*StudioProductImageResponse, error) {
	return s.taskStudioMediaOrDefault().GenerateStudioProductImages(ctx, req)
}
