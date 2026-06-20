package listingkit

import "context"

func (s *service) CreateStudioBatchRun(ctx context.Context, req *CreateStudioBatchRunRequest) (*StudioBatchRunRecord, []StudioBatchRunItemRecord, error) {
	return s.taskStudioBatchRunOrDefault().CreateStudioBatchRun(ctx, req)
}

func (s *service) GetStudioBatchRun(ctx context.Context, runID string) (*StudioBatchRunRecord, error) {
	return s.taskStudioBatchRunOrDefault().GetStudioBatchRun(ctx, runID)
}

func (s *service) ListStudioBatchRunItems(ctx context.Context, runID string) ([]StudioBatchRunItemRecord, error) {
	return s.taskStudioBatchRunOrDefault().ListStudioBatchRunItems(ctx, runID)
}

func (s *service) CancelStudioBatchRun(ctx context.Context, runID string) error {
	return s.taskStudioBatchRunOrDefault().CancelStudioBatchRun(ctx, runID)
}

func (s *service) RecoverStudioBatchRun(ctx context.Context, runID string) error {
	return s.taskStudioBatchRunOrDefault().RecoverStudioBatchRun(ctx, runID)
}
