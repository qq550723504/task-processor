package listingkit

import "context"

func (s *service) GenerateStudioDesigns(ctx context.Context, req *StudioDesignRequest) (*StudioDesignResponse, error) {
	return s.taskStudioMediaOrDefault().GenerateStudioDesigns(ctx, req)
}

func (s *service) GenerateStudioProductImages(ctx context.Context, req *StudioProductImageRequest) (*StudioProductImageResponse, error) {
	return s.taskStudioMediaOrDefault().GenerateStudioProductImages(ctx, req)
}

func (s *service) SubmitStudioDesignsAsync(ctx context.Context, req *StudioDesignRequest) (*AIImageAsyncSubmit, error) {
	asyncService := s.taskStudioMediaOrDefault()
	if asyncService == nil {
		return nil, ErrAsyncImageGenerationNotSupported
	}
	return asyncService.SubmitStudioDesignsAsync(ctx, req)
}

func (s *service) QueryStudioDesignsAsync(ctx context.Context, req *StudioDesignRequest, jobID string) (*studioDesignAsyncQueryResponse, error) {
	asyncService := s.taskStudioMediaOrDefault()
	if asyncService == nil {
		return nil, ErrAsyncImageGenerationNotSupported
	}
	return asyncService.QueryStudioDesignsAsync(ctx, req, jobID)
}
