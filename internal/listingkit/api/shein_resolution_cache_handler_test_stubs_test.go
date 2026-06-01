package api

import (
	"context"

	"task-processor/internal/listingkit"
)

func (s *stubGenerationTaskService) ClearSheinResolutionCache(ctx context.Context, taskID string, kind string) (*listingkit.SheinResolutionCacheClearResult, error) {
	return nil, nil
}

func (s *stubGenerationTaskService) GenerateStudioDesigns(ctx context.Context, req *listingkit.StudioDesignRequest) (*listingkit.StudioDesignResponse, error) {
	s.studioDesignCtx = ctx
	s.studioDesignReq = req
	return s.studioDesigns, s.err
}

func (s *stubGenerationTaskService) GenerateStudioProductImages(ctx context.Context, req *listingkit.StudioProductImageRequest) (*listingkit.StudioProductImageResponse, error) {
	s.studioProductImageCtx = ctx
	s.studioProductImageReq = req
	return s.studioProductImages, s.err
}

func (s *stubHistoryDetailService) ClearSheinResolutionCache(ctx context.Context, taskID string, kind string) (*listingkit.SheinResolutionCacheClearResult, error) {
	return nil, nil
}

func (s *stubHistoryDetailService) GenerateStudioDesigns(ctx context.Context, req *listingkit.StudioDesignRequest) (*listingkit.StudioDesignResponse, error) {
	return nil, nil
}

func (s *stubHistoryDetailService) GenerateStudioProductImages(ctx context.Context, req *listingkit.StudioProductImageRequest) (*listingkit.StudioProductImageResponse, error) {
	return nil, nil
}

func (s *stubHistoryService) ClearSheinResolutionCache(ctx context.Context, taskID string, kind string) (*listingkit.SheinResolutionCacheClearResult, error) {
	return nil, nil
}

func (s *stubHistoryService) GenerateStudioDesigns(ctx context.Context, req *listingkit.StudioDesignRequest) (*listingkit.StudioDesignResponse, error) {
	return nil, nil
}

func (s *stubHistoryService) GenerateStudioProductImages(ctx context.Context, req *listingkit.StudioProductImageRequest) (*listingkit.StudioProductImageResponse, error) {
	return nil, nil
}

func (s *stubRevisionService) ClearSheinResolutionCache(ctx context.Context, taskID string, kind string) (*listingkit.SheinResolutionCacheClearResult, error) {
	return nil, nil
}

func (s *stubRevisionService) GenerateStudioDesigns(ctx context.Context, req *listingkit.StudioDesignRequest) (*listingkit.StudioDesignResponse, error) {
	return nil, nil
}

func (s *stubRevisionService) GenerateStudioProductImages(ctx context.Context, req *listingkit.StudioProductImageRequest) (*listingkit.StudioProductImageResponse, error) {
	return nil, nil
}

func (s *stubRevisionValidateService) ClearSheinResolutionCache(ctx context.Context, taskID string, kind string) (*listingkit.SheinResolutionCacheClearResult, error) {
	return nil, nil
}

func (s *stubRevisionValidateService) GenerateStudioDesigns(ctx context.Context, req *listingkit.StudioDesignRequest) (*listingkit.StudioDesignResponse, error) {
	return nil, nil
}

func (s *stubRevisionValidateService) GenerateStudioProductImages(ctx context.Context, req *listingkit.StudioProductImageRequest) (*listingkit.StudioProductImageResponse, error) {
	return nil, nil
}

func (s *stubSubmitService) ClearSheinResolutionCache(ctx context.Context, taskID string, kind string) (*listingkit.SheinResolutionCacheClearResult, error) {
	return nil, nil
}

func (s *stubSubmitService) GenerateStudioDesigns(ctx context.Context, req *listingkit.StudioDesignRequest) (*listingkit.StudioDesignResponse, error) {
	return nil, nil
}

func (s *stubSubmitService) GenerateStudioProductImages(ctx context.Context, req *listingkit.StudioProductImageRequest) (*listingkit.StudioProductImageResponse, error) {
	return nil, nil
}
