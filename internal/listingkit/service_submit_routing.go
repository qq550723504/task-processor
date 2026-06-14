package listingkit

import (
	"context"
	"time"
)

func (s *service) SubmitTask(ctx context.Context, taskID string, req *SubmitTaskRequest) (*ListingKitPreview, error) {
	return s.taskSubmissionOrDefault().SubmitTask(ctx, taskID, req)
}

func (s *service) RefreshSubmissionStatus(ctx context.Context, taskID string) (*ListingKitPreview, error) {
	return s.taskSubmissionRefreshOrDefault().RefreshSubmissionStatus(ctx, taskID)
}

func (s *service) RecoverTaskNow(ctx context.Context, taskID string) (*Task, error) {
	return s.taskRecoveryOrDefault().RecoverTaskNow(ctx, taskID)
}

func (s *service) RunRecoverySweep(ctx context.Context, now time.Time, limit int) (int64, error) {
	return s.taskRecoveryOrDefault().RunRecoverySweep(ctx, now, limit)
}

func (s *service) BulkRecoverTasks(ctx context.Context, query *RecoverBlockedTasksQuery) (int64, error) {
	return s.taskRecoveryOrDefault().BulkRecoverTasks(ctx, query)
}

func (s *service) RequeuePendingTasks(ctx context.Context, req *RequeuePendingTasksRequest) (*RequeuePendingTasksResult, error) {
	return s.taskRequeueOrDefault().RequeuePendingTasks(ctx, req)
}
