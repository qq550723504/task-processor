package listingkit

import (
	"context"
	"time"

	sheinother "task-processor/internal/shein/api/other"
	sheinproduct "task-processor/internal/shein/api/product"
)

func (s *service) acquireSheinSubmitTask(ctx context.Context, taskID, action, requestID string, startedAt time.Time) (*Task, *ListingKitPreview, error) {
	return s.taskSubmissionRecoveryOrDefault().acquireSheinSubmitTask(ctx, taskID, action, requestID, startedAt)
}

func (s *service) mutateTaskResult(ctx context.Context, taskID string, mutate TaskResultMutation) (*Task, error) {
	return s.taskSubmissionRecoveryOrDefault().mutateTaskResult(ctx, taskID, mutate)
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

func (s *service) resolveSheinSubmitRemoteStatus(productAPI sheinproduct.ProductAPI, otherAPI sheinother.OtherAPI, action, requestID string, lookupCodes []string, spuName string, defaultConfirmed bool, fallbackMessage string, startedAt time.Time, taskID string) (*sheinRemoteConfirmation, error) {
	return s.taskSubmissionRecoveryOrDefault().resolveSheinSubmitRemoteStatus(productAPI, otherAPI, action, requestID, lookupCodes, spuName, defaultConfirmed, fallbackMessage, startedAt, taskID)
}
