package listingkit

import "context"

func (s *service) GetStudioBatchDetail(ctx context.Context, batchID string) (*StudioBatchDetail, error) {
	return s.taskStudioBatchOrDefault().GetStudioBatchDetail(ctx, batchID)
}

func (s *service) StartStudioBatchGeneration(ctx context.Context, batchID string) (*StudioBatchDetail, error) {
	return s.taskStudioBatchOrDefault().StartStudioBatchGeneration(ctx, batchID)
}

func (s *service) PrepareStudioBatchGeneration(ctx context.Context, batchID string) (*StudioBatchDetail, error) {
	return s.taskStudioBatchOrDefault().PrepareStudioBatchGeneration(ctx, batchID)
}

func (s *service) ResumeStudioBatchGeneration(ctx context.Context, batchID string) (*StudioBatchDetail, error) {
	return s.taskStudioBatchOrDefault().ResumeStudioBatchGeneration(ctx, batchID)
}

func (s *service) PrepareRetryStudioBatchItems(ctx context.Context, batchID string, req *RetryStudioBatchItemsRequest) (*StudioBatchDetail, error) {
	return s.taskStudioBatchOrDefault().PrepareRetryStudioBatchItems(ctx, batchID, req)
}

func (s *service) RetryStudioBatchItems(ctx context.Context, batchID string, req *RetryStudioBatchItemsRequest) (*StudioBatchDetail, error) {
	return s.taskStudioBatchOrDefault().RetryStudioBatchItems(ctx, batchID, req)
}

func (s *service) ApproveStudioBatchDesigns(ctx context.Context, batchID string, req *ApproveStudioBatchDesignsRequest) (*StudioBatchDetail, error) {
	return s.taskStudioBatchOrDefault().ApproveStudioBatchDesigns(ctx, batchID, req)
}

func (s *service) CreateStudioBatchTasks(ctx context.Context, batchID string, req *CreateStudioBatchTasksRequest) (*CreateStudioBatchTasksResult, error) {
	return s.taskStudioBatchOrDefault().CreateStudioBatchTasks(ctx, batchID, req)
}

func (s *service) PrepareCreateStudioBatchTasks(ctx context.Context, batchID string, req *CreateStudioBatchTasksRequest) (*CreateStudioBatchTasksResult, error) {
	return s.taskStudioBatchOrDefault().PrepareCreateStudioBatchTasks(ctx, batchID, req)
}
