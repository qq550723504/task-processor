package api

import (
	"context"

	"task-processor/internal/listingkit"
)

func (s *stubHistoryDetailService) ClearSheinResolutionCache(ctx context.Context, taskID string, kind string) (*listingkit.SheinResolutionCacheClearResult, error) {
	return nil, nil
}

func (s *stubHistoryDetailService) GenerateStudioDesigns(ctx context.Context, req *listingkit.StudioDesignRequest) (*listingkit.StudioDesignResponse, error) {
	return nil, nil
}

func (s *stubHistoryService) ClearSheinResolutionCache(ctx context.Context, taskID string, kind string) (*listingkit.SheinResolutionCacheClearResult, error) {
	return nil, nil
}

func (s *stubHistoryService) GenerateStudioDesigns(ctx context.Context, req *listingkit.StudioDesignRequest) (*listingkit.StudioDesignResponse, error) {
	return nil, nil
}

func (s *stubRevisionService) ClearSheinResolutionCache(ctx context.Context, taskID string, kind string) (*listingkit.SheinResolutionCacheClearResult, error) {
	return nil, nil
}

func (s *stubRevisionService) GenerateStudioDesigns(ctx context.Context, req *listingkit.StudioDesignRequest) (*listingkit.StudioDesignResponse, error) {
	return nil, nil
}

func (s *stubRevisionValidateService) ClearSheinResolutionCache(ctx context.Context, taskID string, kind string) (*listingkit.SheinResolutionCacheClearResult, error) {
	return nil, nil
}

func (s *stubRevisionValidateService) GenerateStudioDesigns(ctx context.Context, req *listingkit.StudioDesignRequest) (*listingkit.StudioDesignResponse, error) {
	return nil, nil
}

func (s *stubSubmitService) ClearSheinResolutionCache(ctx context.Context, taskID string, kind string) (*listingkit.SheinResolutionCacheClearResult, error) {
	return nil, nil
}

func (s *stubSubmitService) GenerateStudioDesigns(ctx context.Context, req *listingkit.StudioDesignRequest) (*listingkit.StudioDesignResponse, error) {
	return nil, nil
}
