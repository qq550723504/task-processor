package listingkit

import "context"

func (s *service) ListStudioSessionGallery(ctx context.Context, limit int) (*StudioSessionGalleryResponse, error) {
	return s.taskStudioBatchDraftOrDefault().ListStudioSessionGallery(ctx, limit)
}

func (s *service) ListStudioBatches(ctx context.Context, limit int) (*StudioBatchListResponse, error) {
	return s.taskStudioBatchDraftOrDefault().ListStudioBatches(ctx, limit)
}

func (s *service) GetStudioBatch(ctx context.Context, batchID string) (*StudioBatchDraftDetail, error) {
	return s.taskStudioBatchDraftOrDefault().GetStudioBatch(ctx, batchID)
}

func (s *service) UpsertStudioBatch(ctx context.Context, req *UpsertStudioBatchRequest) (*StudioBatchDraftDetail, error) {
	return s.taskStudioBatchDraftOrDefault().UpsertStudioBatch(ctx, req)
}

func (s *service) DeleteStudioBatch(ctx context.Context, batchID string) error {
	return s.taskStudioBatchDraftOrDefault().DeleteStudioBatch(ctx, batchID)
}

func (s *service) SyncStudioDesignAsyncJob(ctx context.Context, sessionID string, jobStatus StudioAsyncJobStatus, jobID string, errMessage string) error {
	return s.taskStudioSessionOrDefault().SyncStudioDesignAsyncJob(ctx, sessionID, jobStatus, jobID, errMessage)
}
